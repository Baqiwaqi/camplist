package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newTrip saves a list with one item and starts a trip owned by "user".
func newTrip(t *testing.T) (*packing.Store, packing.PackingSession) {
	t.Helper()
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	return store, trip
}

func TestRenameTripSwapsInPlaceAndSupportsNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store, trip := newTrip(t)
			h := handler{packingStore: store}
			r := withItemRoute(packingRequest("/trips/"+trip.ID+"/name", url.Values{"name": {"Lakeside weekend"}}), trip.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.RenameTrip(w, r)
			saved, err := store.GetPackingSession(context.Background(), trip.ID, "user")
			if err != nil || saved.Name != "Lakeside weekend" {
				t.Fatalf("trip not renamed: %q %v", saved.Name, err)
			}
			if !htmx {
				if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/trips/"+trip.ID {
					t.Fatalf("normal form missing redirect: %d %q", w.Code, w.Header().Get("Location"))
				}
				return
			}
			body := w.Body.String()
			for _, want := range []string{
				"<title>Lakeside weekend – Camplist</title>",
				`<h1 id="trip-title" hx-swap-oob="true">Lakeside weekend</h1>`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q in\n%s", want, body)
				}
			}
			// The form and its focused field stay in the page.
			if w.Code != http.StatusOK || strings.Contains(body, "<html") || strings.Contains(body, "<form") {
				t.Errorf("returned more than the heading and title: %d\n%s", w.Code, body)
			}
			if w.Header().Get("HX-Trigger") != "camplist:trip-renamed" {
				t.Error("rename does not tell the offline copy to refresh")
			}
		})
	}
}

func TestAddTripEntryReturnsSectionsAndSupportsNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store, trip := newTrip(t)
			h := handler{packingStore: store}
			const op = "0b8f5f1e-5bd4-4c0a-9d35-3c4f3f4d1a2b"
			r := withItemRoute(packingRequest("/trips/"+trip.ID+"/entries", url.Values{"operationId": {op}, "name": {"Head torch"}}), trip.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.AddTripEntry(w, r)
			if !htmx {
				if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/trips/"+trip.ID {
					t.Fatalf("normal form missing redirect: %d", w.Code)
				}
				return
			}
			body := w.Body.String()
			for _, want := range []string{
				`id="packing-checklist"`, "Head torch", "0 of 2 items packed",
				`id="trip-entry-operation" name="operationId"`,
				`<datalist id="trip-categories" hx-swap-oob="true"><option value="Shelter" data-default>`,
				`<div id="trip-future-save" role="status" hx-swap-oob="innerHTML"></div>`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			// The entry form stays in the page, and an item leaves preparation alone.
			if w.Code != http.StatusOK || strings.Contains(body, "<html") || strings.Contains(body, "<form id=\"trip-entry-form\"") || strings.Contains(body, `id="session-preparation"`) || strings.Contains(body, `value="`+op+`" hx-swap-oob`) {
				t.Errorf("fragment is a page, replaces the form or reuses the operation ID:\n%s", body)
			}
		})
	}
}

func TestTripFormPartialSaveCanRetryWithoutDuplicatingEntry(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	list.Tasks = []packing.PreparationTask{{ID: "charge", Name: "Charge phone", Scope: "person"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner", "Alex")
	if err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "user", Name: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DecideInvitation(ctx, link.Kind, trip.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	const op = "5f5e7c5d-cb59-4d42-9867-6d2453970bc6"
	for i := 0; i < 2; i++ {
		htmx := i == 1
		r := withItemRoute(packingRequest("/trips/"+trip.ID+"/entries", url.Values{"operationId": {op}, "name": {"Bag"}, "saveForFuture": {"true"}}), trip.ID, "")
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		w := httptest.NewRecorder()
		h.AddTripEntry(w, r)
		body := w.Body.String()
		if w.Code != 200 || !strings.Contains(body, "Saved to this trip") || !strings.Contains(body, `name="operationId" value="`+op+`"`) {
			t.Fatalf("partial save %d %s", w.Code, body)
		}
		// Without htmx the result is its own page; with htmx it is a notice in the entry section.
		if fragment := !strings.Contains(body, "<html"); fragment != htmx {
			t.Fatalf("htmx %v rendered fragment %v", htmx, fragment)
		}
		if htmx && (!strings.Contains(body, `<div id="trip-future-save" role="status" hx-swap-oob="innerHTML"><div`) || !strings.Contains(body, `id="packing-checklist"`) || strings.Contains(body, `id="session-preparation"`)) {
			t.Fatalf("partial save fragment lacks the inline notice:\n%s", body)
		}
	}
	saved, err := store.GetPackingSession(ctx, trip.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.List.Items) != 1 {
		t.Fatalf("retry duplicated entry: %d", len(saved.List.Items))
	}
	r := withItemRoute(httptest.NewRequest("GET", "/trips/"+trip.ID, nil), trip.ID, "")
	r = r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "user"))
	w := httptest.NewRecorder()
	h.SessionDetailsPage(w, r)
	if !strings.Contains(w.Body.String(), "<h3>Alex</h3>") || !strings.Contains(w.Body.String(), "<h3>Sam</h3>") {
		t.Fatal("server task fallback lacks participant groups")
	}
}

// sharedTrip stores a trip owned by "owner" and shared with "member".
func sharedTrip(t *testing.T) (*packing.Store, packing.PackingSession) {
	t.Helper()
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner", "Alex")
	if err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "member", Name: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DecideInvitation(ctx, link.Kind, trip.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	return store, trip
}

// visibleEmptyState is the out-of-band empty state without its hidden attribute.
const visibleEmptyState = `<div id="trips-empty" class="card" tabindex="-1" hx-swap-oob="true">`

// tripRequest sends method to path as userID with the trip id route param.
func tripRequest(method, path, userID, tripID string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("HX-Request", "true")
	return withItemRoute(r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, userID)), tripID, "")
}

func TestDeleteTripRemovesOnlyTheOwnersTripCard(t *testing.T) {
	ctx := context.Background()
	store, trip := sharedTrip(t)
	h := handler{packingStore: store}
	deleteAs := func(userID, tripID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.DeletePackingSession(w, tripRequest("DELETE", "/trips/"+tripID+"?view=archive", userID, tripID))
		return w
	}

	if w := deleteAs("member", trip.ID); w.Code != http.StatusForbidden {
		t.Fatalf("member delete got %d, want 403", w.Code)
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "owner"); err != nil {
		t.Fatalf("member delete removed the trip: %v", err)
	}
	if w := deleteAs("stranger", trip.ID); w.Code == http.StatusOK {
		t.Fatal("a user without access deleted the trip")
	}

	w := deleteAs("owner", trip.ID)
	if w.Code != http.StatusOK || w.Header().Get("HX-Refresh") != "" {
		t.Fatalf("owner delete got %d refresh=%q, want 200 without a page refresh", w.Code, w.Header().Get("HX-Refresh"))
	}
	if body := w.Body.String(); !strings.Contains(body, visibleEmptyState) || !strings.Contains(body, "No archived trips yet.") || strings.Contains(body, "trip-archive-link") {
		t.Fatalf("deleting the last archive card does not reveal the archive's empty state: %s", body)
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "owner"); err == nil {
		t.Fatal("owner delete left the trip in storage")
	}
	if w := deleteAs("owner", trip.ID); w.Code != http.StatusNotFound {
		t.Fatalf("repeat delete got %d, want 404", w.Code)
	}
}

func TestArchiveAndRestoreTripAreOwnerOnlyAndHideItForMembers(t *testing.T) {
	ctx := context.Background()
	store, trip := sharedTrip(t)
	h := handler{packingStore: store}
	archivedFor := func(userID string) bool {
		sessions, err := store.ListPackingSession(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		_, archived := packing.PartitionSessions(sessions, time.Now().UTC())
		return len(archived) == 1 && archived[0].ID == trip.ID
	}

	w := httptest.NewRecorder()
	h.ArchiveTrip(w, tripRequest("POST", "/trips/"+trip.ID+"/archive", "member", trip.ID))
	if w.Code != http.StatusForbidden || archivedFor("owner") {
		t.Fatalf("member archive got %d, archived=%v; want 403 and the trip still active", w.Code, archivedFor("owner"))
	}

	w = httptest.NewRecorder()
	h.ArchiveTrip(w, tripRequest("POST", "/trips/"+trip.ID+"/archive", "owner", trip.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("owner archive got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="trip-archive-link"`) || !strings.Contains(body, "Archive (1)") || !strings.Contains(body, visibleEmptyState) || !strings.Contains(body, "No trips in progress.") {
		t.Fatalf("archiving the last trip does not update the archive count and empty state: %s", body)
	}
	if !archivedFor("owner") || !archivedFor("member") {
		t.Fatal("archived trip is still active for the owner or the member")
	}

	w = httptest.NewRecorder()
	h.RestoreTrip(w, tripRequest("POST", "/trips/"+trip.ID+"/restore", "member", trip.ID))
	if w.Code != http.StatusForbidden || !archivedFor("owner") {
		t.Fatalf("member restore got %d; want 403 and the trip still archived", w.Code)
	}

	w = httptest.NewRecorder()
	h.RestoreTrip(w, tripRequest("POST", "/trips/"+trip.ID+"/restore", "owner", trip.ID))
	if w.Code != http.StatusOK || archivedFor("owner") || archivedFor("member") {
		t.Fatalf("owner restore got %d; want the trip active again for everyone", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, visibleEmptyState) || !strings.Contains(body, "No archived trips yet.") {
		t.Fatalf("restoring the last archived trip does not reveal the archive's empty state: %s", body)
	}

	w = httptest.NewRecorder()
	h.ArchiveTrip(w, tripRequest("POST", "/trips/missing/archive", "owner", "missing"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("archiving a missing trip got %d, want 404", w.Code)
	}
}

func TestAddTripTaskSwapsOnlyPreparation(t *testing.T) {
	store, trip := newTrip(t)
	h := handler{packingStore: store}
	r := withItemRoute(packingRequest("/trips/"+trip.ID+"/entries", url.Values{"operationId": {"8c1d8b7e-3f7a-4c55-a0d4-5b7b9d8f2e10"}, "name": {"Buy gas"}, "kind": {"task"}}), trip.ID, "")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AddTripEntry(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, `id="session-preparation"`) || !strings.Contains(body, "Buy gas") || strings.Contains(body, `id="packing-checklist"`) {
		t.Fatalf("task add did not return only the preparation section: %d\n%s", w.Code, body)
	}
}
