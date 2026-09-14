package views

import (
	"bytes"
	"context"
	"testing"

	"golang.org/x/net/html"

	"camplist/internal/packing"
)

func renderListItemForm(t *testing.T, form packing.CreateItemForm) *html.Node {
	t.Helper()
	var out bytes.Buffer
	if err := ListItemForm(form, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	return doc
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

func detailsOpen(t *testing.T, doc *html.Node) bool {
	t.Helper()
	details := findElement(doc, func(n *html.Node) bool { return n.Data == "details" })
	if details == nil {
		t.Fatal("add item form has no optional fields toggle")
	}
	_, open := attr(details, "open")
	return open
}

func TestListItemFormKeepsWhoItIsForFolded(t *testing.T) {
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
	if detailsOpen(t, doc) {
		t.Error("who it's for is open on an empty form")
	}
}

func TestListItemFormOpensWhoItIsForWhenFilled(t *testing.T) {
	if !detailsOpen(t, renderListItemForm(t, packing.CreateItemForm{Scope: "person"})) {
		t.Error("who it's for stays folded after the form comes back with a scope")
	}
	if detailsOpen(t, renderListItemForm(t, packing.CreateItemForm{Category: "Light"})) {
		t.Error("a category alone opened the toggle")
	}
}

func TestListItemFormShowsCategoryPicker(t *testing.T) {
	doc := renderListItemForm(t, packing.CreateItemForm{Category: "Light", Categories: packing.CategoryOptions([]string{"Shelter", "Light"}, []string{"Light"})})

	details := findElement(doc, func(n *html.Node) bool { return n.Data == "details" })
	if findElement(details, hasAttr("name", "category")) != nil {
		t.Error("category is hidden behind the toggle")
	}
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

// Delete removes only its row and drops a repeated request.
func TestItemRowDeletesItsRow(t *testing.T) {
	var out bytes.Buffer
	item := packing.NewItem("Tent", "Shelter")
	if err := ItemRow("list", item).Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	button := findElement(doc, hasAttr("hx-delete", "/packing-lists/list/remove-item/"+item.ID))
	if button == nil {
		t.Fatal("row has no delete button")
	}
	for key, want := range map[string]string{"hx-target": "closest li", "hx-swap": "delete", "hx-sync": "closest li:drop"} {
		if got, _ := attr(button, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
