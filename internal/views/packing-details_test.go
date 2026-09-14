package views

import (
	"strings"
	"testing"

	"golang.org/x/net/html"

	"camplist/internal/packing"
)

func renderListItemForm(t *testing.T, form packing.CreateItemForm) *html.Node {
	t.Helper()
	return renderDoc(t, AddItemForm(form, "token", false))
}

func findElement(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, match); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func hasAttr(key, val string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		v, ok := attr(n, key)
		return ok && v == val
	}
}

// The add form shows every field at once: who the item is for is no longer
// folded behind a toggle.
func TestListItemFormShowsEveryField(t *testing.T) {
	doc := renderListItemForm(t, packing.NewCreateItemForm("list"))

	label := findElement(doc, func(n *html.Node) bool { return n.Data == "label" && hasAttr("for", "item-name-new")(n) })
	if label == nil {
		t.Error("item name input has no label")
	}
	name := findElement(doc, hasAttr("id", "item-name-new"))
	if name == nil {
		t.Fatal("add item form missing name input")
	}
	if v, _ := attr(name, "name"); v != "name" {
		t.Errorf("name input posts as %q, want name", v)
	}
	for _, field := range []string{"category", "scope", "revision", "_csrf"} {
		if findElement(doc, hasAttr("name", field)) == nil {
			t.Errorf("add item form missing %q field", field)
		}
	}
	if findElement(doc, func(n *html.Node) bool { return n.Data == "details" }) != nil {
		t.Error("add item form still folds a field behind a toggle")
	}
	if findElement(doc, func(n *html.Node) bool { return n.Data == "button" && text(n) == "Add item" }) == nil {
		t.Error("add item form has no sentence case Add item button")
	}
}

func TestListItemFormShowsCategoryPicker(t *testing.T) {
	doc := renderListItemForm(t, packing.CreateItemForm{Category: "Light", Categories: packing.CategoryOptions([]string{"Shelter", "Light"}, nil, []string{"Light"})})

	input := findElement(doc, hasAttr("id", "item-category-new"))
	if input == nil {
		t.Fatal("add item form has no category field")
	}
	for key, want := range map[string]string{"name": "category", "value": "Light", "role": "combobox", "list": "item-category-new-options"} {
		if got, _ := attr(input, key); got != want {
			t.Errorf("category %s = %q, want %q", key, got, want)
		}
	}
	if findElement(doc, func(n *html.Node) bool { return n.Data == "label" && hasAttr("for", "item-category-new")(n) }) == nil {
		t.Error("category field has no label")
	}
	options := findElement(doc, hasAttr("id", "item-category-new-options"))
	if options == nil {
		t.Fatal("category picker has no options")
	}
	var values []string
	for c := options.FirstChild; c != nil; c = c.NextSibling {
		if v, ok := attr(c, "value"); ok {
			values = append(values, v)
		}
	}
	if len(values) != 2 || values[0] != "Shelter" || values[1] != "Light" {
		t.Errorf("category options = %v", values)
	}
	if findElement(doc, hasAttr("role", "listbox")) == nil {
		t.Error("category picker has no listbox popup")
	}
}

// The add form swaps itself in place instead of boosting a full navigation,
// and still posts normally without scripts.
func TestListItemFormPostsInPlace(t *testing.T) {
	doc := renderListItemForm(t, packing.NewCreateItemForm("list"))
	form := findElement(doc, hasAttr("id", "add-item"))
	if form == nil {
		t.Fatal("add item form has no id to swap")
	}
	for key, want := range map[string]string{"method": "POST", "action": "/packing-lists/list/add-item", "hx-post": "/packing-lists/list/add-item", "hx-target": "this", "hx-swap": "outerHTML", "hx-sync": "this:drop"} {
		if got, _ := attr(form, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if _, boosted := attr(form, "hx-boost"); boosted {
		t.Error("add item form is still boosted")
	}
}

// A resting row carries one ghost Edit and nothing else to press.
func TestItemRowHasOnlyEdit(t *testing.T) {
	item := packing.NewItem("Tent", "Shelter")
	item.Scope = "person"
	doc := renderDoc(t, ItemRow("list", item))
	controls := findAll(doc, func(n *html.Node) bool { return n.Data == "a" || n.Data == "button" })
	if len(controls) != 1 || text(controls[0]) != "Edit" {
		t.Fatalf("row controls = %d, want only Edit", len(controls))
	}
	if v, _ := attr(controls[0], "class"); !strings.Contains(v, "btn-ghost") {
		t.Errorf("Edit class = %q, want a ghost button", v)
	}
	if findElement(doc, hasAttr("hx-delete", "/packing-lists/list/remove-item/"+item.ID)) != nil {
		t.Error("resting row still deletes")
	}
	if !strings.Contains(text(doc), "For each person") {
		t.Error("row does not tag an item packed for each person")
	}
}

// Delete item sits in the edit row, removes only its row and drops a repeated
// request.
func TestItemEditRowDeletesItsRow(t *testing.T) {
	item := packing.NewItem("Tent", "Shelter")
	doc := renderDoc(t, ItemEditRow("list", item.ID, packing.EditItemForm("list", item)))
	button := findElement(doc, hasAttr("hx-delete", "/packing-lists/list/remove-item/"+item.ID))
	if button == nil || text(button) != "Delete item" {
		t.Fatal("edit row has no Delete item button")
	}
	for key, want := range map[string]string{"type": "button", "hx-target": "closest li", "hx-swap": "delete", "hx-sync": "closest li:drop", "data-confirm-action": "Delete item"} {
		if got, _ := attr(button, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// The gear groups by category in order of first use, each group with its
// count, inside the one element the item saves swap.
func TestListGearGroupsByCategory(t *testing.T) {
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen"), packing.NewItem("Tarp", "Shelter"), packing.NewItem("Map", "")}
	doc := renderDoc(t, ListGear(list, list.Items[2].ID, false))

	gear := findElement(doc, hasAttr("id", "list-items"))
	if gear == nil {
		t.Fatal("gear has no swap target")
	}
	if v, _ := attr(gear, "data-empty-focus"); v != "item-name-new" {
		t.Errorf("data-empty-focus = %q", v)
	}
	var heads []string
	for _, h := range findAll(doc, func(n *html.Node) bool { return n.Data == "h3" }) {
		heads = append(heads, strings.Join(strings.Fields(text(h)), " "))
	}
	if want := []string{"Shelter 2", "Kitchen 1", "Other 1"}; strings.Join(heads, "|") != strings.Join(want, "|") {
		t.Errorf("group heads = %q, want %q", heads, want)
	}
	for i, item := range list.Items {
		edit := findElement(doc, hasAttr("id", "edit-"+item.ID))
		if edit == nil {
			t.Fatalf("no row for %s", item.Name)
		}
		if hasAttrKey(edit, "autofocus") != (i == 2) {
			t.Errorf("%s Edit autofocus = %v", item.Name, hasAttrKey(edit, "autofocus"))
		}
	}
	if strings.Contains(text(doc), "No items yet") {
		t.Error("gear with items shows the empty text")
	}

	empty := renderDoc(t, ListGear(packing.NewList("user", "Empty", ""), "", true))
	if !strings.Contains(text(empty), "No items yet") || findElement(empty, func(n *html.Node) bool { return n.Data == "h3" }) != nil {
		t.Error("empty gear does not show only the empty text")
	}
}

func TestListSummaryCountsItemsAndCategories(t *testing.T) {
	item := func(category string) packing.PackingItem { return packing.NewItem("Thing", category) }
	for _, test := range []struct {
		items []packing.PackingItem
		want  string
	}{
		{nil, "No items yet"},
		{[]packing.PackingItem{item("")}, "1 item"},
		{[]packing.PackingItem{item("Shelter")}, "1 item in 1 category"},
		{[]packing.PackingItem{item("Shelter"), item("Kitchen"), item("Shelter"), item("")}, "4 items in 2 categories"},
	} {
		if got := listSummary(test.items); got != test.want {
			t.Errorf("listSummary(%d items) = %q, want %q", len(test.items), got, test.want)
		}
	}
	if got := gearCount(nil); got != "0 items" {
		t.Errorf("gearCount(nil) = %q", got)
	}
}

// Only the owner sees who a list is shared with.
func TestSharedLabelNamesMembersForTheOwner(t *testing.T) {
	list := packing.NewList("owner", "Weekend", "")
	if got := sharedLabel(list, "owner"); got != "" {
		t.Errorf("private list label = %q", got)
	}
	list.Sharing.Members = map[string]packing.Member{"sam": {Subject: "sam", Name: "Sam Rivera"}}
	if got := sharedLabel(list, "owner"); got != "Shared with Sam Rivera" {
		t.Errorf("owner label = %q", got)
	}
	if got := sharedLabel(list, "sam"); got != "Shared" {
		t.Errorf("member label = %q", got)
	}
	list.Sharing.Members["jo"] = packing.Member{Subject: "jo", Name: "Jo"}
	if got := sharedLabel(list, "owner"); got != "Shared with 2 people" {
		t.Errorf("owner label with two members = %q", got)
	}
}
