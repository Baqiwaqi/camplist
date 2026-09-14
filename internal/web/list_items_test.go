package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The add item form used to be boosted: every add swapped the whole body,
// scrolled to the top and pushed a history entry whose URL answered 405. htmx
// adds now swap the form, whose category picker suggests the new category,
// append the row and update the parts of the page that count items; a normal form post still redirects.
func TestAddItemAppendsTheRowAndSupportsNormalForms(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	current := func() packing.PackingList {
		t.Helper()
		list, err := store.GetPackingList(ctx, list.ID, "user")
		if err != nil {
			t.Fatal(err)
		}
		return list
	}
	add := func(values url.Values, htmx bool) *httptest.ResponseRecorder {
		r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/add-item", values), list.ID, "")
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		w := httptest.NewRecorder()
		h.AddItemHandler(w, r)
		return w
	}

	opened := current()
	w := add(url.Values{"name": {"Headlamp"}, "category": {"Night hike"}, "scope": {"shared"}, "revision": {opened.Revision()}}, true)
	saved := current()
	if len(saved.Items) != 1 || saved.Items[0].Name != "Headlamp" {
		t.Fatalf("item not saved: %+v", saved.Items)
	}
	body := w.Body.String()
	if w.Code != 200 || w.Header().Get("HX-Refresh") != "" || w.Header().Get("Location") != "" {
		t.Fatalf("htmx add: %d %v", w.Code, w.Header())
	}
	if got := listRevision(t, body); got != saved.Revision() {
		t.Errorf("list revision %q, want %q", got, saved.Revision())
	}
	for _, want := range []string{
		`<form id="add-item"`,
		`aria-invalid:border-red-300" autofocus>`,
		`<ul hx-swap-oob="beforeend:#list-items"><li class="item-row`,
		`id="item-` + saved.Items[0].ID + `"`,
		`<p id="list-summary" class="text-sm text-pine-100" hx-swap-oob="true">1 item</p>`,
		`<p id="list-empty" class="text-muted mb-3" hidden hx-swap-oob="true">`,
		`<option value="Night hike" data-custom data-used>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("add response missing %q", want)
		}
	}
	if strings.Contains(body, "<html") || strings.Contains(body, `value="Headlamp"`) {
		t.Error("add response renders the page or keeps the submitted name in the form")
	}

	// Validation errors come back in the form, without touching the list.
	w = add(url.Values{"name": {" "}, "category": {"Light"}, "revision": {saved.Revision()}}, true)
	body = w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Name is required") || !strings.Contains(body, `aria-invalid="true" autofocus`) || strings.Contains(body, "hx-swap-oob") || strings.Contains(body, "<html") {
		t.Fatalf("htmx validation: %d %s", w.Code, body)
	}

	// An add on top of a write the page has not seen reloads the page.
	if w := add(url.Values{"name": {"Socks"}, "scope": {"shared"}, "revision": {opened.Revision()}}, true); w.Code != 200 || w.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("add over an unseen write: %d %v", w.Code, w.Header())
	}

	if w := add(url.Values{"name": {"Stove"}, "scope": {"shared"}, "revision": {current().Revision()}}, false); w.Code != 303 || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Fatalf("no-script add: %d %q", w.Code, w.Header().Get("Location"))
	}
	if len(current().Items) != 3 {
		t.Fatalf("items: %+v", current().Items)
	}
}

// Delete used to answer HX-Refresh. htmx removes the row itself; the response
// updates the count, the empty text and the list revision.
// Private lists have no revision conflicts, so a stale page still deletes and
// edits, but reloads rather than advance its revision past the unseen write.
func TestRemoveItemUpdatesThePageInPlace(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	remove := func(itemID, revision string) *httptest.ResponseRecorder {
		r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/remove-item/"+itemID, nil), list.ID, itemID)
		r.Method = "DELETE"
		r.Header.Set("HX-Request", "true")
		r.Header.Set("X-Camplist-Revision", revision)
		w := httptest.NewRecorder()
		h.RemoveItemHandler(w, r)
		return w
	}
	opened, _ := store.GetPackingList(ctx, list.ID, "user")
	stale := opened
	opened.Description = "Changed in another tab"
	if err := store.SavePackingList(ctx, opened); err != nil {
		t.Fatal(err)
	}
	w := remove(list.Items[1].ID, stale.Revision())
	if saved, _ := store.GetPackingList(ctx, list.ID, "user"); w.Code != 200 || w.Header().Get("HX-Refresh") != "true" || strings.Contains(w.Body.String(), "list-revision") || len(saved.Items) != 1 {
		t.Fatalf("private list delete from a stale page: %d %v %s items=%d", w.Code, w.Header(), w.Body.String(), len(saved.Items))
	}

	edit := url.Values{"name": {"Tarp tent"}, "category": {"Shelter"}, "scope": {"shared"}, "revision": {stale.Revision()}}
	r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+list.Items[0].ID, edit), list.ID, list.Items[0].ID)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.EditItemHandler(w, r)
	if saved, _ := store.GetPackingList(ctx, list.ID, "user"); w.Code != 200 || w.Header().Get("HX-Refresh") != "true" || strings.Contains(w.Body.String(), "list-revision") || saved.Items[0].Name != "Tarp tent" {
		t.Fatalf("private list edit from a stale page: %d %v %s items=%+v", w.Code, w.Header(), w.Body.String(), saved.Items)
	}

	current, _ := store.GetPackingList(ctx, list.ID, "user")
	w = remove(list.Items[0].ID, current.Revision())
	saved, _ := store.GetPackingList(ctx, list.ID, "user")
	body := w.Body.String()
	if w.Code != 200 || w.Header().Get("HX-Refresh") != "" || len(saved.Items) != 0 {
		t.Fatalf("delete: %d %v items=%d", w.Code, w.Header(), len(saved.Items))
	}
	if got := listRevision(t, body); got != saved.Revision() {
		t.Errorf("list revision %q, want %q", got, saved.Revision())
	}
	for _, want := range []string{
		`<p id="list-summary" class="text-sm text-pine-100" hx-swap-oob="true">No items yet</p>`,
		`<p id="list-empty" class="text-muted mb-3" hx-swap-oob="true">No items yet.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("delete response missing %q", want)
		}
	}
}
