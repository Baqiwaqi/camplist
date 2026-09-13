package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"camplist/internal/packing"
)

func renderListItemForm(t *testing.T, form packing.CreateItemForm) string {
	t.Helper()
	var out bytes.Buffer
	if err := ListItemForm(form, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestListItemFormKeepsOptionalFieldsFolded(t *testing.T) {
	body := renderListItemForm(t, packing.NewCreateItemForm("list"))
	for _, want := range []string{
		`<label for="item-name-new" class="sr-only">Item name</label>`,
		`name="name"`,
		`name="category" list="list-categories"`,
		`id="item-scope-new"`,
		`name="revision"`,
		`<details class="mt-1">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("add item form missing %q", want)
		}
	}
}

func TestListItemFormOpensOptionalFieldsWhenFilled(t *testing.T) {
	for name, form := range map[string]packing.CreateItemForm{
		"category": {Category: "Light"},
		"scope":    {Scope: "person"},
	} {
		if body := renderListItemForm(t, form); !strings.Contains(body, `<details class="mt-1" open>`) {
			t.Errorf("%s: optional fields stay folded after the form comes back filled in", name)
		}
	}
}
