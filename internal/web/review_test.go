package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestReviewSubmissionAndSelectionThroughHTTP(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Weekend", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	r := packingRequest("/packing-session/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}})
	route := chi.NewRouteContext()
	route.URLParams.Add("id", session.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("submit review: %d %s", w.Code, w.Body.String())
	}
	read := httptest.NewRecorder()
	h.ReviewPage(read, r)
	if !strings.Contains(read.Body.String(), "Matches") || !strings.Contains(read.Body.String(), "Apply selected changes") {
		t.Fatal("review cannot be selected")
	}
	current, _ := store.GetPackingList(ctx, list.ID, "user")
	r = packingRequest("/packing-session/"+session.ID+"/review/apply", url.Values{"selected": {"matches"}, "revision": {current.Revision()}}).WithContext(r.Context())
	w = httptest.NewRecorder()
	h.ApplyReviewHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	got, _ := store.GetPackingList(ctx, list.ID, "user")
	if len(got.Items) != 1 || got.Items[0].Name != "Matches" {
		t.Fatal("selected addition not persisted")
	}
}
