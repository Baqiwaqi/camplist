package packing

import (
	"time"

	"github.com/google/uuid"
)

func NewList(userId string, name string, description string) PackingList {
	now := time.Now().UTC()
	id := uuid.NewString()

	return PackingList{
		ID:          id,
		UserID:      userId,
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
