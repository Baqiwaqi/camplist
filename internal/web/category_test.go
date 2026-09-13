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
	for _, match := range regexp.MustCompile(`<option value="([^"]*)"`).FindAllStringSubmatch(datalist[1], -1) {
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
	add(first.ID, "Stove", " kitchen and cooking")

	want := append(slices.Clone(packing.DefaultCategories), "Tarps")
	if got := page(second.ID); !slices.Equal(got, want) {
		t.Errorf("another list offers %q, want %q", got, want)
	}

	add(first.ID, "Spare tarp", "TARPS")

	saved, err := store.GetPackingList(ctx, first.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	var categories []string
	for _, item := range saved.Items {
		categories = append(categories, item.Category)
	}
	if !slices.Equal(categories, []string{"Tarps", "Kitchen and cooking", "TARPS"}) {
		t.Errorf("saved categories = %q", categories)
	}

	want = append(slices.Clone(packing.DefaultCategories), "TARPS")
	if got := page(second.ID); !slices.Equal(got, want) {
		t.Errorf("after recasing another list offers %q, want %q", got, want)
	}
	want = append(slices.Clone(packing.DefaultCategories), "Paddling", "TARPS")
	if got := page(shared.ID); !slices.Equal(got, want) {
		t.Errorf("shared list offers %q, want %q", got, want)
	}
}

func TestEditingAnItemRecasesItsCustomCategory(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	h := handler{packingStore: store}
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Rod", "fishing"), packing.NewItem("Net", "fishing")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberCategory(ctx, "user", "fishing"); err != nil {
		t.Fatal(err)
	}
	rod, net := list.Items[0], list.Items[1]

	editItem := func(item packing.PackingItem, name, category string) {
		t.Helper()
		r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {name}, "category": {category}}), list.ID, item.ID)
		w := httptest.NewRecorder()
		h.EditItemHandler(w, r)
		if w.Code != 303 {
			t.Fatalf("edit %q: status %d %s", category, w.Code, w.Body.String())
		}
	}
	edit := func(category string) { t.Helper(); editItem(rod, "Rod", category) }
	categories := func() []string {
		t.Helper()
		saved, err := store.GetPackingList(ctx, list.ID, "user")
		if err != nil {
			t.Fatal(err)
		}
		return []string{saved.Items[0].Category, saved.Items[1].Category}
	}

	edit(" Fishing ")
	if got := categories(); !slices.Equal(got, []string{"Fishing", "fishing"}) {
		t.Errorf("saved categories %q, want the recased item only", got)
	}
	if got, err := store.RememberedCategories(ctx, "user"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered %q, %v", got, err)
	}

	editItem(net, "Landing net", "fishing")
	if got := categories(); !slices.Equal(got, []string{"Fishing", "fishing"}) {
		t.Errorf("renaming changed saved categories to %q", got)
	}
	if got, err := store.RememberedCategories(ctx, "user"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("renaming an item undid the recase: remembered %q, %v", got, err)
	}

	edit(" shelter")
	if got := categories(); got[0] != "Shelter" {
		t.Errorf("saved default category %q, want its canonical spelling", got[0])
	}
	if got, err := store.RememberedCategories(ctx, "user"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("a default changed the remembered categories: %q, %v", got, err)
	}
}
