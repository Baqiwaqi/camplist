package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"camplist/internal/packing"
	"camplist/internal/testsupport"
)

func startTripStore(t *testing.T) (*packing.Store, packing.PackingList) {
	t.Helper()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Camping", "")
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return store, list
}

func tripCount(t *testing.T, store *packing.Store) int {
	t.Helper()
	trips, err := store.ListPackingSession(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	return len(trips)
}

func TestStartTripRedirectsHTMXAndNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(map[bool]string{true: "htmx", false: "form"}[htmx], func(t *testing.T) {
			store, list := startTripStore(t)
			h := handler{packingStore: store}
			r := packingRequest("/packing-lists/start-session", url.Values{"listId": {list.ID}, "name": {"Weekend"}})
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.CreateSessionHandler(w, r)

			trips, err := store.ListPackingSession(context.Background(), "user")
			if err != nil || len(trips) != 1 || trips[0].DisplayName() != "Weekend" {
				t.Fatalf("want one trip named Weekend, got %d (%v)", len(trips), err)
			}
			want := "/trips/" + trips[0].ID
			if htmx {
				if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != want || w.Header().Get("Location") != "" {
					t.Errorf("htmx start got %d HX-Redirect=%q Location=%q", w.Code, w.Header().Get("HX-Redirect"), w.Header().Get("Location"))
				}
				return
			}
			if w.Code != http.StatusSeeOther || w.Header().Get("Location") != want || w.Header().Get("HX-Redirect") != "" {
				t.Errorf("plain start got %d Location=%q HX-Redirect=%q", w.Code, w.Header().Get("Location"), w.Header().Get("HX-Redirect"))
			}
		})
	}
}

func TestStartTripShowsNameErrorInTheForm(t *testing.T) {
	long := strings.Repeat("a", 201)
	for _, htmx := range []bool{true, false} {
		t.Run(map[bool]string{true: "htmx", false: "form"}[htmx], func(t *testing.T) {
			store, list := startTripStore(t)
			h := handler{packingStore: store}
			r := packingRequest("/packing-lists/start-session", url.Values{"listId": {list.ID}, "name": {long}})
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.CreateSessionHandler(w, r)

			if n := tripCount(t, store); n != 0 {
				t.Fatalf("rejected name still started %d trips", n)
			}
			body := w.Body.String()
			if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "" {
				t.Fatalf("got %d HX-Redirect=%q", w.Code, w.Header().Get("HX-Redirect"))
			}
			for _, want := range []string{`id="start-trip"`, "Trip name must be at most 200 characters", `aria-invalid="true"`, `value="` + long + `"`} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			if full := strings.Contains(body, "<html"); full == htmx {
				t.Errorf("htmx=%v but full page=%v", htmx, full)
			}
		})
	}
}

// startSharedTrip posts the start-trip form for actor with the chosen members.
func startSharedTrip(h handler, actor, listID string, members []string, htmx bool) *httptest.ResponseRecorder {
	r := sharingRequest("POST", "/packing-lists/start-session", actor, nil, url.Values{"listId": {listID}, "name": {"Lake weekend"}, "member": members})
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	w := httptest.NewRecorder()
	h.CreateSessionHandler(w, r)
	return w
}

func ownedTrips(t *testing.T, store *packing.Store, owner string) []packing.PackingSession {
	t.Helper()
	all, err := store.ListPackingSession(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	trips := []packing.PackingSession{}
	for _, trip := range all {
		if trip.UserID == owner {
			trips = append(trips, trip)
		}
	}
	return trips
}

func TestListPageAsksOnlyTheOwnerOfASharedListForTripMembers(t *testing.T) {
	store, shared, _ := sharedList(t)
	private := packing.NewList("owner", "Solo", "")
	if err := store.SavePackingList(context.Background(), private); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	for _, test := range []struct {
		name, actor, listID, method string
	}{
		{"private list posts straight away", "owner", private.ID, "post"},
		{"owner of a shared list is asked first", "owner", shared.ID, "get"},
		{"list member starts a private trip", "sam", shared.ID, "post"},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ListDetailsPage(w, sharingRequest("GET", "/packing-lists/"+test.listID, test.actor, map[string]string{"id": test.listID}, nil))
			body := w.Body.String()
			if w.Code != http.StatusOK || !strings.Contains(body, `id="start-trip" method="`+test.method+`"`) {
				t.Fatalf("got %d, want a start-trip form with method %s:\n%s", w.Code, test.method, body)
			}
			if strings.Contains(body, `name="member"`) {
				t.Error("list page shows member checkboxes before Start trip")
			}
		})
	}
}

func TestStartTripPromptOffersListMembersUnticked(t *testing.T) {
	store, list, _ := sharedList(t)
	h := handler{packingStore: store}
	path := "/packing-lists/" + list.ID + "/start"
	params := map[string]string{"id": list.ID}

	t.Run("htmx", func(t *testing.T) {
		r := sharingRequest("GET", path+"?choose=members&name=Lake+weekend", "owner", params, nil)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		h.StartTripPrompt(w, r)
		body := w.Body.String()
		for _, want := range []string{`id="start-trip" method="post"`, `name="member" value="sam"`, "Sam", "sam@example.com", `value="Lake weekend"`, "Select all"} {
			if !strings.Contains(body, want) {
				t.Errorf("prompt missing %q", want)
			}
		}
		if strings.Contains(body, "<html") || strings.Contains(body, `value="sam" checked`) || strings.Contains(body, "guest") || strings.Contains(body, "Robin") {
			t.Errorf("prompt is a full page, pre-ticks members, or offers a pending requester:\n%s", body)
		}

		r = sharingRequest("GET", path+"?name=Lake+weekend", "owner", params, nil)
		r.Header.Set("HX-Request", "true")
		w = httptest.NewRecorder()
		h.StartTripPrompt(w, r)
		if body := w.Body.String(); !strings.Contains(body, `id="start-trip" method="get"`) || strings.Contains(body, `name="member"`) || !strings.Contains(body, "autofocus") {
			t.Errorf("cancel does not restore the collapsed form with focus:\n%s", body)
		}
	})

	t.Run("form", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.StartTripPrompt(w, sharingRequest("GET", path+"?choose=members&name=Lake+weekend", "owner", params, nil))
		body := w.Body.String()
		for _, want := range []string{"<html", `action="/packing-lists/start-session"`, `name="_csrf"`, `name="member" value="sam"`, `href="/packing-lists/` + list.ID + `"`} {
			if !strings.Contains(body, want) {
				t.Errorf("no-script prompt page missing %q", want)
			}
		}
	})

	t.Run("members are not offered to a list member", func(t *testing.T) {
		r := sharingRequest("GET", path+"?choose=members", "sam", params, nil)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		h.StartTripPrompt(w, r)
		if body := w.Body.String(); strings.Contains(body, `name="member"`) || strings.Contains(body, "sam@example.com") {
			t.Errorf("a list member was offered the list's members:\n%s", body)
		}
	})
}

func TestStartTripWithoutMembersStaysPrivate(t *testing.T) {
	store, list, _ := sharedList(t)
	h := handler{packingStore: store}
	w := startSharedTrip(h, "owner", list.ID, nil, true)
	trips := ownedTrips(t, store, "owner")
	if w.Code != http.StatusOK || len(trips) != 1 || w.Header().Get("HX-Redirect") != "/trips/"+trips[0].ID {
		t.Fatalf("got %d HX-Redirect=%q with %d trips", w.Code, w.Header().Get("HX-Redirect"), len(trips))
	}
	if trips[0].IsShared() {
		t.Error("a trip started with nobody ticked is shared")
	}
	if _, err := store.GetPackingSession(context.Background(), trips[0].ID, "sam"); err == nil {
		t.Error("a list member can open a trip they were not added to")
	}
}

func TestStartTripWithChosenMembersSharesTheTrip(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(map[bool]string{true: "htmx", false: "form"}[htmx], func(t *testing.T) {
			store, list, _ := sharedList(t)
			h := handler{packingStore: store}
			w := startSharedTrip(h, "owner", list.ID, []string{"sam"}, htmx)
			trips := ownedTrips(t, store, "owner")
			if len(trips) != 1 {
				t.Fatalf("want one trip, got %d (%d %s)", len(trips), w.Code, w.Body.String())
			}
			trip := trips[0]
			want := "/trips/" + trip.ID
			if htmx && w.Header().Get("HX-Redirect") != want || !htmx && (w.Code != http.StatusSeeOther || w.Header().Get("Location") != want) {
				t.Fatalf("got %d Location=%q HX-Redirect=%q", w.Code, w.Header().Get("Location"), w.Header().Get("HX-Redirect"))
			}
			if trip.DisplayName() != "Lake weekend" {
				t.Errorf("trip name %q", trip.DisplayName())
			}
			if m, ok := trip.Sharing.Members["sam"]; !ok || m.Name != "Sam" || len(trip.Sharing.Members) != 1 {
				t.Errorf("trip members %v, want only Sam", trip.Sharing.Members)
			}

			samTrips, err := store.ListPackingSession(context.Background(), "sam")
			if err != nil || len(samTrips) != 1 || samTrips[0].ID != trip.ID {
				t.Fatalf("Sam's trips %d (%v), want the new trip", len(samTrips), err)
			}
			w = httptest.NewRecorder()
			h.SessionDetailsPage(w, sharingRequest("GET", want, "sam", map[string]string{"id": trip.ID}, nil))
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Lake weekend") {
				t.Fatalf("Sam cannot open the trip: %d", w.Code)
			}
			operationID := "5b0f3c0e-6c7a-4d0b-9d61-0e2b1c7c2f10"
			w = httptest.NewRecorder()
			h.AddTripEntry(w, sharingRequest("POST", want+"/entries", "sam", map[string]string{"id": trip.ID}, url.Values{"operationId": {operationID}, "name": {"Paddle"}}))
			if w.Code != http.StatusSeeOther {
				t.Fatalf("Sam cannot pack on the trip: %d %s", w.Code, w.Body.String())
			}
			w = httptest.NewRecorder()
			h.SharingPage(w, sharingRequest("GET", "/sharing/packing-session/"+trip.ID, "owner", map[string]string{"kind": "packing-session", "id": trip.ID}, nil))
			if !strings.Contains(w.Body.String(), "/sharing/packing-session/"+trip.ID+"/members/sam/remove") {
				t.Error("the owner cannot manage Sam from the trip's sharing page")
			}
		})
	}
}

func TestStartTripRejectsMembersOutsideTheList(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(map[bool]string{true: "htmx", false: "form"}[htmx], func(t *testing.T) {
			store, list, _ := sharedList(t)
			h := handler{packingStore: store}
			// Robin ("guest") has only requested access; "owner" is not a member either.
			for _, members := range [][]string{{"guest"}, {"sam", "stranger"}, {"owner"}} {
				w := startSharedTrip(h, "owner", list.ID, members, htmx)
				body := w.Body.String()
				if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "" || w.Header().Get("Location") != "" {
					t.Fatalf("%v: got %d HX-Redirect=%q Location=%q", members, w.Code, w.Header().Get("HX-Redirect"), w.Header().Get("Location"))
				}
				for _, want := range []string{"no longer a member of this list", `name="member" value="sam"`, `value="Lake weekend"`} {
					if !strings.Contains(body, want) {
						t.Errorf("%v: missing %q", members, want)
					}
				}
				if strings.Contains(body, "robin@example.com") || strings.Contains(body, `value="stranger"`) {
					t.Errorf("%v: the error offers someone outside the list", members)
				}
				if full := strings.Contains(body, "<html"); full == htmx {
					t.Errorf("htmx=%v but full page=%v", htmx, full)
				}
			}
			if n := len(ownedTrips(t, store, "owner")); n != 0 {
				t.Fatalf("rejected members still started %d trips", n)
			}
			for _, subject := range []string{"guest", "sam", "stranger"} {
				if trips, _ := store.ListPackingSession(context.Background(), subject); len(trips) != 0 {
					t.Errorf("%s discovers %d trips after a rejected start", subject, len(trips))
				}
			}
		})
	}
}

func TestListMemberCannotAddMembersToANewTrip(t *testing.T) {
	store, list, _ := sharedList(t)
	h := handler{packingStore: store}
	w := startSharedTrip(h, "sam", list.ID, []string{"sam"}, true)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403", w.Code)
	}
	if n := len(ownedTrips(t, store, "sam")); n != 0 {
		t.Fatalf("forbidden start still created %d trips", n)
	}
}

func TestStartTripNamesPersonCopiesThatExceedTheTripLimit(t *testing.T) {
	store, shared, _ := sharedList(t)
	ctx := context.Background()
	list, err := store.GetPackingList(ctx, shared.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	// 1,995 entries fit on the list; copying the 10 person-scoped ones for Sam makes 2,005.
	for i := 0; i < packing.MaxListEntries-5; i++ {
		item := packing.NewItem(fmt.Sprintf("Item %d", i), "Misc")
		if i < 10 {
			item.Scope = "person"
		}
		list.Items = append(list.Items, item)
	}
	if err = store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	w := startSharedTrip(h, "owner", list.ID, []string{"sam"}, true)
	body := w.Body.String()
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(body, "copied for every camper") || !strings.Contains(body, "5 over") {
		t.Fatalf("got %d %q, want the person copies and how far over the limit", w.Code, body)
	}
	if n := len(ownedTrips(t, store, "owner")); n != 0 {
		t.Fatalf("an oversized trip still started %d trips", n)
	}
	if w = startSharedTrip(h, "owner", list.ID, nil, true); w.Header().Get("HX-Redirect") == "" {
		t.Fatalf("the same list without members did not start: %d %s", w.Code, w.Body.String())
	}
}
