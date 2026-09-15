package views

import (
	"strings"
	"testing"
	"time"

	"camplist/internal/packing"

	"golang.org/x/net/html"
)

func TestPackingSessionItemShowsAttributionOnlyForOthers(t *testing.T) {
	for _, test := range []struct {
		name        string
		changedByID string
		checked     bool
		want        string
	}{
		{name: "own change", changedByID: "user", checked: true, want: ""},
		{name: "other camper packed", changedByID: "friend", checked: true, want: "Packed by Alex Camper"},
		{name: "other camper unpacked", changedByID: "friend", want: "Unpacked by Alex Camper"},
		{name: "legacy without id", changedByID: "", checked: true, want: "Packed by Alex Camper"},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := packing.PackingItem{ID: "tent", Name: "Tent", Checked: test.checked, ChangedBy: "Alex Camper", ChangedByID: test.changedByID}
			doc := renderDoc(t, PackingSessionItem("session", item, false, "token"))
			got := ""
			if captions := findAll(doc, byClass("pack-by")); len(captions) > 0 {
				got = text(captions[0])
			}
			if got != test.want {
				t.Fatalf("caption %q, want %q", got, test.want)
			}
		})
	}
}

// tripFixture is a trip the test viewer ("user") owns: gear in two
// categories, one packed, a personal item, and two preparation tasks.
func tripFixture() packing.PackingSession {
	list := packing.NewList("user", "Weekend car camping", "")
	list.Items = []packing.PackingItem{
		packing.NewItem("Tent", "Shelter"),
		packing.NewItem("Stove", "Kitchen"),
		packing.NewItem("Tarp", " shelter "),
		packing.NewItem("Sleeping bag", "Sleep"),
	}
	list.Items[3].Assignee = "user"
	list.Items[3].AssigneeName = "Tim"
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}, {ID: "site", Name: "Reserve the site"}}
	session := packing.NewPackingSession(list)
	session.List.Items[2].Checked = true
	session.List.Tasks[0].Done = true
	session.CreatedAt = time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)
	session.Name = "Weekend car camping – Sep 13, 2026"
	return session
}

// A private trip's gear is one sheet grouped by category, each group with its
// count; rows are tick buttons only, and a personal item carries Yours.
func TestPackingChecklistGroupsByCategoryWithTickRows(t *testing.T) {
	session := tripFixture()
	doc := renderDoc(t, PackingChecklist(session, "token"))

	if got := len(findAll(doc, byClass("pack-sheet"))); got != 1 {
		t.Fatalf("sheets %d, want 1", got)
	}
	if got := len(findAll(doc, byClass("person-head"))); got != 0 {
		t.Errorf("person heads %d on a private trip, want 0", got)
	}
	var heads []string
	for _, head := range findAll(doc, byClass("group-head")) {
		if head.Data != "h2" {
			t.Errorf("category head is %s, want h2", head.Data)
		}
		heads = append(heads, text(head))
	}
	if got := strings.Join(heads, "|"); got != "Shelter1 of 2|Kitchen0 of 1|Sleep0 of 1" {
		t.Errorf("group heads %q", got)
	}
	if id, _ := attr(findAll(doc, byClass("group-count"))[0], "id"); id != "pack-count-0-0" {
		t.Errorf("first group count id %q", id)
	}
	if body := text(doc); strings.Contains(body, "Unpack") || strings.Contains(body, "Updated by") {
		t.Error("checklist still prints Unpack or Updated by")
	}
	if len(findAll(doc, byClass("pack-action"))) != 0 {
		t.Error("rows still carry a Pack/Unpack pill")
	}
	tags := findAll(doc, byClass("tag"))
	if len(tags) != 1 || text(tags[0]) != "Yours" {
		t.Fatalf("tags %v, want one Yours", tags)
	}
	if row := findElement(doc, hasAttr("id", "pack-"+session.List.Items[3].ID)); row == nil || findElement(row, byClass("tag")) == nil {
		t.Error("the Yours tag is not inside the personal item's button")
	}
	if hasAttrKey(findAll(doc, byClass("pack-row"))[0], "aria-label") {
		t.Error("a row's name should come from its text, which includes the caption and tag")
	}
}

// On a shared trip with personal items the person groups come first, with the
// categories nested inside each.
func TestPackingChecklistNestsCategoriesUnderPeopleOnSharedTrips(t *testing.T) {
	session := tripFixture()
	session.Sharing.Members = map[string]packing.Member{"sam": {Subject: "sam", Name: "Sam Rivera"}}
	sams := packing.NewItem("Sleeping bag", "Sleep")
	sams.Assignee, sams.AssigneeName = "sam", "Sam Rivera"
	session.List.Items = append(session.List.Items, sams)
	doc := renderDoc(t, PackingChecklist(session, "token"))

	var people []string
	for _, head := range findAll(doc, byClass("person-head")) {
		people = append(people, findAllText(head)[0])
	}
	if strings.Join(people, "|") != "Shared|Yours|Sam Rivera" {
		t.Errorf("people %q, want Shared|Yours|Sam Rivera", people)
	}
	if got := len(findAll(doc, byClass("pack-sheet"))); got != 3 {
		t.Errorf("sheets %d, want one per person", got)
	}
	for _, head := range findAll(doc, byClass("group-head")) {
		if head.Data != "h3" {
			t.Errorf("nested category head is %s, want h3", head.Data)
		}
	}
	if len(findAll(doc, byClass("tag"))) != 0 {
		t.Error("the person groups already say whose items are whose")
	}
	if row := findElement(doc, hasAttr("id", "pack-"+sams.ID)); row == nil || !hasAttrKey(row, "disabled") {
		t.Error("another person's item must not be packable")
	}
}

func TestPackingSessionPageHead(t *testing.T) {
	session := tripFixture()
	session.Sharing.Members = map[string]packing.Member{"sam": {Subject: "sam", Name: "Sam Rivera"}}
	doc := renderDoc(t, PackingSessionPage(session.DisplayName(), session, true, nil, "token"))

	if back := findElement(doc, byClass("back-link")); back == nil || text(back) != "Trips" {
		t.Error("missing the back link to Trips")
	}
	if title := findElement(doc, hasAttr("id", "trip-title")); title == nil || text(title) != "Weekend car camping" {
		t.Error("title keeps the start date or lacks its id")
	}
	if meta := findElement(doc, byClass("page-meta")); meta == nil || text(meta) != "Started Sep 13, 2026. Shared with Sam Rivera." {
		t.Error("meta line is not the start date and who it is shared with")
	}
	var items []string
	for _, item := range findAll(findElement(doc, byClass("page-head")), hasAttr("role", "menuitem")) {
		items = append(items, text(item))
	}
	if strings.Join(items, "|") != "Rename trip|Sharing|Archive trip|Delete trip" {
		t.Errorf("More items %q", items)
	}
	if tab := findElement(doc, hasAttr("id", "trip-packing-count")); tab == nil || text(tab) != "1/4" {
		t.Error("Packing tab lacks its count")
	}
	if tab := findElement(doc, hasAttr("id", "trip-tasks-count")); tab == nil || text(tab) != "1/2" {
		t.Error("Before you go tab lacks its count")
	}
	for _, id := range []string{"trip-entry", "trip-rename-dialog"} {
		dialog := findElement(doc, hasAttr("id", id))
		if dialog == nil || dialog.Data != "dialog" || !hasAttrKey(dialog, "data-noscript-inline") {
			t.Errorf("#%s is not a dialog that sits inline without scripts", id)
		}
	}
	open := findElement(doc, hasAttr("id", "trip-entry-open"))
	if open == nil || !hasAttrKey(open, "x-cloak") {
		t.Error("Add to this trip must stay hidden until the script that opens the sheet runs")
	}
	if findElement(doc, hasAttr("id", "future-save-status")) == nil {
		t.Error("missing the offline future-save notices")
	}
}

// A member reads the trip and packs, but renaming, archiving and deleting
// stay with the owner.
func TestPackingSessionPageMenuForMembers(t *testing.T) {
	session := tripFixture()
	session.UserID = "owner"
	session.OwnerName = "Tim"
	session.Sharing.Members = map[string]packing.Member{"user": {Subject: "user", Name: "Sam"}}
	doc := renderDoc(t, PackingSessionPage(session.DisplayName(), session, false, nil, "token"))

	items := findAll(findElement(doc, byClass("page-head")), hasAttr("role", "menuitem"))
	if len(items) != 1 || text(items[0]) != "Sharing" {
		t.Errorf("member More items %d, want Sharing only", len(items))
	}
	if findElement(doc, hasAttr("id", "trip-rename")) != nil {
		t.Error("a member gets the rename form")
	}
	if meta := findElement(doc, byClass("page-meta")); meta == nil || text(meta) != "Started Sep 13, 2026. Shared." {
		t.Errorf("member meta line")
	}
}

// Before you go shows what changed since the last trip, the tasks as tick
// rows and the way to edit them on the list.
func TestSessionPreparationTickRows(t *testing.T) {
	session := tripFixture()
	session.Improvements = []string{"Added: Tent pegs"}
	session.List.Tasks[0].ChangedBy, session.List.Tasks[0].ChangedByID = "Sam", "sam"
	doc := renderDoc(t, SessionPreparation(session, "token"))

	body := text(doc)
	for _, want := range []string{"Since your last trip", "Added: Tent pegs", "Done by Sam", "Edit tasks on the list"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, gone := range []string{"Mark done", "Undo"} {
		if strings.Contains(body, gone) {
			t.Errorf("task rows still say %q", gone)
		}
	}
}

// A toggle answers with its row and, out of band, every group count and the
// Packing tab count, so they follow without resending the checklist.
func TestPackingItemSavedUpdatesCountsOutOfBand(t *testing.T) {
	session := tripFixture()
	doc := renderDoc(t, PackingItemSaved(session, session.List.Items[0], "token"))

	oob := map[string]string{}
	for _, n := range findAll(doc, func(n *html.Node) bool { return hasAttrKey(n, "hx-swap-oob") }) {
		id, _ := attr(n, "id")
		oob[id] = text(n)
	}
	for id, want := range map[string]string{"pack-count-0-0": "1 of 2", "pack-count-0-1": "0 of 1", "trip-packing-count": "1/4"} {
		if got, ok := oob[id]; !ok || got != want {
			t.Errorf("out-of-band #%s = %q, want %q", id, got, want)
		}
	}
	if findElement(doc, hasAttr("id", "packing-checklist")) != nil {
		t.Error("a toggle resends the checklist")
	}
}
