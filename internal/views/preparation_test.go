package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func TestPreparationRowsToggleRenameAndRemove(t *testing.T) {
	list := packing.NewList("user", "Weekend", "")
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}, {ID: "fixed", Name: "Already repaired", Done: true}}
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	var out bytes.Buffer
	if err := PackingDetails("Weekend", list, packing.NewCreateItemForm(list.ID), "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		`aria-label="Toggle done for Buy fuel"`,
		`class="pack-row packed"`,
		">Mark done<",
		">Undo<",
		`name="done" value="true"`,
		`name="done" value="false"`,
		`x-on:click="editing = !editing"`,
		">Edit tasks<",
		`class="btn btn-danger btn-sm" data-confirm-action="Remove task"`,
		`hx-post="/packing-list/` + list.ID + `/preparation/edit"`,
		`data-confirm-action="Remove task"`,
		`id="task-fuel" name="name" value="Buy fuel"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preparation card missing %q", want)
		}
	}
	if strings.Contains(body, `type="checkbox"`) || strings.Contains(body, "Save task") {
		t.Error("old checkbox rows are still rendered")
	}
	if !strings.Contains(body, `&#34;action&#34;:&#34;remove&#34;`) {
		t.Error("remove item does not post action=remove")
	}
}
