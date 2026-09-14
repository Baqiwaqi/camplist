package packing

import "time"

type CreatePackingSession struct {
	ListID string `json:"listId"`
}

type PackingSession struct {
	OwnerName      string                      `json:"ownerName,omitempty"`
	Name           string                      `json:"name,omitempty"`
	Sharing        Sharing                     `json:"sharing,omitempty"`
	Improvements   []string                    `json:"improvements,omitempty"`
	Operations     map[string]PackingOperation `json:"operations,omitempty"`
	ReviewTargetID string                      `json:"reviewTargetId,omitempty"`
	ArchivedAt     *time.Time                  `json:"archivedAt,omitempty"`
	etag           string
	Review         []ReviewEntry `json:"review,omitempty"`
	ID             string        `json:"id"`
	UserID         string        `json:"userId"`
	Type           string        `json:"type"`
	CreatedAt      time.Time     `json:"createdAt"`
	List           PackingList   `json:"list"`
}

type PackingList struct {
	Sharing        Sharing `json:"sharing,omitempty"`
	actor          string
	Tasks          []PreparationTask `json:"tasks,omitempty"`
	AppliedReviews []string          `json:"appliedReviews,omitempty"`
	Changes        []string          `json:"changes,omitempty"`
	etag           string
	ID             string        `json:"id"`
	UserID         string        `json:"userId"`
	Type           string        `json:"type"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Items          []PackingItem `json:"items"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
	DeletedAt      *time.Time    `json:"deletedAt,omitempty"`
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
	Kind           string     `json:"kind,omitempty"`
	Scope          string     `json:"scope,omitempty"`
	SourceID       string     `json:"sourceId,omitempty"`
	Assignee       string     `json:"assignee,omitempty"`
	AssigneeName   string     `json:"assigneeName,omitempty"`
	SourceRevision string     `json:"-"`
	ChangedBy      string     `json:"changedBy,omitempty"`
	ChangedByID    string     `json:"changedById,omitempty"`
	Revision       int64      `json:"revision"`
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Category       string     `json:"category"`
	Checked        bool       `json:"checked"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	DeletedAt      *time.Time `json:"deletedAt,omitempty"`
}

// Revision identifies the template version shown to the camper.
func (l PackingList) Revision() string { return l.etag }

func (s PackingSession) TemplateID() string {
	if s.ReviewTargetID != "" {
		return s.ReviewTargetID
	}
	return s.List.ID
}
