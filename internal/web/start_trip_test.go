package web

import (
	"context"
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
