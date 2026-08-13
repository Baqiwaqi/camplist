package packing

import "time"

type CreatePackingSession struct {
	ListID string `json:"listId"`
}

type PackingSession struct {
	ID        string      `json:"id"`
	UserID    string      `json:"userId"`
	Type      string      `json:"type"`
	CreatedAt time.Time   `json:"createdAt"`
	List      PackingList `json:"list"`
}

type PackingList struct {
	ID          string        `json:"id"`
	UserID      string        `json:"userId"`
	Type        string        `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Items       []PackingItem `json:"items"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	DeletedAt   *time.Time    `json:"deletedAt,omitempty"`
}

func (l PackingList) CountChecked() int {
	var checked int
	for _, item := range l.Items {
		if item.Checked {
			checked++
		}
	}
	return checked
}

type PackingItem struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Category  string     `json:"category"`
	Checked   bool       `json:"checked"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}
