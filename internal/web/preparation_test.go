package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Mark done on the reusable list used to post a plain form and redirect, which
// reloaded the page and scrolled back to the top.
func TestMarkPreparationDoneSwapsOnlyTheTaskList(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}, {ID: "tent", Name: "Repair tent"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	list, err := store.GetPackingList(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	path := "/packing-lists/" + list.ID + "/preparation/edit"
	toggle := func(taskID, name, revision string, headers map[string]string) *httptest.ResponseRecorder {
		values := url.Values{"taskId": {taskID}, "name": {name}, "scope": {"shared"}, "action": {"save"}, "done": {"true"}, "revision": {revision}}
		r := withItemRoute(packingRequest(path, values), list.ID, "")
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		w := httptest.NewRecorder()
		h.EditPreparationTask(w, r)
		return w
	}
	swap := map[string]string{"HX-Request": "true", "HX-Target": "preparation-tasks"}

	w := toggle("fuel", "Buy fuel", list.Revision(), swap)
	if w.Code != 200 || w.Header().Get("HX-Redirect") != "" {
		t.Fatalf("mark done: %d redirect=%q %s", w.Code, w.Header().Get("HX-Redirect"), w.Body.String())
	}
	saved, err := store.GetPackingList(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Tasks[0].Done || saved.Tasks[1].Done {
		t.Fatalf("done state not saved: %+v", saved.Tasks)
	}
	body := w.Body.String()
	for _, want := range []string{
		`<ul id="preparation-tasks">`,
		`aria-pressed="true" aria-label="Toggle done for Buy fuel"`,
		`name="revision" value="` + saved.Revision() + `" id="preparation-revision" hx-swap-oob="true"`,
		`name="revision" value="` + saved.Revision() + `" id="item-revision-new" hx-swap-oob="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment missing %q", want)
		}
	}
	if strings.Contains(body, `value="`+list.Revision()+`"`) {
		t.Error("fragment still carries the stale revision")
	}
	for _, page := range []string{"<html", "<header", "Add an item", "Prepare before your next trip"} {
		if strings.Contains(body, page) {
			t.Errorf("fragment renders page content %q", page)
		}
	}

	// The swapped rows carry the new revision, so the next toggle succeeds.
	if w := toggle("tent", "Repair tent", saved.Revision(), swap); w.Code != 200 {
		t.Fatalf("second toggle with fresh revision: %d %s", w.Code, w.Body.String())
	}
	if w := toggle("fuel", "Buy fuel", saved.Revision(), swap); w.Code != 409 {
		t.Fatalf("stale revision: %d", w.Code)
	}

	current, _ := store.GetPackingList(ctx, list.ID, "user")
	if w := toggle("fuel", "Buy fuel", current.Revision(), nil); w.Code != 303 || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Fatalf("no-script fallback: %d %q", w.Code, w.Header().Get("Location"))
	}
	current, _ = store.GetPackingList(ctx, list.ID, "user")
	if w := toggle("fuel", "Buy fuel", current.Revision(), map[string]string{"HX-Request": "true"}); w.Code != 200 || w.Header().Get("HX-Redirect") != "/packing-lists/"+list.ID {
		t.Fatalf("other htmx edits keep reloading: %d %q", w.Code, w.Header().Get("HX-Redirect"))
	}
}
