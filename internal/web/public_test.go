package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func (s *fakePackingStore) GetPackingLists(context.Context, string) ([]packing.PackingList, error) {
	return []packing.PackingList{s.list}, nil
}

func TestHomePageShowsLandingToVisitors(t *testing.T) {
	h := handler{packingStore: &fakePackingStore{}}
	w := httptest.NewRecorder()
	h.HomePage(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	for _, want := range []string{`href="/demo"`, "Try a demo list", `href="/auth/login"`, `id="how"`, `class="pack-row`} {
		if !strings.Contains(body, want) {
			t.Errorf("landing page missing %q", want)
		}
	}
	for _, reject := range []string{"Sign out", "hx-post", "_csrf"} {
		if strings.Contains(body, reject) {
			t.Errorf("landing page contains %q", reject)
		}
	}
}

func TestHomePageShowsListsToMembers(t *testing.T) {
	list := packing.NewList("member", "Car camping", "")
	h := handler{packingStore: &fakePackingStore{list: list}}
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), auth.USER_ID_KEY, "member"))
	w := httptest.NewRecorder()
	h.HomePage(w, r)
	body := w.Body.String()
	if !strings.Contains(body, "Your lists") || !strings.Contains(body, "Car camping") {
		t.Error("member did not get the lists page")
	}
	if strings.Contains(body, "Try a demo list") {
		t.Error("member got the landing page")
	}
}

func TestDemoPageNeedsNoAccount(t *testing.T) {
	h := handler{}
	w := httptest.NewRecorder()
	h.DemoPage(w, httptest.NewRequest("GET", "/demo", nil))
	body := w.Body.String()
	for _, want := range []string{"Nothing here is saved", `class="pack-row`, "Reset demo", `href="/auth/login"`} {
		if !strings.Contains(body, want) {
			t.Errorf("demo page missing %q", want)
		}
	}
	for _, reject := range []string{"hx-post", "hx-get", "_csrf", "packing-snapshot"} {
		if strings.Contains(body, reject) {
			t.Errorf("demo page talks to the server through %q", reject)
		}
	}
}
