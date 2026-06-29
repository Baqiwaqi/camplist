package packing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

const DemoUserID = "demo-user-id"

type Store struct {
	container *azcosmos.ContainerClient
}

func NewStore(container *azcosmos.ContainerClient) *Store {
	return &Store{
		container,
	}
}

func (s *Store) GetPackingLists(ctx context.Context, userID string) ([]PackingList, error) {
	pk := azcosmos.NewPartitionKeyString(userID)
	query := "SELECT * FROM lists l WHERE l.userId = @userID"
	queryOptions := azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@userID", Value: userID},
		},
	}
	pager := s.container.NewQueryItemsPager(query, pk, &queryOptions)

	items, err := mapPackingList(pager)
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
		return PackingList{}, nil
	}

	return list, nil
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

func mapPackingList(pager *runtime.Pager[azcosmos.QueryItemsResponse]) ([]PackingList, error) {
	items := []PackingList{}

	for pager.More() {
		response, err := pager.NextPage(context.TODO())
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
