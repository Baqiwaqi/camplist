package packing

import (
	"context"
	"time"
)

// FindItem returns the item with the given id.
func (l PackingList) FindItem(id string) (PackingItem, bool) {
	for _, item := range l.Items {
		if item.ID == id {
			return item, true
		}
	}
	return PackingItem{}, false
}

// UpdateItem renames an item and changes its category and returns the saved
// list. Other fields stay as stored. On a shared list the update is refused
// when the item's SourceRevision is stale.
func (s *Store) UpdateItem(ctx context.Context, listID string, userID string, item PackingItem) (PackingList, error) {
	list, err := s.GetPackingList(ctx, listID, userID)
	if err != nil {
		return PackingList{}, err
	}
	if list.IsShared() && item.SourceRevision != list.Revision() {
		return PackingList{}, ErrConflict
	}
	index, err := getItemIndexById(list, item.ID)
	if err != nil {
		return PackingList{}, err
	}
	if !validScope(item.Scope, false) {
		return PackingList{}, ErrInvalid
	}
	list.Items[index].Scope = item.Scope
	list.Items[index].Name = item.Name
	list.Items[index].Category = item.Category
	list.Items[index].UpdatedAt = time.Now().UTC()
	etag, err := s.saveList(ctx, list)
	if err != nil {
		return PackingList{}, err
	}
	list.setRevision(etag)
	return list, nil
}
