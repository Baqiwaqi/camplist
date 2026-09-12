package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func TestSessionCardMenuHoldsLinksAndActions(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		`aria-haspopup="menu"`,
		`role="menu"`,
		`role="menuitem" href="/sharing/packing-session/` + session.ID + `"`,
		`role="menuitem" href="/trips/` + session.ID + `/review"`,
		`role="separator"`,
		`hx-delete="/trips/` + session.ID + `"`,
		`data-confirm-action="Delete trip"`,
		`class="menu-item menu-item-danger"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("session card menu missing %q", want)
		}
	}
	if !strings.Contains(body, `hx-headers="{&#34;X-CSRF-Token&#34;:&#34;token&#34;}"`) {
		t.Error("delete action does not send the CSRF header")
	}
}

func TestVisitorsOfSharedSessionsCannotSeeOwnerActions(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("owner", "Camping", ""))
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "guest")
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if strings.Contains(body, "hx-delete") || strings.Contains(body, "/review") {
		t.Error("guest sees delete or review for a session they do not own")
	}
	if !strings.Contains(body, `href="/sharing/packing-session/`+session.ID+`"`) {
		t.Error("guest lost the sharing link")
	}
}

func TestDetailsHeroMenuKeepsStartSessionVisible(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	var out bytes.Buffer
	if err := PackingDetails("Camping", list, packing.NewCreateItemForm(list.ID), "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		`hx-post="/packing-lists/start-session"`,
		`role="menuitem" href="/packing-lists/` + list.ID + `/edit"`,
		`role="menuitem" href="/sharing/packing-list/` + list.ID + `"`,
		`hx-delete="/packing-lists/` + list.ID + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("details hero missing %q", want)
		}
	}
	if strings.Contains(body, `class="btn btn-secondary" href="/packing-lists/`+list.ID+`/edit"`) {
		t.Error("edit is still a loose button next to the menu")
	}
}
