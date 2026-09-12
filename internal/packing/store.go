package packing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("packing record not found")

// containerClient is the Cosmos boundary used by the packing store.
type containerClient interface {
	CreateItem(context.Context, azcosmos.PartitionKey, []byte, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error)
	ReplaceItem(context.Context, azcosmos.PartitionKey, string, []byte, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error)
	ReadItem(context.Context, azcosmos.PartitionKey, string, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error)
	DeleteItem(context.Context, azcosmos.PartitionKey, string, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error)
	PatchItem(context.Context, azcosmos.PartitionKey, string, azcosmos.PatchOperations, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error)
	NewQueryItemsPager(string, azcosmos.PartitionKey, *azcosmos.QueryOptions) *runtime.Pager[azcosmos.QueryItemsResponse]
}

type Store struct {
	container containerClient
	now       func() time.Time
}

type StoreOption func(*Store)

// WithClock supplies the external clock used for invitation expiry.
func WithClock(now func() time.Time) StoreOption { return func(s *Store) { s.now = now } }
func NewStore(container containerClient, options ...StoreOption) *Store {
	s := &Store{container: container, now: time.Now}
	for _, option := range options {
		option(s)
	}
	return s
}
func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Store) CreatePackingSession(ctx context.Context, listID string, userID string, ownerName ...string) (PackingSession, error) {
	list, err := s.GetPackingList(ctx, listID, userID)
	if err != nil {
		return PackingSession{}, fmt.Errorf("get packing list: %w", err)
	}

	sessions, err := s.ListPackingSession(ctx, userID)
	if err != nil {
		return PackingSession{}, fmt.Errorf("read previous trips: %w", err)
	}
	var previous PackingSession
	for _, candidate := range sessions {
		if candidate.UserID == userID && candidate.List.ID == listID && candidate.CreatedAt.After(previous.CreatedAt) {
			previous = candidate
		}
	}
	session := NewPackingSession(list)
	session.UserID = userID
	if len(ownerName) > 0 && len(ownerName[0]) <= 200 {
		session.OwnerName = ownerName[0]
	}
	session.Name = list.Name + " – " + session.CreatedAt.Format("Jan 2, 2006")
	session.expandPersonalEntries()
	if len(session.List.Items)+len(session.List.Tasks) > 2000 {
		return PackingSession{}, ErrInvalid
	}
	// Changes are append-only; keep the full log in the snapshot for the next boundary.
	firstNew := 0
	for firstNew < len(previous.List.Changes) && firstNew < len(list.Changes) && previous.List.Changes[firstNew] == list.Changes[firstNew] {
		firstNew++
	}
	session.Improvements = append([]string{}, list.Changes[firstNew:]...)

	pk := azcosmos.NewPartitionKeyString(userID)

	bytes, err := json.Marshal(session)
	if err != nil {
		return PackingSession{}, fmt.Errorf("marshal packing session: %w", err)
	}

	_, err = s.container.CreateItem(ctx, pk, bytes, nil)
	if err != nil {
		return PackingSession{}, fmt.Errorf("create packing session: %w", err)
	}

	return session, nil
}

func (s *Store) ListPackingSession(ctx context.Context, userID string) ([]PackingSession, error) {
	pk := azcosmos.NewPartitionKeyString(userID)
	query := "SELECT * FROM sessions s WHERE s.userId = @userID AND s.type = 'packing-session' AND (NOT IS_DEFINED(s.deletedAt) OR IS_NULL(s.deletedAt))"
	queryOptions := azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@userID", Value: userID},
		},
	}
	pager := s.container.NewQueryItemsPager(query, pk, &queryOptions)

	sessions, err := mapPackingSessions(ctx, pager)
	if err != nil {
		return []PackingSession{}, fmt.Errorf("map packing sessions: %w", err)
	}

	refs, err := s.references(ctx, userID, "packing-session")
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		shared, err := s.GetPackingSession(ctx, ref.ResourceID, userID)
		if err == nil {
			sessions = append(sessions, shared)
		} else if !isMissing(err) && !errors.Is(err, ErrAccessRemoved) {
			return nil, err
		}
	}
	return sessions, nil
}

func (s *Store) GetPackingSession(ctx context.Context, id string, userID string) (PackingSession, error) {
	res, _, err := s.resource(ctx, "packing-session", id, userID)
	if err != nil {
		return PackingSession{}, err
	}

	var session PackingSession
	if err := json.Unmarshal(res.Value, &session); err != nil {
		return PackingSession{}, fmt.Errorf("unmarshal packing session: %w", err)
	}

	if session.Type != "packing-session" {
		return PackingSession{}, ErrNotFound
	}
	session.etag = string(res.ETag)
	return session, nil
}

func (s *Store) DeletePackingSession(ctx context.Context, id string, userId string) error {
	session, err := s.GetPackingSession(ctx, id, userId)
	if err != nil {
		return err
	}
	if session.UserID != userId {
		return ErrForbidden
	}
	pk := azcosmos.NewPartitionKeyString(userId)

	etag := azcore.ETag(session.etag)
	_, err = s.container.DeleteItem(ctx, pk, id, &azcosmos.ItemOptions{IfMatchEtag: &etag})
	if err != nil {
		return fmt.Errorf("delete packing session: %w", err)
	}
	return nil
}

// SetSessionItem uses the same revision protocol as offline packing.
func (s *Store) SetSessionItem(ctx context.Context, sessionID, userID, itemID string, checked bool) (PackingSession, error) {
	session, err := s.GetPackingSession(ctx, sessionID, userID)
	if err != nil {
		return PackingSession{}, err
	}
	if session.IsShared() {
		return PackingSession{}, ErrInvalid
	}
	i, err := getItemIndexById(session.List, itemID)
	if err != nil {
		return PackingSession{}, ErrNotFound
	}
	return s.SyncSessionItem(ctx, sessionID, userID, PackingOperation{ID: uuid.NewString(), ItemID: itemID, Checked: checked, ExpectedRevision: session.List.Items[i].Revision})
}

func (s *Store) GetPackingLists(ctx context.Context, userID string) ([]PackingList, error) {
	pk := azcosmos.NewPartitionKeyString(userID)
	query := "SELECT * FROM lists l WHERE l.userId = @userID AND l.type = 'packing-list' AND (NOT IS_DEFINED(l.deletedAt) OR IS_NULL(l.deletedAt))"
	queryOptions := azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@userID", Value: userID},
		},
	}
	pager := s.container.NewQueryItemsPager(query, pk, &queryOptions)

	items, err := mapPackingList(ctx, pager)
	if err != nil {
		return items, err
	}

	refs, err := s.references(ctx, userID, "packing-list")
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		shared, err := s.GetPackingList(ctx, ref.ResourceID, userID)
		if err == nil {
			items = append(items, shared)
		} else if !isMissing(err) && !errors.Is(err, ErrAccessRemoved) {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) SavePackingList(ctx context.Context, list PackingList) error {
	if list.etag != "" {
		actor := list.actor
		if actor == "" {
			actor = list.UserID
		}
		current, err := s.GetPackingList(ctx, list.ID, actor)
		if err != nil {
			return err
		}
		if current.UserID != list.UserID {
			return ErrForbidden
		}
		if current.etag != list.etag {
			return ErrConflict
		}
		list.Sharing = current.Sharing
	}
	pk := azcosmos.NewPartitionKeyString(list.UserID)

	list.UpdatedAt = time.Now().UTC()

	bytes, err := json.Marshal(list)
	if err != nil {
		return err
	}

	if list.etag == "" {
		_, err = s.container.CreateItem(ctx, pk, bytes, nil)
	} else {
		etag := azcore.ETag(list.etag)
		_, err = s.container.ReplaceItem(ctx, pk, list.ID, bytes, &azcosmos.ItemOptions{IfMatchEtag: &etag})
	}
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) GetPackingList(ctx context.Context, id string, userId string) (PackingList, error) {
	res, _, err := s.resource(ctx, "packing-list", id, userId)
	if err != nil {
		return PackingList{}, err
	}

	list, err := decodePackingList(res)
	if err != nil {
		return PackingList{}, err
	}

	if list.Type != "packing-list" || list.DeletedAt != nil {
		return PackingList{}, ErrNotFound
	}
	list.etag = string(res.ETag)
	list.actor = userId
	for i := range list.Items {
		list.Items[i].SourceRevision = list.etag
	}
	return list, nil
}

func (s *Store) DeletePackingList(ctx context.Context, id, user string) error {
	list, err := s.GetPackingList(ctx, id, user)
	if err != nil {
		return err
	}
	if list.UserID != user {
		return ErrForbidden
	}
	now := time.Now().UTC()
	list.DeletedAt = &now
	return s.SavePackingList(ctx, list)
}
func (s *Store) AddItem(ctx context.Context, id, user string, item PackingItem) error {
	if !validScope(item.Scope, false) {
		return ErrInvalid
	}
	for attempt := 0; attempt < 5; attempt++ {
		list, err := s.GetPackingList(ctx, id, user)
		if err != nil {
			return err
		}
		list.Items = append(list.Items, item)
		err = s.SavePackingList(ctx, list)
		if preconditionFailed(err) || errors.Is(err, ErrConflict) {
			continue
		}
		return err
	}
	return ErrConflict
}

func (s *Store) RemoveItem(ctx context.Context, id string, userId string, itemId string, revision ...string) error {
	// get list
	list, err := s.GetPackingList(ctx, id, userId)
	if err != nil {
		return err
	}

	if list.IsShared() && (len(revision) == 0 || revision[0] != list.Revision()) {
		return ErrConflict
	}
	// remove item
	err = removeItemById(&list, itemId)
	if err != nil {
		return err
	}

	// save updated list
	err = s.SavePackingList(ctx, list)
	if err != nil {
		return err
	}

	return nil
}

func mapPackingList(ctx context.Context, pager *runtime.Pager[azcosmos.QueryItemsResponse]) ([]PackingList, error) {
	items := []PackingList{}

	for pager.More() {
		response, err := pager.NextPage(ctx)
		if err != nil {
			return []PackingList{}, err
		}

		for _, bytes := range response.Items {
			item := PackingList{}
			err := json.Unmarshal(bytes, &item)
			if err != nil {
				return []PackingList{}, err
			}
			items = append(items, item)
		}
	}

	return items, nil
}

func mapPackingSessions(ctx context.Context, pager *runtime.Pager[azcosmos.QueryItemsResponse]) ([]PackingSession, error) {
	sessions := []PackingSession{}

	for pager.More() {
		response, err := pager.NextPage(ctx)
		if err != nil {
			return []PackingSession{}, err
		}

		for _, bytes := range response.Items {
			item := PackingSession{}
			err := json.Unmarshal(bytes, &item)
			if err != nil {
				return []PackingSession{}, err
			}
			sessions = append(sessions, item)
		}
	}

	return sessions, nil
}

func decodePackingList(res azcosmos.ItemResponse) (PackingList, error) {
	var list PackingList

	if err := json.Unmarshal(res.Value, &list); err != nil {
		return PackingList{}, fmt.Errorf("unmarshal packing list: %w", err)
	}

	return list, nil
}

func getItemIndexById(list PackingList, itemID string) (int, error) {
	for i, item := range list.Items {
		if item.ID == itemID {
			return i, nil
		}
	}

	return -1, fmt.Errorf("item with id %q not found", itemID)
}

func removeItemById(list *PackingList, itemID string) error {
	index, err := getItemIndexById(*list, itemID)
	if err != nil {
		return err
	}

	list.Items = append(list.Items[:index], list.Items[index+1:]...)

	return nil
}
