package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/go-chi/chi/v5"
)

func bulkStore(t *testing.T, lists ...packing.PackingList) *packing.Store {
	t.Helper()
	store := packing.NewStore(testsupport.NewDocuments())
	for _, list := range lists {
		if err := store.SavePackingList(context.Background(), list); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func savedNames(t *testing.T, store *packing.Store, listID, actor string) []string {
	t.Helper()
	list, err := store.GetPackingList(context.Background(), listID, actor)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, item := range list.Items {
		names = append(names, item.Name+"/"+item.Category)
	}
	return names
}

func addSeveral(h handler, actor, listID, lines, category string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.AddSeveralHandler(w, sharingRequest("POST", "/packing-lists/"+listID+"/add-several", actor, map[string]string{"id": listID}, url.Values{"lines": {lines}, "category": {category}}))
	return w
}

func TestAddSeveralReportsWhatItAddedAndSkipped(t *testing.T) {
	list := packing.NewList("camper", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Passport", "Documents")}
	store := bulkStore(t, list)
	h := handler{packingStore: store}

	w := httptest.NewRecorder()
	h.AddSeveralPage(w, sharingRequest("GET", "/packing-lists/"+list.ID+"/add-several", "camper", map[string]string{"id": list.ID}, nil))
	if body := w.Body.String(); !strings.Contains(body, `<h1 id="bulk-add-title"`) || !strings.Contains(body, `action="/packing-lists/`+list.ID+`/add-several"`) {
		t.Fatalf("the step without scripts is not the Add several page: %s", body)
	}

	lines := "Documents:\n- Passport\n- Driving licence\n* Paper map\n\nClimbing:\n- Chalk bag\n"
	// Without scripts a finished add redirects, so a refresh does not repost it.
	w = addSeveral(h, "camper", list.ID, lines, "")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Fatalf("without scripts: %d %q", w.Code, w.Header().Get("Location"))
	}
	want := []string{"Passport/Documents", "Driving licence/Documents", "Paper map/Documents", "Chalk bag/Climbing"}
	if got := savedNames(t, store, list.ID, "camper"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("saved %v, want %v", got, want)
	}

	if w := addSeveral(h, "camper", list.ID, "tarp\ntent", "shelter"); w.Code != http.StatusSeeOther {
		t.Fatalf("category for lines without a heading: %d", w.Code)
	}
	want = append(want, "tarp/Shelter", "tent/Shelter")
	if got := savedNames(t, store, list.ID, "camper"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("saved %v, want %v", got, want)
	}
	if w := addSeveral(h, "camper", list.ID, lines, ""); w.Code != http.StatusSeeOther {
		t.Fatalf("double submit: %d", w.Code)
	}
	if got := savedNames(t, store, list.ID, "camper"); len(got) != 6 {
		t.Fatalf("double submit added items: %v", got)
	}
}

func TestAddSeveralExplainsInvalidPastes(t *testing.T) {
	list := packing.NewList("camper", "Weekend", "")
	store := bulkStore(t, list)
	h := handler{packingStore: store}
	many := strings.Repeat("Item\n", packing.MaxItemsPerAdd+1)
	for _, test := range []struct{ lines, category, want string }{
		{"\n - \n", "", "Type or paste at least one item, one per line."},
		{many, "", "Add at most 100 items at a time. This has 101, so split it in two."},
		{"Tent\n" + strings.Repeat("a", 201), "", "Line 2 is longer than 200 characters."},
		{strings.Repeat("c", 101) + ":\nTent", "", "The category heading above line 2 is longer than 100 characters."},
		{"Tent", strings.Repeat("c", 101), "The category is longer than 100 characters."},
	} {
		w := addSeveral(h, "camper", list.ID, test.lines, test.category)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), test.want) || !strings.Contains(w.Body.String(), `aria-invalid="true"`) {
			t.Errorf("%.20q: %d, want %q in %.300s", test.lines, w.Code, test.want, w.Body.String())
		}
	}
	if got := savedNames(t, store, list.ID, "camper"); len(got) != 0 {
		t.Fatalf("invalid pastes saved %v", got)
	}

	full := packing.NewList("camper", "Full", "")
	for i := 0; i < packing.MaxListEntries; i++ {
		full.Items = append(full.Items, packing.NewItem(fmt.Sprint("Item ", i), ""))
	}
	h = handler{packingStore: bulkStore(t, full)}
	if w := addSeveral(h, "camper", full.ID, "Tent", ""); !strings.Contains(w.Body.String(), "A list holds at most 2,000 items") {
		t.Fatalf("full list: %s", w.Body.String())
	}
}

// conflictingDocuments makes every conditional replacement lose to another
// write, as when other campers keep editing the list during the add.
type conflictingDocuments struct{ *testsupport.Documents }

func (conflictingDocuments) ReplaceItem(context.Context, azcosmos.PartitionKey, string, []byte, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: http.StatusPreconditionFailed}
}

// A write that cannot land is a page-level failure the error toast reports
// (with Reload on 409), not a problem with the pasted lines.
func TestAddSeveralReportsWriteConflictsAsAStatus(t *testing.T) {
	list := packing.NewList("camper", "Weekend", "")
	docs := conflictingDocuments{testsupport.NewDocuments()}
	store := packing.NewStore(docs)
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}

	w := addSeveral(h, "camper", list.ID, "Tent", "")
	if w.Code != http.StatusConflict || strings.Contains(w.Body.String(), "<form") || strings.TrimSpace(w.Body.String()) == "" {
		t.Fatalf("conflict: %d %s", w.Code, w.Body.String())
	}
}

// Bulk add forms post through the same CSRF protection as every other form.
func TestBulkAddRequiresTheCSRFToken(t *testing.T) {
	source := packing.NewList("camper", "Documents", "")
	source.Items = []packing.PackingItem{packing.NewItem("Passport", "")}
	list := packing.NewList("camper", "Weekend", "")
	store := bulkStore(t, source, list)
	h := &handler{packingStore: store}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "camper")))
		})
	})
	router.Get("/packing-lists/{id}/add-several", h.AddSeveralPage)
	router.Post("/packing-lists/{id}/add-several", h.AddSeveralHandler)
	router.Get("/packing-lists/{id}/add-from/{sourceId}", h.AddFromListItemsPage)
	router.Post("/packing-lists/{id}/add-from/{sourceId}", h.AddFromListHandler)
	key := []byte("01234567890123456789012345678901")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		CSRFProtection(key, false, r.Host)(router).ServeHTTP(w, r)
	}))
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	// A finished add redirects to the list page, which this router does not
	// serve, so the redirect itself is the answer under test.
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for i, test := range []struct{ path, field, value string }{
		{"/packing-lists/" + list.ID + "/add-several", "lines", "Tent"},
		{"/packing-lists/" + list.ID + "/add-from/" + source.ID, "item", source.Items[0].ID},
	} {
		res, err := client.Get(srv.URL + test.path)
		if err != nil {
			t.Fatal(err)
		}
		page, _ := io.ReadAll(res.Body)
		res.Body.Close()
		token := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindSubmatch(page)
		if token == nil {
			t.Fatalf("%s: no CSRF field in %s", test.path, page)
		}
		post := func(values url.Values) int {
			r, _ := http.NewRequest("POST", srv.URL+test.path, strings.NewReader(values.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", srv.URL)
			res, err := client.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			return res.StatusCode
		}
		if status := post(url.Values{test.field: {test.value}}); status != http.StatusForbidden {
			t.Errorf("%s without a token: %d", test.path, status)
		}
		if got := savedNames(t, store, list.ID, "camper"); len(got) != i {
			t.Fatalf("%s without a token saved %v", test.path, got)
		}
		if status := post(url.Values{test.field: {test.value}, "_csrf": {html.UnescapeString(string(token[1]))}}); status != http.StatusSeeOther {
			t.Errorf("%s with the token: %d", test.path, status)
		}
	}
	if got := savedNames(t, store, list.ID, "camper"); len(got) != 2 {
		t.Fatalf("with tokens saved %v", got)
	}
}

// sharedSource is a Climbing list owned by "owner" and shared with "member",
// who also owns a Weekend list that already holds a chalk bag.
func sharedSource(t *testing.T) (*packing.Store, packing.PackingList, packing.PackingList) {
	t.Helper()
	ctx := context.Background()
	climbing := packing.NewList("owner", "Climbing", "")
	climbing.Items = []packing.PackingItem{packing.NewItem("Climbing shoes", "Climbing"), packing.NewItem("Chalk bag", "Climbing"), packing.NewItem("Crash pad", "Climbing"), packing.NewItem("Clothes", "")}
	weekend := packing.NewList("member", "Weekend", "")
	weekend.Items = []packing.PackingItem{packing.NewItem("Chalk bag", "Climbing")}
	store := bulkStore(t, climbing, weekend)
	link, _ := store.CreateInvitation(ctx, "packing-list", climbing.ID, "owner")
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "member", Name: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, "packing-list", climbing.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	return store, climbing, weekend
}

func addFrom(h handler, actor, listID, sourceID string, ids ...string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.AddFromListHandler(w, sharingRequest("POST", "/packing-lists/"+listID+"/add-from/"+sourceID, actor, map[string]string{"id": listID, "sourceId": sourceID}, url.Values{"item": ids}))
	return w
}

func TestAddFromListCopiesTickedItemsFromASharedList(t *testing.T) {
	store, climbing, weekend := sharedSource(t)
	h := handler{packingStore: store}
	params := map[string]string{"id": weekend.ID}

	w := httptest.NewRecorder()
	h.AddFromListPage(w, sharingRequest("GET", "/packing-lists/"+weekend.ID+"/add-from", "member", params, nil))
	if body := w.Body.String(); !strings.Contains(body, "Climbing") || !strings.Contains(body, "4 items") || strings.Contains(body, "/add-from/"+weekend.ID) {
		t.Fatalf("sources: %s", body)
	}

	params["sourceId"] = climbing.ID
	w = httptest.NewRecorder()
	h.AddFromListItemsPage(w, sharingRequest("GET", "/packing-lists/"+weekend.ID+"/add-from/"+climbing.ID, "member", params, nil))
	body := w.Body.String()
	if !strings.Contains(body, "4 items, 1 already on this list") || !strings.Contains(body, "Add 3 items") || strings.Count(body, " checked") != 3 || strings.Count(body, " disabled") != 1 {
		t.Fatalf("items step: %s", body)
	}

	shoes, pad, clothes := climbing.Items[0].ID, climbing.Items[2].ID, climbing.Items[3].ID
	// The chalk bag is posted even though its box was disabled; it is skipped.
	w = addFrom(h, "member", weekend.ID, climbing.ID, shoes, climbing.Items[1].ID, clothes)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/packing-lists/"+weekend.ID {
		t.Fatalf("copy: %d %q", w.Code, w.Header().Get("Location"))
	}
	want := []string{"Chalk bag/Climbing", "Climbing shoes/Climbing", "Clothes/"}
	if got := savedNames(t, store, weekend.ID, "member"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("saved %v, want %v", got, want)
	}

	if w := addFrom(h, "member", weekend.ID, climbing.ID, shoes, clothes); w.Code != http.StatusSeeOther {
		t.Fatalf("double submit: %d", w.Code)
	}
	if got := savedNames(t, store, weekend.ID, "member"); len(got) != 3 {
		t.Fatalf("double submit added items: %v", got)
	}

	if w := addFrom(h, "member", weekend.ID, climbing.ID); !strings.Contains(w.Body.String(), "Tick at least one item to add.") {
		t.Fatalf("nothing ticked: %s", w.Body.String())
	}
	if saved := savedNames(t, store, climbing.ID, "owner"); len(saved) != 4 || pad == "" {
		t.Fatalf("the source changed: %v", saved)
	}
}

func TestAddFromListNeedsEditAccessToTheDestination(t *testing.T) {
	store, climbing, weekend := sharedSource(t)
	h := handler{packingStore: store}
	ids := []string{climbing.Items[0].ID}

	// "owner" can read Climbing but has no access to member's Weekend list.
	if w := addFrom(h, "owner", weekend.ID, climbing.ID, ids...); w.Code != http.StatusNotFound {
		t.Fatalf("non-editor at the destination: %d %s", w.Code, w.Body.String())
	}
	// A stranger can open neither list.
	if w := addFrom(h, "stranger", weekend.ID, climbing.ID, ids...); w.Code != http.StatusNotFound {
		t.Fatalf("stranger: %d", w.Code)
	}
	private := packing.NewList("owner", "Private", "")
	private.Items = []packing.PackingItem{packing.NewItem("Secret stash", "")}
	if err := store.SavePackingList(context.Background(), private); err != nil {
		t.Fatal(err)
	}
	if w := addFrom(h, "member", weekend.ID, private.ID, private.Items[0].ID); w.Code != http.StatusNotFound {
		t.Fatalf("unreadable source: %d", w.Code)
	}
	// Removed from the source list, the member can no longer copy from it.
	if err := store.RemoveMember(context.Background(), "packing-list", climbing.ID, "owner", "member"); err != nil {
		t.Fatal(err)
	}
	if w := addFrom(h, "member", weekend.ID, climbing.ID, ids...); w.Code != http.StatusForbidden {
		t.Fatalf("removed member: %d", w.Code)
	}
	if w := addFrom(h, "member", weekend.ID, weekend.ID, weekend.Items[0].ID); w.Code != http.StatusBadRequest {
		t.Fatalf("same list: %d", w.Code)
	}
	if got := savedNames(t, store, weekend.ID, "member"); len(got) != 1 {
		t.Fatalf("rejected copies saved %v", got)
	}
}

func TestAddFromListCopiesMoreThanOnePasteAndStopsAtTheListLimit(t *testing.T) {
	source := packing.NewList("camper", "Weekend camping", "")
	var ids []string
	for i := 0; i < packing.MaxItemsPerAdd+50; i++ {
		item := packing.NewItem(fmt.Sprintf("Item %d", i), "")
		source.Items = append(source.Items, item)
		ids = append(ids, item.ID)
	}
	list := packing.NewList("camper", "Crag day", "")
	full := packing.NewList("camper", "Expedition", "")
	for i := 0; i < packing.MaxListEntries-10; i++ {
		full.Items = append(full.Items, packing.NewItem(fmt.Sprintf("Kit %d", i), ""))
	}
	store := bulkStore(t, source, list, full)
	h := handler{packingStore: store}

	if w := addFrom(h, "camper", list.ID, source.ID, ids...); w.Code != http.StatusSeeOther {
		t.Fatalf("copy of 150 items: %d %s", w.Code, w.Body.String())
	}
	if got := savedNames(t, store, list.ID, "camper"); len(got) != packing.MaxItemsPerAdd+50 {
		t.Fatalf("copied %d items", len(got))
	}
	w := addFrom(h, "camper", full.ID, source.ID, ids...)
	if body := html.UnescapeString(w.Body.String()); w.Code != http.StatusOK || !strings.Contains(body, "A list holds at most 2,000 items and preparation tasks together.") {
		t.Fatalf("copy past the list limit: %d %s", w.Code, body)
	}
	if got := savedNames(t, store, full.ID, "camper"); len(got) != packing.MaxListEntries-10 {
		t.Fatalf("a copy past the limit saved %d items", len(got))
	}
}

func TestAddFromListExplainsWhyItemsCannotBeCopied(t *testing.T) {
	source := packing.NewList("camper", "Old list", "")
	source.Items = []packing.PackingItem{packing.NewItem(strings.Repeat("a", packing.MaxItemNameLength+1), "")}
	list := packing.NewList("camper", "Weekend", "")
	h := handler{packingStore: bulkStore(t, source, list)}

	if body := addFrom(h, "camper", list.ID, source.ID, source.Items[0].ID).Body.String(); !strings.Contains(body, "Some of those items cannot be copied.") || strings.Contains(body, "no longer on") {
		t.Fatalf("invalid source item: %s", body)
	}
	if body := addFrom(h, "camper", list.ID, source.ID, "gone").Body.String(); !strings.Contains(body, "Those items are no longer on Old list.") {
		t.Fatalf("missing selection: %s", body)
	}
}

func htmxRequest(r *http.Request) *http.Request {
	r.Header.Set("HX-Request", "true")
	return r
}

func TestBulkAddsUpdateTheListPageInPlace(t *testing.T) {
	ctx := context.Background()
	store, climbing, weekend := sharedSource(t)
	h := handler{packingStore: store}
	page, _ := store.GetPackingList(ctx, weekend.ID, "member")

	// The dialog steps are fragments.
	for _, get := range []struct {
		path    string
		handler http.HandlerFunc
		want    string
	}{
		{"/packing-lists/" + weekend.ID + "/add-several", h.AddSeveralPage, `<h2 id="bulk-add-title" class="dialog-title focus-visible:outline-none">Add several items</h2>`},
		{"/packing-lists/" + weekend.ID + "/add-from", h.AddFromListPage, `hx-target="#bulk-add-panel"`},
		{"/packing-lists/" + weekend.ID + "/add-from/" + climbing.ID, h.AddFromListItemsPage, `hx-post="/packing-lists/` + weekend.ID + `/add-from/` + climbing.ID + `"`},
	} {
		w := httptest.NewRecorder()
		get.handler(w, htmxRequest(sharingRequest("GET", get.path, "member", map[string]string{"id": weekend.ID, "sourceId": climbing.ID}, nil)))
		if body := w.Body.String(); strings.Contains(body, "<html") || !strings.Contains(body, get.want) {
			t.Errorf("%s: %.400s", get.path, body)
		}
	}

	// A rejected paste comes back into the dialog.
	w := httptest.NewRecorder()
	h.AddSeveralHandler(w, htmxRequest(sharingRequest("POST", "/", "member", map[string]string{"id": weekend.ID}, url.Values{"lines": {" "}, "revision": {page.Revision()}})))
	if body := w.Body.String(); strings.Contains(body, "<html") || !strings.Contains(body, "Type or paste at least one item") || strings.Contains(body, "hx-swap-oob") {
		t.Fatalf("rejected paste: %s", body)
	}

	w = httptest.NewRecorder()
	h.AddSeveralHandler(w, htmxRequest(sharingRequest("POST", "/", "member", map[string]string{"id": weekend.ID}, url.Values{"lines": {"Documents:\nPassport\nchalk bag"}, "revision": {page.Revision()}})))
	saved, _ := store.GetPackingList(ctx, weekend.ID, "member")
	body := w.Body.String()
	for _, want := range []string{
		`<p id="list-add-status" role="status" class="mt-3 rounded-card bg-pine-100 px-4 py-3 text-pine-900 empty:hidden" hx-swap-oob="innerHTML">Added 1 item to Documents. Skipped 1 already on this list: chalk bag.</p>`,
		`<ul hx-swap-oob="beforeend:#list-items">`,
		`id="list-summary"`,
		`id="list-empty"`,
		`<div id="list-bulk-actions" class="mt-1 border-t border-sand-100 pt-3" hx-swap-oob="true">`,
		`id="list-revision" value="` + html.EscapeString(saved.Revision()) + `" hx-swap-oob="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("add several response missing %q in %s", want, body)
		}
	}
	// Everything is out of band, which leaves the dialog's panel empty and closes it.
	if strings.Contains(body, "<html") || strings.Count(body, "Passport") != 1 || w.Header().Get("HX-Refresh") != "" {
		t.Fatalf("add several response: %s", body)
	}

	// Nothing new: only the summary changes.
	w = httptest.NewRecorder()
	h.AddSeveralHandler(w, htmxRequest(sharingRequest("POST", "/", "member", map[string]string{"id": weekend.ID}, url.Values{"lines": {"Passport"}, "revision": {saved.Revision()}})))
	if body := w.Body.String(); !strings.Contains(body, "Added 0 items. Skipped 1") || strings.Contains(body, "list-items") || strings.Contains(body, "list-revision") {
		t.Fatalf("skipped everything: %s", body)
	}

	// Copying on top of a revision the page did not show reloads the page.
	w = httptest.NewRecorder()
	h.AddFromListHandler(w, htmxRequest(sharingRequest("POST", "/", "member", map[string]string{"id": weekend.ID, "sourceId": climbing.ID}, url.Values{"item": {climbing.Items[2].ID}, "revision": {page.Revision()}})))
	if w.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("stale page was not reloaded: %v %s", w.Header(), w.Body.String())
	}
	saved, _ = store.GetPackingList(ctx, weekend.ID, "member")
	w = httptest.NewRecorder()
	h.AddFromListHandler(w, htmxRequest(sharingRequest("POST", "/", "member", map[string]string{"id": weekend.ID, "sourceId": climbing.ID}, url.Values{"item": {climbing.Items[0].ID}, "revision": {saved.Revision()}})))
	if body := w.Body.String(); !strings.Contains(body, "Added 1 item from Climbing.") || !strings.Contains(body, "Climbing shoes") || w.Header().Get("HX-Refresh") != "" {
		t.Fatalf("copy in place: %s", body)
	}
}

func TestListPageOffersBulkAddAndNewListOpensIt(t *testing.T) {
	store := bulkStore(t)
	h := handler{packingStore: store}
	w := httptest.NewRecorder()
	h.NewListHandler(w, sharingRequest("POST", "/packing-lists/new", "camper", nil, url.Values{"name": {"Climbing"}}))
	lists, _ := store.GetPackingLists(context.Background(), "camper")
	if w.Code != http.StatusSeeOther || len(lists) != 1 || w.Header().Get("Location") != "/packing-lists/"+lists[0].ID {
		t.Fatalf("new list: %d %q", w.Code, w.Header().Get("Location"))
	}

	w = httptest.NewRecorder()
	h.ListDetailsPage(w, sharingRequest("GET", "/packing-lists/"+lists[0].ID, "camper", map[string]string{"id": lists[0].ID}, nil))
	body := w.Body.String()
	for _, want := range []string{
		`<div id="list-empty" class="mb-3 grid justify-items-start gap-3">`,
		`<dialog id="bulk-add"`,
		`<div id="list-bulk-actions" class="mt-1 border-t border-sand-100 pt-3" hidden>`,
		`<p id="list-add-status" role="status"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("list page missing %q", want)
		}
	}
	// Both actions sit in the empty state; the add card's pair stays hidden until items arrive.
	if strings.Count(body, `href="/packing-lists/`+lists[0].ID+`/add-several"`) != 2 || strings.Count(body, `href="/packing-lists/`+lists[0].ID+`/add-from"`) != 2 {
		t.Fatalf("bulk add actions: %s", body)
	}
}

// countingDocuments counts the conditional writes a request makes, so a bulk
// add can be held to one list write plus one remembered-categories write.
type countingDocuments struct {
	*testsupport.Documents
	replaced int
}

func (d *countingDocuments) ReplaceItem(ctx context.Context, key azcosmos.PartitionKey, id string, body []byte, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	d.replaced++
	return d.Documents.ReplaceItem(ctx, key, id, body, options)
}

func TestBulkAddRemembersEveryCategoryInOneWrite(t *testing.T) {
	ctx := context.Background()
	docs := &countingDocuments{Documents: testsupport.NewDocuments()}
	store := packing.NewStore(docs)
	source := packing.NewList("camper", "Kit", "")
	var ids []string
	for i := 0; i < 20; i++ {
		item := packing.NewItem(fmt.Sprintf("Item %d", i), fmt.Sprintf("Custom %d", i))
		source.Items = append(source.Items, item)
		ids = append(ids, item.ID)
	}
	list := packing.NewList("camper", "Weekend", "")
	for _, saved := range []packing.PackingList{source, list} {
		if err := store.SavePackingList(ctx, saved); err != nil {
			t.Fatal(err)
		}
	}
	h := handler{packingStore: store}

	docs.replaced = 0
	if w := addFrom(h, "camper", list.ID, source.ID, ids...); w.Code != http.StatusSeeOther {
		t.Fatalf("copy: %d %s", w.Code, w.Body.String())
	}
	if docs.replaced > 2 {
		t.Errorf("a copy of 20 categories made %d conditional writes, want the list write plus one category write", docs.replaced)
	}
	got, err := store.RememberedCategories(ctx, "camper")
	if err != nil || len(got) != 20 {
		t.Fatalf("remembered %q, %v", got, err)
	}
}
