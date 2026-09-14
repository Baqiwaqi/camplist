package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"html"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// The preparation card used to post plain forms and redirect, which reloaded
// the page and scrolled back to the top. htmx saves now swap only the card and
// the page's list revision; a normal form post still redirects.
func TestPreparationSavesSwapTheCardAndSupportNormalForms(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}, {ID: "tent", Name: "Repair tent"}, {ID: "lamp", Name: "Charge lamp"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	current := func() packing.PackingList {
		t.Helper()
		list, err := store.GetPackingList(ctx, list.ID, "user")
		if err != nil {
			t.Fatal(err)
		}
		return list
	}
	card := func(w *httptest.ResponseRecorder, saved packing.PackingList, want ...string) string {
		t.Helper()
		body := w.Body.String()
		if w.Code != 200 || w.Header().Get("HX-Redirect") != "" || w.Header().Get("HX-Refresh") != "" {
			t.Fatalf("htmx save: %d %v %s", w.Code, w.Header(), body)
		}
		if got := listRevision(t, body); got != saved.Revision() {
			t.Errorf("list revision %q, want the saved %q", got, saved.Revision())
		}
		for _, want := range append(want, `<section id="list-preparation"`) {
			if !strings.Contains(body, want) {
				t.Errorf("card missing %q", want)
			}
		}
		for _, page := range []string{"<html", "<header", "Add an item"} {
			if strings.Contains(body, page) {
				t.Errorf("card renders page content %q", page)
			}
		}
		return body
	}
	htmx := map[string]string{"HX-Request": "true"}

	opened := current()
	w := savePreparation(h, list.ID, url.Values{"taskId": {"fuel"}, "name": {"Buy fuel"}, "scope": {"shared"}, "action": {"save"}, "done": {"true"}, "revision": {opened.Revision()}}, htmx)
	saved := current()
	if !saved.Tasks[0].Done || saved.Tasks[1].Done {
		t.Fatalf("done state not saved: %+v", saved.Tasks)
	}
	body := card(w, saved, `aria-pressed="true" aria-label="Toggle done for Buy fuel"`, "1 of 3 done", `x-data="{ editing: false }"`)
	if strings.Contains(body, "autofocus") {
		t.Error("mark done moves focus; htmx keeps it on the toggle")
	}

	// The page's previous revision is stale now.
	if w := savePreparation(h, list.ID, url.Values{"taskId": {"tent"}, "name": {"Repair tent"}, "scope": {"shared"}, "action": {"save"}, "done": {"true"}, "revision": {opened.Revision()}}, htmx); w.Code != 409 {
		t.Fatalf("stale revision: %d", w.Code)
	}

	// Add focuses the new task field again.
	w = savePreparation(h, list.ID, url.Values{"name": {"Pack pegs"}, "scope": {"shared"}, "revision": {saved.Revision()}}, htmx)
	saved = current()
	card(w, saved, "Pack pegs", `id="new-task" name="name" placeholder="Like buying fuel" required maxlength="200" autofocus`)

	// A rename made in edit mode keeps edit mode and its field.
	w = savePreparation(h, list.ID, url.Values{"taskId": {"tent"}, "name": {"Repair tent poles"}, "scope": {"shared"}, "action": {"save"}, "done": {"false"}, "editing": {"true"}, "revision": {saved.Revision()}}, htmx)
	saved = current()
	card(w, saved, `x-data="{ editing: true }"`, `id="task-tent" name="name" value="Repair tent poles" required maxlength="200" autofocus`)

	// Remove focuses the task the button named.
	w = savePreparation(h, list.ID, url.Values{"taskId": {"tent"}, "name": {"Repair tent poles"}, "action": {"remove"}, "next": {"lamp"}, "editing": {"true"}, "revision": {saved.Revision()}}, htmx)
	saved = current()
	body = card(w, saved, `x-data="{ editing: true }"`, `id="task-lamp" name="name" value="Charge lamp" required maxlength="200" autofocus`)
	if strings.Contains(body, "Repair tent poles") {
		t.Error("removed task still rendered")
	}

	// Removing the last task leaves edit mode and returns to the new task field.
	for _, task := range saved.Tasks {
		w = savePreparation(h, list.ID, url.Values{"taskId": {task.ID}, "name": {task.Name}, "action": {"remove"}, "editing": {"true"}, "revision": {current().Revision()}}, htmx)
	}
	card(w, current(), `x-data="{ editing: false }"`, "No tasks yet", `placeholder="Like buying fuel" required maxlength="200" autofocus`)

	if w := savePreparation(h, list.ID, url.Values{"name": {"Buy fuel"}, "scope": {"shared"}, "revision": {current().Revision()}}, nil); w.Code != 303 || w.Header().Get("Location") != "/packing-lists/"+list.ID {
		t.Fatalf("no-script fallback: %d %q", w.Code, w.Header().Get("Location"))
	}
}

// On a shared list, item Delete, an inline edit and the preparation card all
// send the page's one list revision. After a save they must work with the
// revision that save swapped in, while another member's later write still
// conflicts.
func TestSharedListActionsFollowTheSwappedRevision(t *testing.T) {
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
	markDoneFrom := func(revision string, done string) string {
		w := savePreparation(h, list.ID, url.Values{"taskId": {"fuel"}, "name": {"Buy fuel"}, "scope": {"shared"}, "action": {"save"}, "done": {done}, "revision": {revision}}, map[string]string{"HX-Request": "true"})
		if w.Code != 200 {
			t.Fatalf("mark done: %d %s", w.Code, w.Body.String())
		}
		swapped := listRevision(t, w.Body.String())
		if swapped == "" || swapped == revision {
			t.Fatalf("mark done swapped revision %q after %q", swapped, revision)
		}
		return swapped
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

	revision := markDoneFrom(page().Revision(), "true")
	w := deleteItem(tent, revision)
	if w.Code != 200 || w.Header().Get("HX-Refresh") != "" {
		t.Fatalf("delete after mark done: %d %s", w.Code, w.Body.String())
	}
	revision = listRevision(t, w.Body.String())

	w = editItem(stove, "Gas stove", revision)
	if w.Code != 200 || strings.Contains(w.Body.String(), "This list changed") || !strings.Contains(w.Body.String(), "Gas stove") {
		t.Fatalf("inline edit after delete: %d %s", w.Code, w.Body.String())
	}
	revision = listRevision(t, w.Body.String())
	markDoneFrom(revision, "false")
	if saved := page(); len(saved.Items) != 1 || saved.Items[0].Name != "Gas stove" || saved.Tasks[0].Done {
		t.Fatalf("saved list: %+v %+v", saved.Items, saved.Tasks)
	}

	// Another member writes right after the toggle is saved: the response
	// still carries the toggle's own revision, so the page stays stale.
	racing.after = memberWrite
	advanced := markDoneFrom(page().Revision(), "true")
	racing.after = nil
	if advanced == page().Revision() {
		t.Fatal("swapped revision skipped another member's write")
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

func savePreparation(h handler, listID string, values url.Values, headers map[string]string) *httptest.ResponseRecorder {
	r := withItemRoute(packingRequest("/packing-lists/"+listID+"/preparation/edit", values), listID, "")
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	h.EditPreparationTask(w, r)
	return w
}

var listRevisionInput = regexp.MustCompile(`<input type="hidden" id="list-revision" value="([^"]*)" hx-swap-oob="true">`)

// listRevision is the revision a response swaps into the page's #list-revision.
func listRevision(t *testing.T, body string) string {
	t.Helper()
	match := listRevisionInput.FindAllStringSubmatch(body, -1)
	if len(match) != 1 {
		t.Fatalf("want one out-of-band list revision, found %d in %s", len(match), body)
	}
	return html.UnescapeString(match[0][1])
}
