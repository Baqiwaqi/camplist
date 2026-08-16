package packing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

type Store struct {
	container *azcosmos.ContainerClient
}

func NewStore(container *azcosmos.ContainerClient) *Store {
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
	query := "SELECT * FROM sessions s WHERE s.userId = @userID AND (NOT IS_DEFINED(s.deletedAt) OR IS_NULL(s.deletedAt))"
	queryOptions := azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@userID", Value: userID},
		},
	}
	pager := s.container.NewQueryItemsPager(query, pk, &queryOptions)

	sessions, err := mapPackingSessions(ctx, pager)
	if err != nil {
		return []PackingSession{}, fmt.Errorf("Unable to map packing sessions")
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

	return session, nil
}

func (s *Store) DeletePackingSession(ctx context.Context, id string, userId string) error {
	pk := azcosmos.NewPartitionKeyString(userId)

	_, err := s.container.DeleteItem(ctx, pk, id, nil)
	if err != nil {
		return fmt.Errorf("Error during packing session deletion")
	}
	return nil
}

func (s *Store) ToggleSessionItem(ctx context.Context, sessionID string, userID string, itemID string) (PackingItem, error) {
	session, err := s.GetPackingSession(ctx, sessionID, userID)
	if err != nil {
		return PackingItem{}, fmt.Errorf("get packing session: %w", err)
	}

	// toggle item
	i, err := getItemIndexById(session.List, itemID)
	if err != nil {
		return PackingItem{}, fmt.Errorf("get item index by id: %w", err)
	}

	ops := azcosmos.PatchOperations{}
	ops.AppendReplace(fmt.Sprintf("/list/items/%d/checked", i), !session.List.Items[i].Checked)

	pk := azcosmos.NewPartitionKeyString(userID)
	_, err = s.container.PatchItem(ctx, pk, sessionID, ops, nil)
	if err != nil {
		return PackingItem{}, fmt.Errorf("replace packing session: %w", err)
	}

	return session.List.Items[i], nil
}

func (s *Store) GetPackingLists(ctx context.Context, userID string) ([]PackingList, error) {
	pk := azcosmos.NewPartitionKeyString(userID)
	query := "SELECT * FROM lists l WHERE l.userId = @userID AND (NOT IS_DEFINED(l.deletedAt) OR IS_NULL(l.deletedAt))"
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

	_, err = s.container.UpsertItem(ctx, pk, bytes, nil)
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

	return list, nil
}

func (s *Store) DeletePackingList(ctx context.Context, id string, userId string) error {
	pk := azcosmos.NewPartitionKeyString(userId)

	ops := azcosmos.PatchOperations{}
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
