package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"

	"github.com/go-chi/chi/v5"
)

func (s *fakePackingStore) UpdateItem(_ context.Context, _ string, _ string, item packing.PackingItem) (packing.PackingList, error) {
	for i := range s.list.Items {
		if s.list.Items[i].ID == item.ID {
			s.list.Items[i].Name = item.Name
			s.list.Items[i].Category = item.Category
		}
	}
	return s.list, nil
}

func withItemRoute(r *http.Request, listID, itemID string) *http.Request {
	route := chi.NewRouteContext()
	route.URLParams.Add("id", listID)
	route.URLParams.Add("itemId", itemID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
}

func TestEditItemInlineRoundTrip(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	store := &fakePackingStore{list: list}
	h := handler{packingStore: store}
	item := list.Items[0]

	r := httptest.NewRequest("GET", "/packing-lists/"+list.ID+"/items/"+item.ID+"/edit", nil)
	r.Header.Set("HX-Request", "true")
	r = withItemRoute(r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "user")), list.ID, item.ID)
	w := httptest.NewRecorder()
	h.EditItemPage(w, r)
	body := w.Body.String()
	for _, want := range []string{`value="Tent"`, `value="Shelter"`, `hx-post="/packing-lists/` + list.ID + `/edit-item/` + item.ID + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("edit fragment missing %q", want)
		}
	}
	if strings.Contains(body, "<html") {
		t.Error("htmx edit request returned a full page")
	}

	r = withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {" "}, "category": {"Sleep"}}), list.ID, item.ID)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.EditItemHandler(w, r)
	if !strings.Contains(w.Body.String(), "Name is required") || !strings.Contains(w.Body.String(), `value="Sleep"`) {
		t.Error("invalid edit did not re-render the form with the error")
	}
	if store.list.Items[0].Name != "Tent" {
		t.Error("invalid edit changed the item")
	}

	r = withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {"Big tent"}, "category": {"Shelter"}}), list.ID, item.ID)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.EditItemHandler(w, r)
	if store.list.Items[0].Name != "Big tent" {
		t.Fatal("item not updated")
	}
	if !strings.Contains(w.Body.String(), "Big tent") || strings.Contains(w.Body.String(), "<form") {
		t.Error("save did not return the read-only row")
	}
	if !strings.Contains(w.Body.String(), `id="edit-`+item.ID+`"`) || !strings.Contains(w.Body.String(), "autofocus") {
		t.Error("saved row does not hand focus back to its Edit link")
	}

	r = withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {"Tent"}, "category": {""}}), list.ID, item.ID)
	w = httptest.NewRecorder()
	h.EditItemHandler(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Errorf("plain form got %d %q", w.Code, w.Header().Get("Location"))
	}

	r = httptest.NewRequest("GET", "/packing-lists/"+list.ID+"/items/"+item.ID+"/edit", nil)
	r = withItemRoute(r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "user")), list.ID, item.ID)
	w = httptest.NewRecorder()
	h.EditItemPage(w, r)
	if !strings.Contains(w.Body.String(), "<html") || !strings.Contains(w.Body.String(), "Edit item") {
		t.Error("plain edit request did not return the fallback page")
	}
}

func TestInvalidEditItemKeepsAddressBarOnFormPage(t *testing.T) {
	for _, mode := range []string{"boosted", "htmx", "plain"} {
		t.Run(mode, func(t *testing.T) {
			list := packing.NewList("user", "Camping", "")
			list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
			h := handler{packingStore: &fakePackingStore{list: list}}
			item := list.Items[0]
			r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {" "}, "category": {"Shelter"}}), list.ID, item.ID)
			switch mode {
			case "boosted":
				r.Header.Set("HX-Request", "true")
				r.Header.Set("HX-Boosted", "true")
			case "htmx":
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.EditItemHandler(w, r)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Name is required") {
				t.Fatalf("invalid edit did not re-render the form: %d", w.Code)
			}
			// The POST URL has no GET route; pushing it would make reload and Back fail.
			want := ""
			if mode == "boosted" {
				want = "false"
			}
			if got := w.Header().Get("HX-Push-Url"); got != want {
				t.Errorf("HX-Push-Url = %q, want %q", got, want)
			}
		})
	}
}

func TestValidBoostedEditItemRedirectsSoHtmxPushesTheListURL(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	h := handler{packingStore: &fakePackingStore{list: list}}
	item := list.Items[0]
	r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+item.ID, url.Values{"name": {"Big tent"}, "category": {"Shelter"}}), list.ID, item.ID)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Boosted", "true")
	w := httptest.NewRecorder()
	h.EditItemHandler(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Fatalf("boosted save did not redirect to the list: %d %q", w.Code, w.Header().Get("Location"))
	}
	if w.Header().Get("HX-Push-Url") != "" {
		t.Error("successful save suppressed the history entry for the list page")
	}
}
