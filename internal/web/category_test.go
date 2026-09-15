package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
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
	if err := store.RememberCategories(ctx, "user", "fishing"); err != nil {
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

// categoryRequest posts a categories form as the signed-in camper "user".
func categoryRequest(path string, values url.Values, htmx bool, currentURL string) *http.Request {
	r := packingRequest(path, values)
	if htmx {
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Current-URL", currentURL)
	}
	return r
}

type categoriesChanged struct {
	Changed struct {
		From, To              string
		Custom, RenamedInView bool
	} `json:"categories-changed"`
}

func triggered(t *testing.T, w *httptest.ResponseRecorder) categoriesChanged {
	t.Helper()
	var event categoriesChanged
	if err := json.Unmarshal([]byte(w.Header().Get("HX-Trigger")), &event); err != nil {
		t.Fatalf("HX-Trigger %q: %v", w.Header().Get("HX-Trigger"), err)
	}
	return event
}

// typoFixture stores a list "user" owns and a list a friend shared with them,
// both with items under the typo "Fishnig", which "user" remembers.
func typoFixture(t *testing.T) (context.Context, *packing.Store, handler, packing.PackingList, packing.PackingList) {
	t.Helper()
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	own := packing.NewList("user", "Lake", "")
	own.Items = []packing.PackingItem{packing.NewItem("Rod", "Fishnig"), packing.NewItem("Tent", "Shelter"), packing.NewItem("Net", "fishnig")}
	shared := packing.NewList("friend", "Boat", "")
	shared.Items = []packing.PackingItem{packing.NewItem("Tackle", "Fishnig")}
	for _, list := range []packing.PackingList{own, shared} {
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
	own, err = store.GetPackingList(ctx, own.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	return ctx, store, handler{packingStore: store}, own, shared
}

func TestRenameCategoryFromPickerUpdatesTheListInView(t *testing.T) {
	ctx, store, h, own, shared := typoFixture(t)
	if err := store.RememberCategories(ctx, "user", "Fishnig"); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.RenameCategory(w, categoryRequest("/categories/rename", url.Values{"from": {"Fishnig"}, "to": {"Fishing "}, "revision": {own.Revision()}}, true, "http://camplist.test/packing-lists/"+own.ID))
	if w.Code != 200 || w.Header().Get("HX-Refresh") != "" {
		t.Fatalf("rename: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	event := triggered(t, w)
	if event.Changed.From != "Fishnig" || event.Changed.To != "Fishing" || !event.Changed.Custom || !event.Changed.RenamedInView {
		t.Errorf("categories-changed = %+v", event.Changed)
	}
	saved, err := store.GetPackingList(ctx, own.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	// The renamed items regroup under the new name. The rename is made from
	// the picker of a row open for editing, so every row carries hx-preserve:
	// htmx keeps the open row, and what was typed in it, in place.
	for _, want := range []string{
		`<div id="list-items"`,
		`Fishing <span class="font-semibold text-muted">2</span></h3>`,
		`id="item-` + saved.Items[0].ID + `" hx-preserve>`,
		`id="item-` + saved.Items[2].ID + `" hx-preserve>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rename response missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "Fishnig") {
		t.Error("rename response still shows the old name")
	}
	if got := listRevision(t, body); got != saved.Revision() {
		t.Errorf("list revision %q, want %q", got, saved.Revision())
	}
	if got := itemCategoryNames(t, store, shared.ID, "friend"); !slices.Equal(got, []string{"Fishnig"}) {
		t.Errorf("shared-in list changed to %q", got)
	}
	if got, err := store.RememberedCategories(ctx, "user"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered %q, %v", got, err)
	}

	// From another page there is nothing to swap; a list page showing an
	// older revision reloads.
	if err := store.RememberCategories(ctx, "user", "Tarsp"); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.RenameCategory(w, categoryRequest("/categories/rename", url.Values{"from": {"Fishing"}, "to": {"Fiske – ørret"}}, true, "http://camplist.test/trips"))
	if header := w.Header().Get("HX-Trigger"); strings.ContainsFunc(header, func(r rune) bool { return r > 127 }) {
		t.Errorf("HX-Trigger is not ASCII: %q", header)
	}
	if event := triggered(t, w); w.Code != 200 || w.Body.Len() != 0 || event.Changed.To != "Fiske – ørret" || event.Changed.RenamedInView {
		t.Errorf("rename from trips page: %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.RenameCategory(w, categoryRequest("/categories/rename", url.Values{"from": {"Fiske – ørret"}, "to": {"Fishing"}, "revision": {own.Revision()}}, true, "http://camplist.test/packing-lists/"+own.ID))
	if w.Header().Get("HX-Refresh") != "true" {
		t.Errorf("stale list page was not reloaded: %v", w.Header())
	}
}

func itemCategoryNames(t *testing.T, store *packing.Store, listID, userID string) []string {
	t.Helper()
	list, err := store.GetPackingList(context.Background(), listID, userID)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Category)
	}
	return names
}

func TestRenameAndRemoveCategoryWithoutScripts(t *testing.T) {
	ctx, store, h, own, _ := typoFixture(t)
	for _, category := range []string{"Fishnig", "Fishing"} {
		if err := store.RememberCategories(ctx, "user", category); err != nil {
			t.Fatal(err)
		}
	}
	page := httptest.NewRecorder()
	h.CategoriesPage(page, packingRequest("/categories", nil))
	for _, want := range []string{`action="/categories/rename"`, `name="from" value="Fishnig"`, `action="/categories/remove"`, `name="_csrf"`} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("categories page missing %q", want)
		}
	}

	w := httptest.NewRecorder()
	h.RenameCategory(w, categoryRequest("/categories/rename", url.Values{"from": {"fishnig"}, "to": {"FISHING"}}, false, ""))
	if w.Code != 303 || w.Header().Get("Location") != "/categories" {
		t.Errorf("merge without scripts: %d %s", w.Code, w.Body.String())
	}
	if got := itemCategoryNames(t, store, own.ID, "user"); !slices.Equal(got, []string{"Fishing", "Shelter", "Fishing"}) {
		t.Errorf("merged categories %q", got)
	}

	w = httptest.NewRecorder()
	h.RemoveCategory(w, categoryRequest("/categories/remove", url.Values{"name": {"Fishing"}}, false, ""))
	if w.Code != 303 || w.Header().Get("Location") != "/categories" {
		t.Errorf("remove without scripts: %d %s", w.Code, w.Body.String())
	}
	if got, err := store.RememberedCategories(ctx, "user"); err != nil || len(got) != 0 {
		t.Errorf("remembered after remove %q, %v", got, err)
	}
	if got := itemCategoryNames(t, store, own.ID, "user"); !slices.Equal(got, []string{"Fishing", "Shelter", "Fishing"}) {
		t.Errorf("remove changed items to %q", got)
	}
}

func TestRemoveCategoryFromPickerKeepsItems(t *testing.T) {
	ctx, store, h, own, _ := typoFixture(t)
	if err := store.RememberCategories(ctx, "user", "Fishnig"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.RemoveCategory(w, categoryRequest("/categories/remove", url.Values{"name": {"Fishnig"}}, true, "http://camplist.test/packing-lists/"+own.ID))
	event := triggered(t, w)
	if w.Code != 200 || event.Changed.From != "Fishnig" || event.Changed.To != "" {
		t.Errorf("remove: %d %+v", w.Code, event.Changed)
	}
	if got := itemCategoryNames(t, store, own.ID, "user"); !slices.Equal(got, []string{"Fishnig", "Shelter", "fishnig"}) {
		t.Errorf("remove changed items to %q", got)
	}
}

func TestCategoryChangesRefuseDefaultsUnknownAndSignedOut(t *testing.T) {
	_, _, h, _, _ := typoFixture(t)
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
		values  url.Values
		status  int
		message string
	}{
		{"rename default", h.RenameCategory, url.Values{"from": {"shelter"}, "to": {"Tents"}}, 400, "Built-in categories cannot be renamed or removed."},
		{"remove default", h.RemoveCategory, url.Values{"name": {"Other"}}, 400, "Built-in categories cannot be renamed or removed."},
		{"rename to blank", h.RenameCategory, url.Values{"from": {"Fishnig"}, "to": {" "}}, 400, "Type a new name"},
		{"rename unknown", h.RenameCategory, url.Values{"from": {"Paddling"}, "to": {"Kayaking"}}, 404, "no longer in your suggestions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			test.handler(w, categoryRequest("/categories", test.values, true, ""))
			if w.Code != test.status || !strings.Contains(w.Body.String(), test.message) || w.Header().Get("HX-Trigger") != "" {
				t.Errorf("%d %q %v", w.Code, w.Body.String(), w.Header())
			}
		})
	}
	for _, handle := range []http.HandlerFunc{h.RenameCategory, h.RemoveCategory, h.CategoriesPage} {
		r := httptest.NewRequest("POST", "/categories/rename", strings.NewReader("from=Fishnig&to=Fishing&name=Fishnig"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handle(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("signed out: %d", w.Code)
		}
	}
}

func TestCategoryRoutesRequireCSRF(t *testing.T) {
	routes := Routes(Config{Auth: &auth.Auth{}})
	for _, path := range []string{"/categories/rename", "/categories/remove"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader("from=Fishnig&to=Fishing&name=Fishnig"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		csrf.Protect([]byte("01234567890123456789012345678901"), csrf.Secure(false))(routes).ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s without a token: %d", path, w.Code)
		}
	}
}

func TestPickerMarksOnlyRememberedCustomCategories(t *testing.T) {
	ctx, store, h, own, _ := typoFixture(t)
	if err := store.RememberCategories(ctx, "user", "Tarps"); err != nil {
		t.Fatal(err)
	}
	if err := store.ForgetCategory(ctx, "user", "Fishnig"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/packing-lists/"+own.ID, nil)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", own.ID)
	r = r.WithContext(context.WithValue(context.WithValue(r.Context(), auth.USER_ID_KEY, "user"), chi.RouteCtxKey, rc))
	w := httptest.NewRecorder()
	h.ListDetailsPage(w, r)
	body := w.Body.String()
	for _, want := range []string{`<option value="Shelter" data-default data-used>`, `<option value="Fishnig" data-used>`, `<option value="Tarps" data-custom>`, `id="category-dialog"`} {
		if !strings.Contains(body, want) {
			t.Errorf("list page missing %q", want)
		}
	}
}
