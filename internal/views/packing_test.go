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
	if !strings.Contains(out.String(), `hx-delete="/packing-list/`+list.ID+`"`) {
		t.Fatal("delete URL does not match route")
	}
}

func TestSessionsCanBeReopened(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `href="/packing-session/`+session.ID+`"`) {
		t.Fatal("no link to reopen session")
	}
}
