package views

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"camplist/internal/auth"
	"camplist/internal/packing"

	"github.com/a-h/templ"
)

func TestSessionCardMenuHoldsLinksAndActions(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, 0, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	for _, want := range []string{
		`aria-haspopup="menu"`,
		`role="menu"`,
		`x-bind:class="{ 'menu-popup-start': alignStart }"`,
		`role="menuitem" href="/sharing/packing-session/` + session.ID + `"`,
		`role="menuitem" href="/trips/` + session.ID + `/review"`,
		`role="separator"`,
		`hx-delete="/trips/` + session.ID + `"`,
		`data-confirm-action="Delete trip"`,
		`hx-post="/trips/` + session.ID + `/archive"`,
		`hx-target="closest [data-saved-trip]"`,
		`hx-swap="delete"`,
		`hx-sync="closest [data-saved-trip]:drop"`,
		`<div id="trips-empty" class="card" tabindex="-1" hidden>`,
		`class="menu-item menu-item-danger"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("session card menu missing %q", want)
		}
	}
	if !strings.Contains(body, `<body hx-headers="{&#34;X-CSRF-Token&#34;:&#34;token&#34;}">`) {
		t.Error("layout body does not give htmx requests the CSRF header")
	}
	if strings.Count(body, "X-CSRF-Token") != 1 {
		t.Error("delete action repeats the CSRF header the body already sets")
	}
}

func TestShellTurnsOffHtmxHistoryCache(t *testing.T) {
	for name, page := range map[string]templ.Component{
		"layout": Layout("Lists", "token"),
		"public": Shell("Welcome"),
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := page.Render(context.Background(), &out); err != nil {
				t.Fatal(err)
			}
			body := out.String()
			if !strings.Contains(body, `<meta name="htmx-config" content='{"historyCacheSize":0,"refreshOnHistoryMiss":true}'>`) {
				t.Error("htmx history cache is not disabled")
			}
			if name == "public" && strings.Contains(body, "X-CSRF-Token") {
				t.Error("public shell carries a CSRF header")
			}
		})
	}
}

func TestRemoveTaskReliesOnLayoutCSRFHeader(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	vals := removeTaskAttrs(list, packing.PreparationTask{ID: "task"})["hx-vals"].(string)
	if strings.Contains(vals, "_csrf") || !strings.Contains(vals, `"action":"remove"`) {
		t.Errorf("remove task values = %s", vals)
	}
}

func TestVisitorsOfSharedSessionsCannotSeeOwnerActions(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("owner", "Camping", ""))
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "guest")
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, 0, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if strings.Contains(body, "hx-delete") || strings.Contains(body, "/review") || strings.Contains(body, `hx-post="/trips/`+session.ID+`/archive"`) {
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

func TestArchivePageOffersRestoreOnlyForManuallyArchivedTrips(t *testing.T) {
	now := time.Now().UTC()
	manual := packing.NewPackingSession(packing.NewList("user", "Manual", ""))
	manual.ArchivedAt = &now
	packed := packing.NewPackingSession(packing.NewList("user", "Packed", ""))
	packed.List.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	packed.List.Items[0].Checked = true
	packed.List.Items[0].UpdatedAt = now.Add(-48 * time.Hour)
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	var out bytes.Buffer
	if err := ArchivedPackingSessionsPage("Trip archive", []packing.PackingSession{manual, packed}, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	if !strings.Contains(body, `hx-delete="/trips/`+manual.ID+`?view=archive"`) {
		t.Error("archive page delete does not ask for the archive's empty state")
	}
	if !strings.Contains(body, `hx-post="/trips/`+manual.ID+`/restore"`) {
		t.Error("manually archived trip has no Restore")
	}
	if strings.Contains(body, packed.ID+`/restore"`) || strings.Contains(body, `hx-post="/trips/`+manual.ID+`/archive"`) || strings.Contains(body, `hx-post="/trips/`+packed.ID+`/archive"`) {
		t.Error("archive page offers Restore for an automatic archive or Archive again")
	}
}
