package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"

	"github.com/go-chi/chi/v5"
)

type deleteListStore struct {
	packingStore
	lists []packing.PackingList
}

func (s *deleteListStore) DeletePackingList(_ context.Context, id, userID string) error {
	for i, list := range s.lists {
		if list.ID == id && list.UserID == userID {
			s.lists = append(s.lists[:i], s.lists[i+1:]...)
			return nil
		}
	}
	return packing.ErrNotFound
}

func (s *deleteListStore) GetPackingLists(context.Context, string) ([]packing.PackingList, error) {
	return s.lists, nil
}

func TestDeleteListRemovesCardInPlaceOrRedirects(t *testing.T) {
	for _, test := range []struct {
		name         string
		htmx         bool
		target       bool
		remaining    int
		wantRedirect string
		wantEmpty    bool
	}{
		{name: "lists page keeps other cards", htmx: true, target: true, remaining: 1},
		{name: "lists page last card shows empty state", htmx: true, target: true, wantEmpty: true},
		{name: "details page redirects", htmx: true, remaining: 1, wantRedirect: "/"},
		{name: "plain request redirects", remaining: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := packing.NewList("user", "Camping", "")
			store := &deleteListStore{lists: []packing.PackingList{list}}
			for range test.remaining {
				store.lists = append(store.lists, packing.NewList("user", "Other", ""))
			}
			r := httptest.NewRequest("DELETE", "/packing-lists/"+list.ID, nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("id", list.ID)
			ctx := context.WithValue(r.Context(), auth.USER_ID_KEY, "user")
			r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, route))
			if test.htmx {
				r.Header.Set("HX-Request", "true")
			}
			if test.target {
				r.Header.Set("HX-Target", "list-"+list.ID)
			}
			w := httptest.NewRecorder()
			(&handler{packingStore: store}).DeleteListHandler(w, r)

			if len(store.lists) != test.remaining {
				t.Fatalf("store holds %d lists, want %d", len(store.lists), test.remaining)
			}
			body := w.Body.String()
			switch {
			case !test.htmx:
				if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
					t.Errorf("plain request: status %d location %q", w.Code, w.Header().Get("Location"))
				}
			case test.wantRedirect != "":
				if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != test.wantRedirect || body != "" {
					t.Errorf("details page: status %d redirect %q body %q", w.Code, w.Header().Get("HX-Redirect"), body)
				}
			default:
				if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "" || strings.Contains(body, "<html") {
					t.Errorf("in-place delete navigated: status %d redirect %q body %.80q", w.Code, w.Header().Get("HX-Redirect"), body)
				}
				hasEmpty := strings.Contains(body, `id="packing-lists-empty"`) && strings.Contains(body, `hx-swap-oob="true"`) && strings.Contains(body, "No packing lists yet")
				if hasEmpty != test.wantEmpty {
					t.Errorf("empty state swapped = %v, want %v; body %q", hasEmpty, test.wantEmpty, body)
				}
			}
		})
	}
}
