package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestInvalidNewListPreservesForm(t *testing.T) {
	values := url.Values{"name": {""}, "description": {"Our weekend"}, "Action": {"https://example.com"}, "SubmitButtonText": {"Wrong"}}
	r := httptest.NewRequest("POST", "/packing-lists/new", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h := handler{}
	h.NewListHandler(w, r)
	body := w.Body.String()
	for _, want := range []string{`action="/packing-lists/new"`, "Create list", "Name is required", "Our weekend"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "https://example.com") {
		t.Error("client controlled the form action")
	}
}

type fakePackingStore struct {
	packingStore
	list       packing.PackingList
	session    packing.PackingSession
	setErr     error
	remembered []string
}

func (s *fakePackingStore) RememberedCategories(context.Context, string) ([]string, error) {
	return s.remembered, nil
}
func (s *fakePackingStore) RememberCategories(_ context.Context, _ string, categories ...string) error {
	s.remembered = append(s.remembered, categories...)
	return nil
}

func (s *fakePackingStore) GetPackingList(context.Context, string, string) (packing.PackingList, error) {
	return s.list, nil
}
func (s *fakePackingStore) SetSessionItem(_ context.Context, sessionID, userID, itemID string, checked bool) (packing.PackingSession, error) {
	if sessionID != s.session.ID || userID != s.session.UserID || itemID != s.session.List.Items[0].ID {
		return packing.PackingSession{}, fmt.Errorf("wrong request identity")
	}
	s.session.List.Items[0].Checked = checked
	return s.session, s.setErr
}
func packingRequest(path string, values url.Values) *http.Request {
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "user"))
}
func TestInvalidEditAndItemFormsRemainUsable(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	h := handler{packingStore: &fakePackingStore{list: list}}
	for _, test := range []struct {
		name, path, want string
		handler          http.HandlerFunc
	}{
		{"edit", "/packing-lists/" + list.ID + "/edit", "Save", h.EditListHandler},
		{"item", "/packing-lists/" + list.ID + "/add-item", "Add item", h.AddItemHandler},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := packingRequest(test.path, url.Values{"name": {"  "}, "category": {"Shelter"}, "description": {"Weekend"}})
			route := chi.NewRouteContext()
			route.URLParams.Add("id", list.ID)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
			w := httptest.NewRecorder()
			test.handler(w, r)
			for _, want := range []string{"Name is required", test.want, `action="` + test.path + `"`} {
				if !strings.Contains(w.Body.String(), want) {
					t.Errorf("missing %q", want)
				}
			}
			if test.name == "item" && !strings.Contains(w.Body.String(), `value="Shelter"`) {
				t.Error("lost category")
			}
		})
	}
}
func TestSetItemReturnsSavedRowAndSupportsNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			list := packing.NewList("user", "Camping", "")
			list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
			session := packing.NewPackingSession(list)
			store := &fakePackingStore{session: session}
			h := handler{packingStore: store}
			r := packingRequest("/trips/set-item", url.Values{"sessionId": {session.ID}, "itemId": {list.Items[0].ID}, "checked": {"true"}})
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.SetSessionItemHandler(w, r)
			if !store.session.List.Items[0].Checked {
				t.Fatal("item not packed")
			}
			if htmx {
				body := w.Body.String()
				for _, want := range []string{`id="pack-` + list.Items[0].ID + `"`, `aria-pressed="true"`, `value="false"`, `id="packing-progress"`, `hx-swap-oob="true"`, "1 of 1 items packed", `<p id="packing-progress-status" hx-swap-oob="innerHTML">1 of 1 items packed</p>`, `<span id="pack-count-0-0" class="group-count group-done" hx-swap-oob="true">All packed</span>`, `<span id="trip-packing-count" class="tab-count" hx-swap-oob="true">1/1</span>`} {
					if !strings.Contains(body, want) {
						t.Errorf("missing %q", want)
					}
				}
				if strings.Contains(body, "<html") || strings.Contains(body, `id="packing-checklist"`) || strings.Contains(body, "<form") || w.Header().Get("HX-Refresh") != "" {
					t.Errorf("returned more than the row and progress:\n%s", body)
				}
			} else if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/trips/"+session.ID {
				t.Error("normal form missing redirect")
			}
		})
	}
}
func TestSetItemRejectsInvalidState(t *testing.T) {
	h := handler{}
	r := packingRequest("/trips/set-item", url.Values{"sessionId": {"session"}, "itemId": {"item"}, "checked": {"maybe"}})
	w := httptest.NewRecorder()
	h.SetSessionItemHandler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}
func TestSignoutRequiresPostAndCSRF(t *testing.T) {
	routes := Routes(Config{Auth: &auth.Auth{}})
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "http://localhost/auth/signout", nil)
		csrf.Protect([]byte("01234567890123456789012345678901"), csrf.Secure(false))(routes).ServeHTTP(w, r)
		want := http.StatusForbidden
		if method == "GET" {
			want = http.StatusMethodNotAllowed
		}
		if w.Code != want {
			t.Errorf("%s: got %d want %d", method, w.Code, want)
		}
	}
}

func (s *fakePackingStore) SavePackingList(_ context.Context, list packing.PackingList) error {
	s.list = list
	return nil
}
func TestEditCanClearDescriptionAndRequiresSubmittedName(t *testing.T) {
	for _, name := range []string{"New name", ""} {
		t.Run(name, func(t *testing.T) {
			list := packing.NewList("user", "Camping", "Old description")
			store := &fakePackingStore{list: list}
			h := handler{packingStore: store}
			r := packingRequest("/packing-lists/"+list.ID+"/edit", url.Values{"name": {name}, "description": {""}})
			route := chi.NewRouteContext()
			route.URLParams.Add("id", list.ID)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
			w := httptest.NewRecorder()
			h.EditListHandler(w, r)
			if name == "" {
				if !strings.Contains(w.Body.String(), "Name is required") || store.list.Name != "Camping" {
					t.Error("empty submitted name did not fail validation")
				}
			} else if w.Code != http.StatusSeeOther || store.list.Description != "" {
				t.Error("could not clear description")
			}
		})
	}
}
func TestFailedCheckoffDoesNotRenderSuccess(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	session := packing.NewPackingSession(list)
	h := handler{packingStore: &fakePackingStore{session: session, setErr: &azcore.ResponseError{StatusCode: 412}}}
	r := packingRequest("/trips/set-item", url.Values{"sessionId": {session.ID}, "itemId": {list.Items[0].ID}, "checked": {"true"}})
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.SetSessionItemHandler(w, r)
	if w.Code != http.StatusConflict || strings.Contains(w.Body.String(), `id="packing-checklist"`) {
		t.Fatal("failed save rendered a successful checklist")
	}
}
func TestDeleteCSRFUsesHeader(t *testing.T) {
	protected := csrf.Protect([]byte("01234567890123456789012345678901"), csrf.Secure(false), csrf.FieldName("_csrf"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(csrf.Token(r)))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	initial := httptest.NewRecorder()
	protected.ServeHTTP(initial, httptest.NewRequest("GET", "https://example.com/", nil))
	for _, header := range []bool{false, true} {
		r := httptest.NewRequest("DELETE", "https://example.com/packing-lists/list", strings.NewReader(url.Values{"_csrf": {initial.Body.String()}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "https://example.com")
		for _, cookie := range initial.Result().Cookies() {
			r.AddCookie(cookie)
		}
		if header {
			r.Header.Set("X-CSRF-Token", initial.Body.String())
		}
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, r)
		want := http.StatusForbidden
		if header {
			want = http.StatusNoContent
		}
		if w.Code != want {
			t.Errorf("header %v got %d want %d", header, w.Code, want)
		}
	}
}

func TestSyncRouteRejectsMissingCSRF(t *testing.T) {
	routes := Routes(Config{Auth: &auth.Auth{}})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "https://example.com/api/sessions/trip/sync", strings.NewReader(`{"id":"operation","itemId":"tent","checked":true,"expectedRevision":0}`))
	r.Header.Set("Content-Type", "application/json")
	csrf.Protect([]byte("01234567890123456789012345678901"), csrf.Secure(false))(routes).ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("sync bypassed CSRF: %d", w.Code)
	}
}

func TestHealthRouteDoesNotRequireLoginOrDatabase(t *testing.T) {
	r := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	Routes(Config{Auth: &auth.Auth{}}).ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok\n" {
		t.Fatalf("health probe failed: %d %s", w.Code, w.Body.String())
	}
}
