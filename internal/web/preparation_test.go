package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"encoding/json"
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
	toggle := func(taskID, name, revision string, headers map[string]string) *httptest.ResponseRecorder {
		return togglePreparation(h, list.ID, taskID, name, revision, headers)
	}

	w := toggle("fuel", "Buy fuel", list.Revision(), markDone)
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
	if from, to := listRevisionTrigger(t, w); from != list.Revision() || to != saved.Revision() {
		t.Fatalf("list-revision trigger from %q to %q, want %q to %q", from, to, list.Revision(), saved.Revision())
	}
	body := w.Body.String()
	for _, want := range []string{
		`<ul id="preparation-tasks">`,
		`aria-pressed="true" aria-label="Toggle done for Buy fuel"`,
		`name="revision" value="` + saved.Revision() + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment missing %q", want)
		}
	}
	if strings.Contains(body, `value="`+list.Revision()+`"`) {
		t.Error("fragment still carries the stale revision")
	}
	for _, page := range []string{"<html", "<header", "Add an item", "Prepare before your next trip", "hx-swap-oob"} {
		if strings.Contains(body, page) {
			t.Errorf("fragment renders page content %q", page)
		}
	}

	// The swapped rows carry the new revision, so the next toggle succeeds.
	if w := toggle("tent", "Repair tent", saved.Revision(), markDone); w.Code != 200 {
		t.Fatalf("second toggle with fresh revision: %d %s", w.Code, w.Body.String())
	}
	if w := toggle("fuel", "Buy fuel", saved.Revision(), markDone); w.Code != 409 || w.Header().Get("HX-Trigger") != "" {
		t.Fatalf("stale revision: %d trigger=%q", w.Code, w.Header().Get("HX-Trigger"))
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

// On a shared list, item Delete and an open inline edit carry the page's list
// revision. After Mark done they must work with the revision the toggle
// produced, while another member's later write still conflicts.
func TestSharedListItemActionsAfterMarkDone(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen")}
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-list", list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "member", Name: "Member"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "user", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	memberWrite := func() {
		shared, err := store.GetPackingList(ctx, list.ID, "member")
		if err != nil {
			t.Fatal(err)
		}
		shared.Description += "!"
		if err := store.SavePackingList(ctx, shared); err != nil {
			t.Fatal(err)
		}
	}
	racing := &writeAfterPreparation{Store: store}
	h := handler{packingStore: racing}
	page := func() packing.PackingList {
		page, err := store.GetPackingList(ctx, list.ID, "user")
		if err != nil {
			t.Fatal(err)
		}
		if !page.IsShared() {
			t.Fatal("list is not shared")
		}
		return page
	}
	markDoneFrom := func(page packing.PackingList, done bool) string {
		values := map[bool]string{true: "true", false: "false"}
		w := togglePreparationDone(h, list.ID, "fuel", "Buy fuel", values[done], page.Revision(), markDone)
		if w.Code != 200 {
			t.Fatalf("mark done: %d %s", w.Code, w.Body.String())
		}
		from, to := listRevisionTrigger(t, w)
		if from != page.Revision() || to == "" || to == from {
			t.Fatalf("list-revision trigger from %q to %q, page %q", from, to, page.Revision())
		}
		return to
	}
	deleteItem := func(itemID, revision string) *httptest.ResponseRecorder {
		r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/remove-item/"+itemID, nil), list.ID, itemID)
		r.Method = "DELETE"
		r.Header.Set("HX-Request", "true")
		r.Header.Set("X-Camplist-Revision", revision)
		w := httptest.NewRecorder()
		h.RemoveItemHandler(w, r)
		return w
	}
	editItem := func(itemID, name, revision string) *httptest.ResponseRecorder {
		values := url.Values{"name": {name}, "category": {"Kitchen"}, "scope": {"shared"}, "revision": {revision}}
		r := withItemRoute(packingRequest("/packing-lists/"+list.ID+"/edit-item/"+itemID, values), list.ID, itemID)
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		h.EditItemHandler(w, r)
		return w
	}
	tent, stove := list.Items[0].ID, list.Items[1].ID

	opened := page()
	if w := deleteItem(tent, markDoneFrom(opened, true)); w.Code != 200 || w.Header().Get("HX-Refresh") != "true" {
		t.Fatalf("delete after mark done: %d %s", w.Code, w.Body.String())
	}

	opened = page()
	w := editItem(stove, "Gas stove", markDoneFrom(opened, false))
	if w.Code != 200 || strings.Contains(w.Body.String(), "This list changed") || !strings.Contains(w.Body.String(), "Gas stove") {
		t.Fatalf("inline edit after mark done: %d %s", w.Code, w.Body.String())
	}
	if saved := page(); len(saved.Items) != 1 || saved.Items[0].Name != "Gas stove" || saved.Tasks[0].Done {
		t.Fatalf("saved list: %+v %+v", saved.Items, saved.Tasks)
	}

	// Another member writes right after the toggle is saved: the trigger still
	// names the toggle's own revision, so the page's controls stay stale.
	racing.after = memberWrite
	opened = page()
	advanced := markDoneFrom(opened, true)
	racing.after = nil
	if current := page(); advanced == current.Revision() {
		t.Fatal("trigger advanced controls past another member's write")
	}
	if w := deleteItem(stove, advanced); w.Code != 409 {
		t.Fatalf("delete over another member's write: %d", w.Code)
	}
	if w := editItem(stove, "Camp stove", advanced); !strings.Contains(w.Body.String(), "This list changed") {
		t.Fatalf("inline edit over another member's write: %d %s", w.Code, w.Body.String())
	}
	if saved := page(); len(saved.Items) != 1 || saved.Items[0].Name != "Gas stove" {
		t.Fatalf("stale actions changed the list: %+v", saved.Items)
	}
}

var markDone = map[string]string{"HX-Request": "true", "HX-Target": "preparation-tasks"}

type writeAfterPreparation struct {
	*packing.Store
	after func()
}

func (s *writeAfterPreparation) EditPreparationTask(ctx context.Context, id, actor string, task packing.PreparationTask, remove bool, revision string) (packing.PackingList, error) {
	list, err := s.Store.EditPreparationTask(ctx, id, actor, task, remove, revision)
	if err == nil && s.after != nil {
		s.after()
	}
	return list, err
}

func togglePreparation(h handler, listID, taskID, name, revision string, headers map[string]string) *httptest.ResponseRecorder {
	return togglePreparationDone(h, listID, taskID, name, "true", revision, headers)
}

func togglePreparationDone(h handler, listID, taskID, name, done, revision string, headers map[string]string) *httptest.ResponseRecorder {
	values := url.Values{"taskId": {taskID}, "name": {name}, "scope": {"shared"}, "action": {"save"}, "done": {done}, "revision": {revision}}
	r := withItemRoute(packingRequest("/packing-lists/"+listID+"/preparation/edit", values), listID, "")
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	h.EditPreparationTask(w, r)
	return w
}

func listRevisionTrigger(t *testing.T, w *httptest.ResponseRecorder) (from, to string) {
	t.Helper()
	var trigger struct {
		ListRevision struct{ From, To string } `json:"list-revision"`
	}
	if err := json.Unmarshal([]byte(w.Header().Get("HX-Trigger")), &trigger); err != nil {
		t.Fatalf("HX-Trigger %q: %v", w.Header().Get("HX-Trigger"), err)
	}
	return trigger.ListRevision.From, trigger.ListRevision.To
}
