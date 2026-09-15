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
		`class="pack-row py-2 packed"`,
		">Mark done<",
		">Undo<",
		`name="done" value="true"`,
		`name="done" value="false"`,
		`<section id="list-preparation" class="card p-0" x-data="{ editing: false }">`,
		"1 of 2 done",
		`x-on:click="editing = !editing"`,
		">Edit tasks<",
		`data-confirm-action="Remove task"`,
		`&#34;next&#34;:&#34;fixed&#34;`,
		`id="task-fuel" name="name" value="Buy fuel"`,
		`<ul id="preparation-tasks">`,
		`method="post" action="/packing-lists/` + list.ID + `/preparation/edit" hx-post="/packing-lists/` + list.ID + `/preparation/edit" hx-swap="outerHTML" hx-sync="this:drop" hx-target="#list-preparation"`,
		`name="editing" x-bind:value="editing"`,
		`id="task-done-fuel"`,
		`<input type="hidden" id="list-revision" value="">`,
		`<div id="list-items" class="-mt-px pb-2 md:columns-2 md:gap-x-6" data-empty-focus="item-name-new">`,
		`<script src="/static/revision.js" defer></script>`,
		`<script src="/static/removal-focus.js" defer></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("list page missing %q", want)
		}
	}
	if strings.Contains(body, "autofocus") || strings.Contains(body, "hx-boost") {
		t.Error("list page autofocuses a field or boosts a form")
	}
	if strings.Contains(body, `type="checkbox"`) || strings.Contains(body, "Save task") {
		t.Error("old checkbox rows are still rendered")
	}
	if !strings.Contains(body, `&#34;action&#34;:&#34;remove&#34;`) {
		t.Error("remove item does not post action=remove")
	}
}
