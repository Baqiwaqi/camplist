package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func renderDoc(t *testing.T, c templ.Component) *html.Node {
	t.Helper()
	var out bytes.Buffer
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	if err := c.Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func hasAttrKey(n *html.Node, key string) bool {
	_, ok := attr(n, key)
	return ok
}

func TestItemRowFocusesEditOnlyAfterASwap(t *testing.T) {
	item := packing.NewItem("Tent", "Shelter")
	for _, test := range []struct {
		name  string
		row   templ.Component
		focus bool
	}{
		{"page row", ItemRow("list", item, "token"), false},
		{"swapped row", FocusedItemRow("list", item, "token"), true},
	} {
		edit := findElement(renderDoc(t, test.row), hasAttr("id", "edit-"+item.ID))
		if edit == nil {
			t.Fatalf("%s: Edit link has no stable id", test.name)
		}
		if hasAttrKey(edit, "autofocus") != test.focus {
			t.Errorf("%s: Edit autofocus = %v, want %v", test.name, !test.focus, test.focus)
		}
	}
}

func TestItemEditRowFocusesNameAndDropsRepeatSaves(t *testing.T) {
	item := packing.NewItem("Tent", "Shelter")
	doc := renderDoc(t, ItemEditRow("list", item.ID, packing.EditItemForm("list", item), "token"))
	name := findElement(doc, hasAttr("id", "item-name-"+item.ID))
	if name == nil || !hasAttrKey(name, "autofocus") {
		t.Error("inline edit does not move focus to the name field")
	}
	if findElement(doc, hasAttr("hx-sync", "this:drop")) == nil {
		t.Error("inline edit form can submit twice")
	}
}

func TestStartTripFormDropsRepeatPresses(t *testing.T) {
	list := packing.NewList("owner", "Weekend", "")
	doc := renderDoc(t, StartTripForm(StartTrip{List: list, Name: "Weekend"}, "token"))
	form := findElement(doc, hasAttr("id", "start-trip"))
	if form == nil {
		t.Fatal("missing start trip form")
	}
	for key, want := range map[string]string{"method": "post", "action": "/packing-lists/start-session", "hx-sync": "this:drop", "hx-target": "this"} {
		if got, _ := attr(form, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if button := findElement(form, func(n *html.Node) bool { return n.Data == "button" }); button == nil || hasAttrKey(button, "disabled") {
		t.Error("start trip button is missing or starts disabled")
	}
	if name := findElement(doc, hasAttr("id", "trip-name")); hasAttrKey(name, "autofocus") || hasAttrKey(name, "aria-invalid") {
		t.Error("a fresh start trip form grabs focus or reads as invalid")
	}
}

func TestTripTogglesKeepFocusAndProgressStatusOutsideSwaps(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}}
	session := packing.NewPackingSession(list)
	var out bytes.Buffer
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	if err := PackingSessionPage("Camping", session, true, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "hx-disabled-elt") {
		t.Error("trip toggles still disable buttons, which drops keyboard focus")
	}
	doc, err := html.Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"packing-checklist", "session-preparation"} {
		region := findElement(doc, hasAttr("id", section))
		if region == nil {
			t.Fatalf("missing #%s", section)
		}
		form := findElement(region, func(n *html.Node) bool { return n.Data == "form" })
		if form == nil {
			t.Fatalf("#%s has no toggle form", section)
		}
		if got, _ := attr(form, "hx-sync"); got != "closest section:drop" {
			t.Errorf("#%s toggle hx-sync = %q", section, got)
		}
		if findElement(region, hasAttr("role", "status")) != nil {
			t.Errorf("#%s holds a live region that its swap replaces", section)
		}
	}
	status := findElement(doc, hasAttr("id", "packing-progress-status"))
	if status == nil || !hasAttr("role", "status")(status) || status.FirstChild == nil || status.FirstChild.Data != "0 of 1 items packed" {
		t.Error("trip page lacks a persistent progress status")
	}
}

func TestStartTripMemberPromptStartsUntickedAndDropsRepeatPresses(t *testing.T) {
	list := packing.NewList("owner", "Weekend", "")
	members := []packing.Member{{Subject: "robin", Name: "Robin", Email: "robin@example.com"}, {Subject: "sam", Name: "Sam"}}

	collapsed := renderDoc(t, StartTripForm(StartTrip{List: list, Name: "Weekend", Members: members}, "token"))
	form := findElement(collapsed, hasAttr("id", "start-trip"))
	for key, want := range map[string]string{"method": "get", "action": "/packing-lists/" + list.ID + "/start", "hx-get": "/packing-lists/" + list.ID + "/start", "hx-sync": "this:drop"} {
		if got, _ := attr(form, key); got != want {
			t.Errorf("collapsed %s = %q, want %q", key, got, want)
		}
	}
	if findElement(collapsed, hasAttr("name", "member")) != nil {
		t.Error("collapsed form already shows members")
	}

	doc := renderDoc(t, StartTripForm(StartTrip{List: list, Name: "Weekend", Members: members, Prompt: true}, "token"))
	form = findElement(doc, hasAttr("id", "start-trip"))
	for key, want := range map[string]string{"method": "post", "action": "/packing-lists/start-session", "hx-sync": "this:drop"} {
		if got, _ := attr(form, key); got != want {
			t.Errorf("prompt %s = %q, want %q", key, got, want)
		}
	}
	for i, subject := range []string{"robin", "sam"} {
		box := findElement(doc, hasAttr("value", subject))
		if box == nil || hasAttrKey(box, "checked") {
			t.Fatalf("member %s is missing or starts ticked", subject)
		}
		if hasAttrKey(box, "autofocus") != (i == 0) {
			t.Errorf("focus should land on the first member only, %s autofocus=%v", subject, hasAttrKey(box, "autofocus"))
		}
	}
}
