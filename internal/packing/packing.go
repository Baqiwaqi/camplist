package packing

import (
	"time"

	"github.com/google/uuid"
)

func NewPackingSession(list PackingList) PackingSession {
	list.Sharing = Sharing{}
	list.actor = ""
	list.Tasks = append([]PreparationTask{}, list.Tasks...)
	for i := range list.Tasks {
		list.Tasks[i].Done = false
		list.Tasks[i].Revision = 0
		list.Tasks[i].ChangedBy = ""
		list.Tasks[i].ChangedByID = ""
	}
	list.AppliedReviews = append([]string{}, list.AppliedReviews...)
	list.Changes = append([]string{}, list.Changes...)
	list.Items = append([]PackingItem{}, list.Items...)
	for i := range list.Items {
		list.Items[i].Checked = false
		list.Items[i].Revision = 0
	}
	return PackingSession{
		ID:        uuid.NewString(),
		UserID:    list.UserID,
		Type:      "packing-session",
		CreatedAt: time.Now().UTC(),
		List:      list,
	}
}

func NewList(userId string, name string, description string) PackingList {
	now := time.Now().UTC()
	id := uuid.NewString()

	return PackingList{
		ID:          id,
		UserID:      userId,
		Type:        "packing-list",
		Name:        name,
		Description: description,
		Items:       []PackingItem{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewItem(name string, category string) PackingItem {
	now := time.Now().UTC()
	id := uuid.NewString()

	return PackingItem{
		ID:        id,
		Name:      name,
		Category:  category,
		Checked:   false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
