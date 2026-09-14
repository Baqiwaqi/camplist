package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestReviewSubmissionAndSelectionThroughHTTP(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Weekend", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	h := handler{packingStore: store}
	r := packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}})
	route := chi.NewRouteContext()
	route.URLParams.Add("id", session.ID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("submit review: %d %s", w.Code, w.Body.String())
	}
	read := httptest.NewRecorder()
	h.ReviewPage(read, r)
	if !strings.Contains(read.Body.String(), "Matches") || !strings.Contains(read.Body.String(), "Apply selected changes") {
		t.Fatal("review cannot be selected")
	}
	current, _ := store.GetPackingList(ctx, list.ID, "user")
	r = packingRequest("/trips/"+session.ID+"/review/apply", url.Values{"selected": {"matches"}, "revision": {current.Revision()}}).WithContext(r.Context())
	w = httptest.NewRecorder()
	h.ApplyReviewHandler(w, r)
	if w.Code != 303 {
		t.Fatalf("apply: %d %s", w.Code, w.Body.String())
	}
	got, _ := store.GetPackingList(ctx, list.ID, "user")
	if len(got.Items) != 1 || got.Items[0].Name != "Matches" {
		t.Fatal("selected addition not persisted")
	}
}

func TestNewSessionPageSurfacesPreparationAndRecentImprovements(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}, {ID: "done", Name: "Already repaired", Done: true}}
	list.Changes = []string{"Added spare matches"}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	route := chi.NewRouteContext()
	route.URLParams.Add("id", session.ID)
	r := packingRequest("/trips/"+session.ID, nil)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h := handler{packingStore: store}
	h.SessionDetailsPage(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Buy fuel") || !strings.Contains(w.Body.String(), "Added spare matches") || strings.Contains(w.Body.String(), "<li>Already repaired</li>") {
		t.Fatalf("preparation absent or completed work resurfaced: %s", w.Body.String())
	}
}

func TestBeforeTripTasksHaveCompletionControls(t *testing.T) {
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Weekend", "")
	list.Tasks = []packing.PreparationTask{{ID: "car", Name: "Auto opladen"}, {ID: "sleep", Name: "Slaapspullen ophalen nederhorst"}}
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePackingSession(context.Background(), list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	r := withItemRoute(packingRequest("/trips/"+session.ID, nil), session.ID, "")
	w := httptest.NewRecorder()
	h := handler{packingStore: store}
	h.SessionDetailsPage(w, r)
	if !strings.Contains(w.Body.String(), `/trips/`+session.ID+`/preparation`) || !strings.Contains(w.Body.String(), `Mark done`) {
		t.Fatal("before-trip tasks are read-only: no completion controls")
	}
	submit := func(taskID, done, revision string) *httptest.ResponseRecorder {
		r := withItemRoute(packingRequest("/trips/"+session.ID+"/preparation", url.Values{"taskId": {taskID}, "done": {done}, "expectedRevision": {revision}}), session.ID, "")
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		h.SetSessionPreparationTask(w, r)
		return w
	}
	for _, id := range []string{"car", "sleep"} {
		response := submit(id, "true", "0")
		if response.Code != 200 || !strings.Contains(response.Body.String(), "Undo") {
			t.Fatalf("complete %s: %d %s", id, response.Code, response.Body.String())
		}
	}
	reopened, err := store.GetPackingSession(context.Background(), session.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.List.Tasks[0].Done || !reopened.List.Tasks[1].Done {
		t.Fatal("completion was not persisted")
	}
	original, err := store.GetPackingList(context.Background(), list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if original.Tasks[0].Done || original.Tasks[1].Done {
		t.Fatal("trip completion changed reusable tasks")
	}
	if response := submit("car", "false", "0"); response.Code != 409 {
		t.Fatalf("stale undo: %d", response.Code)
	}
	if response := submit("car", "false", "1"); response.Code != 200 {
		t.Fatalf("undo: %d", response.Code)
	}
	if response := submit("missing", "true", "0"); response.Code != 404 {
		t.Fatalf("unknown task: %d", response.Code)
	}
	if response := submit("car", "true", ""); response.Code != 400 {
		t.Fatalf("missing revision: %d", response.Code)
	}

}

func TestAddReviewReturnsFragmentsForHTMXAndRedirectsNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store := packing.NewStore(testsupport.NewDocuments())
			ctx := context.Background()
			list := packing.NewList("user", "Weekend", "")
			if err := store.SavePackingList(ctx, list); err != nil {
				t.Fatal(err)
			}
			session, err := store.CreatePackingSession(ctx, list.ID, "user")
			if err != nil {
				t.Fatal(err)
			}
			h := handler{packingStore: store}
			r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}}), session.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.AddReviewHandler(w, r)
			saved, _ := store.GetPackingSession(ctx, session.ID, "user")
			if len(saved.Review) != 1 || saved.Review[0].Name != "Matches" {
				t.Fatal("observation not persisted")
			}
			body := w.Body.String()
			if !htmx {
				if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/trips/"+session.ID+"/review" {
					t.Fatalf("normal form missing redirect: %d %q", w.Code, w.Header().Get("Location"))
				}
				return
			}
			if w.Code != http.StatusOK || strings.Contains(body, "<html") || w.Header().Get("Location") != "" {
				t.Fatalf("returned navigation instead of fragments: %d %s", w.Code, body)
			}
			for _, want := range []string{`id="review-form"`, `hx-sync="this:drop"`, `id="review-submit"`, `id="review-save-status" hx-swap-oob="innerHTML"`, "Saved “Matches”.", `id="trip-observations"`, `hx-swap-oob="outerHTML"`, "<h3 class=\"mt-0\">Matches</h3>", "Apply selected changes"} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(body, `name="entryId" value="matches"`) || strings.Contains(body, `value="Matches"`) {
				t.Error("form was not reset with a fresh entry")
			}
		})
	}
}

func TestAddReviewErrorKeepsSubmittedValues(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store := packing.NewStore(testsupport.NewDocuments())
			ctx := context.Background()
			list := packing.NewList("user", "Weekend", "")
			if err := store.SavePackingList(ctx, list); err != nil {
				t.Fatal(err)
			}
			session, err := store.CreatePackingSession(ctx, list.ID, "user")
			if err != nil {
				t.Fatal(err)
			}
			h := handler{packingStore: store}
			// No observation ticked, so the store rejects the entry.
			r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"stove"}, "name": {"Stove"}, "action": {"none"}}), session.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.AddReviewHandler(w, r)
			body := w.Body.String()
			if !strings.Contains(body, "Please check the submitted values.") || !strings.Contains(body, `value="Stove"`) || !strings.Contains(body, `name="entryId" value="stove"`) {
				t.Fatalf("error lost the message or submitted values: %s", body)
			}
			if htmx {
				if w.Code != http.StatusOK || strings.Contains(body, "<html") || strings.Contains(body, `id="trip-observations"`) || !strings.Contains(body, `id="review-save-status" hx-swap-oob="innerHTML"></div>`) {
					t.Fatalf("htmx error should swap only the form: %d %s", w.Code, body)
				}
			} else if w.Code != http.StatusBadRequest || !strings.Contains(body, "<html") {
				t.Fatalf("normal form error should re-render the page: %d", w.Code)
			}
		})
	}
}

func reviewFixture(t *testing.T) (*packing.Store, packing.PackingList, packing.PackingSession, handler) {
	t.Helper()
	store := packing.NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := packing.NewList("user", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	session, err := store.CreatePackingSession(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	list, err = store.GetPackingList(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	return store, list, session, handler{packingStore: store}
}

func TestSaveAndApplyNowAppliesTheProposedChange(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store, list, session, h := reviewFixture(t)
			r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}, "apply": {"true"}, "revision": {list.Revision()}}), session.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.AddReviewHandler(w, r)
			got, _ := store.GetPackingList(context.Background(), list.ID, "user")
			if len(got.Items) != 2 || got.Items[1].Name != "Matches" || !slices.Contains(got.AppliedReviews, session.ID+":matches") {
				t.Fatalf("change not applied: %+v", got.Items)
			}
			body := w.Body.String()
			if !htmx {
				if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/trips/"+session.ID+"/review" {
					t.Fatalf("normal form missing redirect: %d", w.Code)
				}
				return
			}
			for _, want := range []string{"Saved and applied “Matches”.", `id="current-review-list" class="card mt-8" hx-swap-oob="outerHTML"`, "<li>Matches — </li>", ">Applied</span>", `name="revision" value="` + got.Revision() + `"`} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}

func TestSaveOnlyLeavesTheListAndSaysSo(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}, "revision": {list.Revision()}}), session.ID, "")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	got, _ := store.GetPackingList(context.Background(), list.ID, "user")
	if len(got.Items) != 1 || len(got.AppliedReviews) != 0 {
		t.Fatal("save only changed the reusable list")
	}
	body := w.Body.String()
	if !strings.Contains(body, "Not applied yet") || !strings.Contains(body, ">Waiting to be applied</span>") {
		t.Fatalf("save only response unclear: %s", body)
	}
}

func TestSaveOnlyRefreshesTheListCardWithTheRevisionTheFormsCarry(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	shown := list.Revision()
	list.Items = append(list.Items, packing.NewItem("Stove", "Kitchen"))
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	current, _ := store.GetPackingList(context.Background(), list.ID, "user")
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}, "revision": {shown}}), session.ID, "")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	body := w.Body.String()
	for _, want := range []string{`id="current-review-list" class="card mt-8" hx-swap-oob="outerHTML"`, "<li>Stove — Kitchen</li>", `name="revision" value="` + current.Revision() + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, `value="`+shown+`"`) {
		t.Error("a form still carries the revision the old list card showed")
	}
}

func TestReviewErrorKeepsTheRevisionThePageWasShowing(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	shown := list.Revision()
	list.Description = "Changed elsewhere"
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"stove"}, "name": {"Stove"}, "action": {"add"}, "revision": {shown}}), session.ID, "")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	body := w.Body.String()
	if !strings.Contains(body, "Please check the submitted values.") || !strings.Contains(body, `name="revision" value="`+shown+`"`) || strings.Contains(body, `id="current-review-list"`) {
		t.Fatalf("error form should keep the displayed revision and leave the list card: %s", body)
	}
}

func TestReviewPageShowsNoApplyStatusWhenTheListIsUnavailable(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	ctx := context.Background()
	for _, entry := range []packing.ReviewEntry{
		{ID: "pending", Name: "Stove", Forgotten: true, Action: packing.ReviewAdd},
		{ID: "applied", Name: "Matches", Forgotten: true, Action: packing.ReviewAdd},
	} {
		if _, err := store.AddReviewEntry(ctx, session.ID, "user", entry); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ApplyReview(ctx, session.ID, "user", list.Revision(), []string{"applied"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePackingList(ctx, list.ID, "user"); err != nil {
		t.Fatal(err)
	}
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", nil), session.ID, "")
	w := httptest.NewRecorder()
	h.ReviewPage(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "The reusable list is unavailable") {
		t.Fatalf("list should be unavailable: %d %s", w.Code, body)
	}
	for _, want := range []string{`<h3 class="mt-0">Stove</h3></div>`, `<h3 class="mt-0">Matches</h3></div>`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, label := range []string{"Applied</span>", "Waiting to be applied", "Not applied"} {
		if strings.Contains(body, label) {
			t.Errorf("unavailable list still labelled %q", label)
		}
	}
}

func TestSaveWhileTheListIsUnavailableSaysTheChangeCannotBeApplied(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	if err := store.DeletePackingList(context.Background(), list.ID, "user"); err != nil {
		t.Fatal(err)
	}
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"compass"}, "name": {"Compass"}, "forgotten": {"true"}, "action": {"add"}}), session.ID, "")
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.AddReviewHandler(w, r)
	body := w.Body.String()
	want := "Saved “Compass”. The list this trip came from is unavailable, so this change can&#39;t be applied right now. Recover the list below to apply it."
	if w.Code != http.StatusOK || !strings.Contains(body, want) {
		t.Fatalf("missing unavailable confirmation %q: %d %s", want, w.Code, body)
	}
	if strings.Contains(body, "Not applied yet") {
		t.Error("unavailable list still tells the user to select and apply the change")
	}
}

func TestSaveAndApplyNowKeepsConflictHandling(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store, list, session, h := reviewFixture(t)
			stale := list.Revision()
			list.Description = "Changed elsewhere"
			if err := store.SavePackingList(context.Background(), list); err != nil {
				t.Fatal(err)
			}
			r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}, "apply": {"true"}, "revision": {stale}}), session.ID, "")
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.AddReviewHandler(w, r)
			got, _ := store.GetPackingList(context.Background(), list.ID, "user")
			saved, _ := store.GetPackingSession(context.Background(), session.ID, "user")
			if len(got.Items) != 1 || len(saved.Review) != 1 {
				t.Fatalf("stale apply overwrote the list or lost the observation: %d items, %d reviews", len(got.Items), len(saved.Review))
			}
			body := w.Body.String()
			if !strings.Contains(body, "but its change was not applied. The saved version changed.") {
				t.Fatalf("conflict message missing: %s", body)
			}
			if htmx && (w.Code != http.StatusOK || !strings.Contains(body, ">Waiting to be applied</span>")) {
				t.Fatalf("htmx conflict: %d", w.Code)
			}
			if !htmx && w.Code != http.StatusConflict {
				t.Fatalf("plain conflict status %d", w.Code)
			}
		})
	}
}

func TestReviewPageLabelsProposalsAndUsesListboxSelects(t *testing.T) {
	store, list, session, h := reviewFixture(t)
	ctx := context.Background()
	for _, entry := range []packing.ReviewEntry{
		{ID: "note", Name: "Chair", Unused: true, Action: packing.ReviewObserve},
		{ID: "pending", Name: "Stove", Forgotten: true, Action: packing.ReviewAdd},
		{ID: "applied", Name: "Matches", Forgotten: true, Action: packing.ReviewAdd},
	} {
		if _, err := store.AddReviewEntry(ctx, session.ID, "user", entry); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ApplyReview(ctx, session.ID, "user", list.Revision(), []string{"applied"}); err != nil {
		t.Fatal(err)
	}
	r := withItemRoute(packingRequest("/trips/"+session.ID+"/review", nil), session.ID, "")
	w := httptest.NewRecorder()
	h.ReviewPage(w, r)
	body := w.Body.String()
	for _, want := range []string{
		`<h3 class="mt-0">Chair</h3></div>`,
		`<h3 class="mt-0">Stove</h3><span class="tag tag-quiet">Waiting to be applied</span>`,
		`<h3 class="mt-0">Matches</h3><span class="tag">Applied</span>`,
		`role="combobox"`, `role="listbox"`, `role="option"`,
		`<select id="review-item" name="itemId"`, `<select id="review-action" name="action"`,
		`id="review-save-apply"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(body, `value="true" hidden`) {
		t.Error("Save and apply now should start hidden for Keep observation only")
	}
}
