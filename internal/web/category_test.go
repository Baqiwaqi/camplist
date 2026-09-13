package web

import (
	"context"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"

	"github.com/go-chi/chi/v5"
)

// categoryOptions reads the values a rendered category picker offers.
func categoryOptions(t *testing.T, body string) []string {
	t.Helper()
	datalist := regexp.MustCompile(`(?s)<datalist id="item-category-new-options">(.*?)</datalist>`).FindStringSubmatch(body)
	if datalist == nil {
		t.Fatal("page has no category picker for new items")
	}
	var values []string
	for _, match := range regexp.MustCompile(`<option value="([^"]*)">`).FindAllStringSubmatch(datalist[1], -1) {
		values = append(values, match[1])
	}
	return values
}

func TestSavedCategoriesAreOfferedOnEveryList(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	h := handler{packingStore: store}
	first := packing.NewList("user", "Weekend", "")
	second := packing.NewList("user", "Winter", "")
	shared := packing.NewList("friend", "Friend's gear", "")
	shared.Items = []packing.PackingItem{packing.NewItem("Kayak", "Paddling")}
	for _, list := range []packing.PackingList{first, second, shared} {
		if err := store.SavePackingList(ctx, list); err != nil {
			t.Fatal(err)
		}
	}
	link, err := store.CreateInvitation(ctx, "packing-list", shared.ID, "friend")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "user", Name: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "friend", link.Hash(), true); err != nil {
		t.Fatal(err)
	}

	add := func(listID, name, category string) {
		t.Helper()
		r := packingRequest("/packing-lists/"+listID+"/add-item", url.Values{"name": {name}, "category": {category}})
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", listID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
		w := httptest.NewRecorder()
		h.AddItemHandler(w, r)
		if w.Code != 303 {
			t.Fatalf("add %s: status %d %s", name, w.Code, w.Body.String())
		}
	}
	page := func(listID string) []string {
		t.Helper()
		r := httptest.NewRequest("GET", "/packing-lists/"+listID, nil)
		rc := chi.NewRouteContext()
		rc.URLParams.Add("id", listID)
		r = r.WithContext(context.WithValue(context.WithValue(r.Context(), auth.USER_ID_KEY, "user"), chi.RouteCtxKey, rc))
		w := httptest.NewRecorder()
		h.ListDetailsPage(w, r)
		return categoryOptions(t, w.Body.String())
	}

	if got := page(second.ID); !slices.Equal(got, packing.DefaultCategories) {
		t.Errorf("new camper is offered %q, want the defaults", got)
	}

	add(first.ID, "Tarp", " Tarps ")
	add(first.ID, "Spare tarp", "TARPS")
	add(first.ID, "Stove", " kitchen and cooking")

	saved, err := store.GetPackingList(ctx, first.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	var categories []string
	for _, item := range saved.Items {
		categories = append(categories, item.Category)
	}
	if !slices.Equal(categories, []string{"Tarps", "Tarps", "Kitchen and cooking"}) {
		t.Errorf("saved categories = %q", categories)
	}

	want := append(slices.Clone(packing.DefaultCategories), "Tarps")
	if got := page(second.ID); !slices.Equal(got, want) {
		t.Errorf("another list offers %q, want %q", got, want)
	}
	want = append(slices.Clone(packing.DefaultCategories), "Paddling", "Tarps")
	if got := page(shared.ID); !slices.Equal(got, want) {
		t.Errorf("shared list offers %q, want %q", got, want)
	}
}

func TestEditingAnItemRemembersItsNewCategory(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Rod", "")}
	store := &fakePackingStore{list: list, remembered: []string{"Fishing"}}
	h := handler{packingStore: store}
	item := list.Items[0]

	r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {"Rod"}, "category": {" FISHING "}}), list.ID, item.ID)
	h.EditItemHandler(httptest.NewRecorder(), r)
	if got := store.list.Items[0].Category; got != "Fishing" {
		t.Errorf("saved category %q, want the remembered spelling", got)
	}

	r = withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {"Rod"}, "category": {"Bait"}}), list.ID, item.ID)
	h.EditItemHandler(httptest.NewRecorder(), r)
	if got := store.list.Items[0].Category; got != "Bait" || !slices.Contains(store.remembered, "Bait") {
		t.Errorf("saved %q, remembered %q", got, store.remembered)
	}
}
