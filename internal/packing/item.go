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

// UpdateItem renames an item and changes its category. Other fields stay as stored.
func (s *Store) UpdateItem(ctx context.Context, listID string, userID string, item PackingItem) error {
	list, err := s.GetPackingList(ctx, listID, userID)
	if err != nil {
		return err
	}
	if list.IsShared() && item.SourceRevision != list.Revision() {
		return ErrConflict
	}
	index, err := getItemIndexById(list, item.ID)
	if err != nil {
		return err
	}
	if !validScope(item.Scope, false) {
		return ErrInvalid
	}
	list.Items[index].Scope = item.Scope
	list.Items[index].Name = item.Name
	list.Items[index].Category = item.Category
	list.Items[index].UpdatedAt = time.Now().UTC()
	return s.SavePackingList(ctx, list)
}
