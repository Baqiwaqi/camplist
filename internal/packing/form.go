package packing

import "strings"

type CreatePackingListForm struct {
	Action           string `schema:"-"`
	Initial          bool   `schema:"-"`
	SubmitButtonText string `schema:"-"`
	Name             string
	Description      string
	Error            []string `schema:"-"`
}

func NewCreatePackingListForm() CreatePackingListForm {
	return CreatePackingListForm{
		Action:           "/packing-list/new",
		Initial:          true,
		SubmitButtonText: "Create New",
	}
}

func EditPackingListForm(list PackingList) CreatePackingListForm {
	return CreatePackingListForm{
		Action:           "/packing-list/" + list.ID + "/edit",
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
	Action           string `schema:"-"`
	Initial          bool   `schema:"-"`
	SubmitButtonText string `schema:"-"`
	Name             string
	Category         string
	Error            []string `schema:"-"`
}

func NewCreateItemForm(listId string) CreateItemForm {
	return CreateItemForm{
		Action:           "/packing-list/" + listId + "/add-item",
		Initial:          true,
		SubmitButtonText: "Add Item",
	}
}

func EditItemForm(listId string, item PackingItem) CreateItemForm {
	return CreateItemForm{
		Action:           "/packing-list/" + listId + "/edit-item/" + item.ID,
		Initial:          false,
		SubmitButtonText: "Save",
		Name:             item.Name,
		Category:         item.Category,
	}
}

func (f CreateItemForm) ValidateName() []string {
	if f.Initial {
		return nil
	}

	var msgs []string
	if strings.TrimSpace(f.Name) == "" {
		msgs = append(msgs, "Name is required")
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

	var msgs []string
	msgs = append(msgs, f.ValidateName()...)

	return msgs
}
