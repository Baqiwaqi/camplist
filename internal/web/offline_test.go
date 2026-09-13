package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestOfflineSyncHTTPGuardsAccountAndReturnsConflictState(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	ses, _ := store.CreatePackingSession(ctx, list.ID, "user")
	h := handler{packingStore: store}
	send := func(owner, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/sessions/"+ses.ID+"/sync", strings.NewReader(body))
		r.Header.Set("X-Camplist-Account", owner)
		route := chi.NewRouteContext()
		route.URLParams.Add("id", ses.ID)
		r = r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, "user"))
		w := httptest.NewRecorder()
		h.SyncSessionAPI(w, r)
		return w
	}
	if w := send("user", `{"id":"incomplete","itemId":"`+list.Items[0].ID+`","expectedRevision":0}`); w.Code != 400 {
		t.Fatal("missing desired state must be rejected")
	}
	body, _ := json.Marshal(packing.PackingOperation{ID: "operation", ItemID: list.Items[0].ID, Checked: true})
	if w := send("other", string(body)); w.Code != 409 || !strings.Contains(w.Body.String(), "account") {
		t.Fatal("cross-account queue not blocked")
	}
	if w := send("user", string(body)); w.Code != 200 {
		t.Fatalf("sync failed: %d %s", w.Code, w.Body.String())
	}
	stale, _ := json.Marshal(packing.PackingOperation{ID: "old-offline", ItemID: list.Items[0].ID, Checked: false})
	w := send("user", string(stale))
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"checked":true`) {
		t.Fatal("conflict response missing authoritative state")
	}
}

func TestSharedAPIUsesMemberAccountAndHidesInvitationMetadata(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Shared", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	session, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-session", session.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Email: "private@example.com"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	h := handler{packingStore: store}
	route := chi.NewRouteContext()
	route.URLParams.Add("id", session.ID)
	r := httptest.NewRequest("GET", "/api/sessions/"+session.ID, nil)
	r = r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, "guest"))
	r.Header.Set("X-Camplist-Account", "guest")
	w := httptest.NewRecorder()
	h.SessionAPI(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var body struct {
		Session struct {
			AccountID string `json:"accountId"`
			UserID    string `json:"userId"`
			Shared    bool   `json:"shared"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Session.AccountID != "guest" || body.Session.UserID != "owner" || !body.Session.Shared {
		t.Fatal("actor/owner were not separated", w.Body.String())
	}
	if strings.Contains(w.Body.String(), link.Hash()) || strings.Contains(w.Body.String(), "private@example.com") {
		t.Fatal("private sharing metadata exposed")
	}
	store.RemoveMember(ctx, link.Kind, link.ID, "owner", "guest")
	w = httptest.NewRecorder()
	h.SessionAPI(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "access_removed") {
		t.Fatal("removed access indistinguishable from login", w.Code, w.Body.String())
	}
}

func TestTripAdditionHTTPUsesAuthenticatedIdentityAndTaskSync(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Camping", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	send := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/sessions/"+trip.ID+"/sync", strings.NewReader(body))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", trip.ID)
		r = r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, "user"))
		w := httptest.NewRecorder()
		h.SyncSessionAPI(w, r)
		return w
	}
	w := send(`{"id":"add","itemId":"charge","action":"add","kind":"task","scope":"mine","name":"Charge phone"}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"assignee":"user"`) {
		t.Fatalf("add task %d %s", w.Code, w.Body.String())
	}
	w = send(`{"id":"done","itemId":"charge","kind":"task","checked":true,"expectedRevision":0}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"checked":true`) {
		t.Fatalf("complete task %d %s", w.Code, w.Body.String())
	}
	w = send(`{"id":"spoof","itemId":"fake","action":"add","name":"Fake","assignee":"other"}`)
	if w.Code != 400 {
		t.Fatalf("accepted assignee injection %d", w.Code)
	}
}

func TestSessionAPISnapshotCarriesDefaultCategories(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Rod", "Fishing")}
	store.SavePackingList(ctx, list)
	ses, _ := store.CreatePackingSession(ctx, list.ID, "user")
	h := handler{packingStore: store}

	r := httptest.NewRequest("GET", "/api/sessions/"+ses.ID, nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", ses.ID)
	r = r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, "user"))
	w := httptest.NewRecorder()
	h.SessionAPI(w, r)

	var body struct {
		Session struct {
			Categories []string `json:"categories"`
		} `json:"session"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); w.Code != 200 || err != nil {
		t.Fatalf("session API: %d %v %s", w.Code, err, w.Body.String())
	}
	if !slices.Equal(body.Session.Categories, packing.DefaultCategories) {
		t.Errorf("snapshot categories = %q, want the defaults", body.Session.Categories)
	}
}
