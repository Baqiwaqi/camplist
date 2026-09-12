package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
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
