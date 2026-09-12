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
}

func NewStore(container containerClient) *Store {
	return &Store{
		container,
	}
}

func (s *Store) CreatePackingSession(ctx context.Context, listID string, userID string) (PackingSession, error) {
	list, err := s.GetPackingList(ctx, listID, userID)
	if err != nil {
		return PackingSession{}, fmt.Errorf("get packing list: %w", err)
	}

	session := NewPackingSession(list)

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

	return sessions, nil
}

func (s *Store) GetPackingSession(ctx context.Context, id string, userID string) (PackingSession, error) {
	pk := azcosmos.NewPartitionKeyString(userID)

	res, err := s.container.ReadItem(ctx, pk, id, nil)
	if err != nil {
		return PackingSession{}, fmt.Errorf("read packing session: %w", err)
	}

	var session PackingSession
	if err := json.Unmarshal(res.Value, &session); err != nil {
		return PackingSession{}, fmt.Errorf("unmarshal packing session: %w", err)
	}

	if session.Type != "packing-session" || session.UserID != userID {
		return PackingSession{}, ErrNotFound
	}
	session.etag = string(res.ETag)
	return session, nil
}

func (s *Store) DeletePackingSession(ctx context.Context, id string, userId string) error {
	if _, err := s.GetPackingSession(ctx, id, userId); err != nil {
		return err
	}
	pk := azcosmos.NewPartitionKeyString(userId)

	_, err := s.container.DeleteItem(ctx, pk, id, nil)
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

	return items, nil
}

func (s *Store) SavePackingList(ctx context.Context, list PackingList) error {
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
	pk := azcosmos.NewPartitionKeyString(userId)

	res, err := s.container.ReadItem(ctx, pk, id, nil)
	if err != nil {
		return PackingList{}, err
	}

	list, err := decodePackingList(res)
	if err != nil {
		return PackingList{}, err
	}

	if list.Type != "packing-list" || list.UserID != userId || list.DeletedAt != nil {
		return PackingList{}, ErrNotFound
	}
	list.etag = string(res.ETag)
	return list, nil
}

func (s *Store) DeletePackingList(ctx context.Context, id string, userId string) error {
	pk := azcosmos.NewPartitionKeyString(userId)

	ops := azcosmos.PatchOperations{}
	ops.SetCondition("FROM c WHERE c.type = 'packing-list' AND (NOT IS_DEFINED(c.deletedAt) OR IS_NULL(c.deletedAt))")
	ops.AppendSet("/deletedAt", time.Now().UTC())
	_, err := s.container.PatchItem(ctx, pk, id, ops, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) AddItem(ctx context.Context, id string, userId string, item PackingItem) error {
	pk := azcosmos.NewPartitionKeyString(userId)

	ops := azcosmos.PatchOperations{}
	ops.SetCondition("FROM c WHERE c.type = 'packing-list' AND (NOT IS_DEFINED(c.deletedAt) OR IS_NULL(c.deletedAt))")
	ops.AppendAdd("/items/-", item)
	ops.AppendReplace("/updatedAt", time.Now().UTC())

	_, err := s.container.PatchItem(ctx, pk, id, ops, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) RemoveItem(ctx context.Context, id string, userId string, itemId string) error {
	// get list
	list, err := s.GetPackingList(ctx, id, userId)
	if err != nil {
		return err
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
