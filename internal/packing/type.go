package packing

import "time"

type CreatePackingList struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PackingList struct {
	ID          string        `json:"id"`
	UserID      string        `json:"userId"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Items       []PackingItem `json:"items"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

type AddPackingItem struct {
	PackingID string `json:"packingId"`
	Name      string `json:"name"`
	Category  string `json:"category"`
}

type RemovePackingItem struct {
	PackingID string `json:"packingId"`
	ItemID    string `json:"itemId"`
}

type PackingItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Category  string    `json:"category"`
	Checked   bool      `json:"checked"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
