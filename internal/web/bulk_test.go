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

	lines := "Documents:\n- Passport\n- Driving licence\n- [ ] Paper map\n\nClimbing:\n* Chalk bag\n"
	w := addSeveral(h, "camper", list.ID, lines, "")
	body := html.UnescapeString(w.Body.String())
	if w.Code != http.StatusOK || !strings.Contains(body, "Added 3 items. Skipped 1 already on this list: Passport.") {
		t.Fatalf("summary: %d %s", w.Code, body)
	}
	if !strings.Contains(body, `<h1 id="bulk-add-title"`) || !strings.Contains(body, `action="/packing-lists/`+list.ID+`/add-several"`) {
		t.Fatal("the result without scripts is not the Add several page with a fresh form")
	}
	want := []string{"Passport/Documents", "Driving licence/Documents", "Paper map/Documents", "Chalk bag/Climbing"}
	if got := savedNames(t, store, list.ID, "camper"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("saved %v, want %v", got, want)
	}

	w = addSeveral(h, "camper", list.ID, "tarp\ntent", "shelter")
	if body := w.Body.String(); !strings.Contains(body, "Added 2 items to Shelter.") {
		t.Fatalf("category for lines without a heading: %s", body)
	}
	w = addSeveral(h, "camper", list.ID, lines, "")
	if body := html.UnescapeString(w.Body.String()); !strings.Contains(body, "Added 0 items. Skipped 4 already on this list: Passport, Driving licence, Paper map, Chalk bag.") {
		t.Fatalf("double submit: %s", body)
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
	client := &http.Client{Jar: jar}

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
		if status := post(url.Values{test.field: {test.value}, "_csrf": {html.UnescapeString(string(token[1]))}}); status != http.StatusOK {
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
	if body := html.UnescapeString(w.Body.String()); w.Code != http.StatusOK || !strings.Contains(body, "Added 2 items from Climbing. Skipped 1 already on this list: Chalk bag.") {
		t.Fatalf("copy: %d %s", w.Code, body)
	}
	want := []string{"Chalk bag/Climbing", "Climbing shoes/Climbing", "Clothes/"}
	if got := savedNames(t, store, weekend.ID, "member"); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("saved %v, want %v", got, want)
	}

	w = addFrom(h, "member", weekend.ID, climbing.ID, shoes, clothes)
	if body := w.Body.String(); !strings.Contains(body, "Added 0 items from Climbing. Skipped 2") {
		t.Fatalf("double submit: %s", body)
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
