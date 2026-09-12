package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

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
		r := withItemRoute(packingRequest("/trips/"+trip.ID+"/entries", url.Values{"operationId": {op}, "name": {"Bag"}, "saveForFuture": {"true"}}), trip.ID, "")
		w := httptest.NewRecorder()
		h.AddTripEntry(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Saved to this trip") || !strings.Contains(w.Body.String(), op) {
			t.Fatalf("partial save %d %s", w.Code, w.Body.String())
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
