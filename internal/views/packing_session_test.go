package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func TestPackingSessionItemShowsAttributionOnlyForOthers(t *testing.T) {
	for _, test := range []struct {
		name        string
		changedByID string
		show        bool
	}{
		{name: "own change", changedByID: "user", show: false},
		{name: "other camper", changedByID: "friend", show: true},
		{name: "legacy without id", changedByID: "", show: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := packing.PackingItem{ID: "tent", Name: "Tent", ChangedBy: "Alex Camper", ChangedByID: test.changedByID}
			var out bytes.Buffer
			ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
			if err := PackingSessionItem("session", item, "token").Render(ctx, &out); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(out.String(), "Alex Camper"); got != test.show {
				t.Fatalf("attribution shown = %v, want %v", got, test.show)
			}
		})
	}
}
