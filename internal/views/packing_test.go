package views

import (
	"bytes"
	"camplist/internal/auth"
	"camplist/internal/packing"
	"context"
	"strings"
	"testing"
)

func TestListDeleteURL(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	var out bytes.Buffer
	if err := PackingListPage("Lists", []packing.PackingList{list}, "token").Render(context.WithValue(context.Background(), auth.USER_ID_KEY, "user"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `hx-delete="/packing-lists/`+list.ID+`"`) {
		t.Fatal("delete URL does not match route")
	}
	card := `id="` + PackingListCardID(list.ID) + `"`
	for _, want := range []string{card, `hx-target="#` + PackingListCardID(list.ID) + `"`, `hx-swap="delete"`, `hx-sync="this:drop"`, `id="packing-lists-empty"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("lists page missing %q for in-place delete", want)
		}
	}
}

func TestSessionsCanBeReopened(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, 0, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `href="/trips/`+session.ID+`"`) {
		t.Fatal("no link to reopen session")
	}
}

func TestTripsOverviewLinksToSeparateArchive(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	var overview bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", nil, 2, "token").Render(context.Background(), &overview); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="/trips/archive"`, "Archive (2)"} {
		if !strings.Contains(overview.String(), want) {
			t.Fatalf("overview missing %q", want)
		}
	}

	var archive bytes.Buffer
	if err := ArchivedPackingSessionsPage("Trip archive", []packing.PackingSession{session}, "token").Render(context.Background(), &archive); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Trip archive", `href="/trips"`, session.DisplayName()} {
		if !strings.Contains(archive.String(), want) {
			t.Fatalf("archive missing %q", want)
		}
	}
}
