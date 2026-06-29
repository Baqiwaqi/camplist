package packing

type CreatePackingListForm struct {
	Initial          bool
	SubmitButtonText string
	Name             string
	Description      string
	Error            []string
}

func NewCreatePackingListForm() CreatePackingListForm {
	return CreatePackingListForm{
		Initial: true,
	}
}

func (f CreatePackingListForm) ValidateName() []string {
	if f.Initial {
		return nil
	}

	var msgs []string
	if f.Name == "" {
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
