package packing

import (
	"fmt"
	"strings"
)

type CreatePackingListForm struct {
	Revision         string
	Action           string `schema:"-"`
	Initial          bool   `schema:"-"`
	SubmitButtonText string `schema:"-"`
	Name             string
	Description      string
	Error            []string `schema:"-"`
}

func NewCreatePackingListForm() CreatePackingListForm {
	return CreatePackingListForm{
		Action:           "/packing-lists/new",
		Initial:          true,
		SubmitButtonText: "Create list",
	}
}

func EditPackingListForm(list PackingList) CreatePackingListForm {
	return CreatePackingListForm{
		Action:           "/packing-lists/" + list.ID + "/edit",
		Revision:         list.Revision(),
		Initial:          false,
		SubmitButtonText: "Save",
		Name:             list.Name,
		Description:      list.Description,
	}
}

func (f CreatePackingListForm) ValidateName() []string {
	if f.Initial {
		return nil
	}

	var msgs []string
	if strings.TrimSpace(f.Name) == "" {
		msgs = append(msgs, "Name is required")
	}
	return msgs
}

func (f CreatePackingListForm) NameHasError() bool {
	return len(f.ValidateName()) > 0
}

func (f CreatePackingListForm) Validate() []string {
	if f.Initial {
		return nil
	}

	var msgs []string
	msgs = append(msgs, f.ValidateName()...)

	return msgs
}

type CreateItemForm struct {
	Scope            string
	Categories       []CategoryOption `schema:"-"`
	Revision         string
	Action           string `schema:"-"`
	Initial          bool   `schema:"-"`
	SubmitButtonText string `schema:"-"`
	Name             string
	Category         string
	Error            []string `schema:"-"`
}

func NewCreateItemForm(listId string) CreateItemForm {
	return CreateItemForm{
		Action:           "/packing-lists/" + listId + "/add-item",
		Initial:          true,
		SubmitButtonText: "Add item",
	}
}

func EditItemForm(listId string, item PackingItem) CreateItemForm {
	return CreateItemForm{
		Action:           "/packing-lists/" + listId + "/edit-item/" + item.ID,
		Revision:         item.SourceRevision,
		Initial:          false,
		SubmitButtonText: "Save",
		Name:             item.Name,
		Category:         item.Category,
		Scope:            item.Scope,
	}
}

func (f CreateItemForm) ValidateName() []string {
	if f.Initial {
		return nil
	}

	var msgs []string
	switch name := strings.TrimSpace(f.Name); {
	case name == "":
		msgs = append(msgs, "Name is required")
	case len(name) > MaxItemNameLength:
		msgs = append(msgs, fmt.Sprintf("The name is longer than %d characters.", MaxItemNameLength))
	}
	return msgs
}

func (f CreateItemForm) NameHasError() bool {
	return len(f.ValidateName()) > 0
}

func (f CreateItemForm) Validate() []string {
	if f.Initial {
		return nil
	}

	msgs := f.ValidateName()
	if len(strings.TrimSpace(f.Category)) > MaxCategoryLength {
		msgs = append(msgs, fmt.Sprintf("The category is longer than %d characters.", MaxCategoryLength))
	}
	return msgs
}
