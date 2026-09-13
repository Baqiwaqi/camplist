package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func sharingRequest(method, path, actor string, params map[string]string, form url.Values) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	route := chi.NewRouteContext()
	for key, value := range params {
		route.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, actor))
}

func TestListApprovalAsksToShareTripsWithTheSameAccount(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Robin"})
	h := handler{packingStore: store}
	params := map[string]string{"kind": "packing-list", "id": list.ID, "hash": link.Hash()}
	base := "/sharing/packing-list/" + list.ID

	w := httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "owner", params, nil))
	if !strings.Contains(w.Body.String(), `hx-get="`+base+"/invitations/"+link.Hash()+`/approve"`) {
		t.Fatalf("approval does not open the trip prompt:\n%s", w.Body.String())
	}

	r := sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "owner", params, nil)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, r)
	body := w.Body.String()
	for _, want := range []string{"Also share trips with Robin?", `name="trip" value="` + trip.ID + `"`, `value="` + other.ID + `"`, `value="approve"`, "Cancel"} {
		if !strings.Contains(body, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(body, "<html") || strings.Contains(body, "checked") {
		t.Fatalf("prompt must be an unticked fragment:\n%s", body)
	}

	// Without htmx the prompt opens on the full page.
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "owner", params, nil))
	if !strings.Contains(w.Body.String(), "<html") || !strings.Contains(w.Body.String(), "Also share trips with Robin?") {
		t.Fatal("no-script fallback lost the prompt")
	}

	// The requester cannot open the owner's prompt.
	r = sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "guest", params, nil)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, r)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), trip.ID) {
		t.Fatalf("requester saw the trip prompt: %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", base+"/invitations/"+link.Hash(), "owner", params, url.Values{"decision": {"approve"}, "trip": {trip.ID}}))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "guest"); err != nil {
		t.Fatalf("chosen trip not shared: %v", err)
	}
	if _, err := store.GetPackingSession(ctx, other.ID, "guest"); err == nil {
		t.Fatal("unchosen trip shared")
	}
	w = httptest.NewRecorder()
	h.SessionDetailsPage(w, sharingRequest("GET", "/trips/"+trip.ID, "guest", map[string]string{"id": trip.ID}, nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Tent") {
		t.Fatalf("recipient cannot open shared trip: %d", w.Code)
	}
}

func TestListApprovalWithoutTripsApprovesDirectly(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	store.SavePackingList(ctx, list)
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Robin"})
	h := handler{packingStore: store}
	params := map[string]string{"kind": "packing-list", "id": list.ID, "hash": link.Hash()}
	base := "/sharing/packing-list/" + list.ID

	w := httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "owner", params, nil))
	body := w.Body.String()
	if strings.Contains(body, "/approve") || !strings.Contains(body, `name="decision" value="approve"`) {
		t.Fatalf("a list without trips should approve without asking:\n%s", body)
	}
	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", base+"/invitations/"+link.Hash(), "owner", params, url.Values{"decision": {"approve"}}))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	if _, err := store.GetPackingList(ctx, list.ID, "guest"); err != nil {
		t.Fatalf("list not shared: %v", err)
	}

	// Trip choices only belong to list requests.
	trip, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	tripLink, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", "/sharing/packing-session/"+trip.ID+"/invitations/"+tripLink.Hash(), "owner", map[string]string{"kind": "packing-session", "id": trip.ID, "hash": tripLink.Hash()}, url.Values{"decision": {"revoke"}, "trip": {trip.ID}}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trip choice on a trip invitation: %d", w.Code)
	}
}
