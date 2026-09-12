package web

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"net/url"
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
	r := packingRequest("/packing-session/"+session.ID+"/review", url.Values{"entryId": {"matches"}, "name": {"Matches"}, "forgotten": {"true"}, "action": {"add"}})
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
	r = packingRequest("/packing-session/"+session.ID+"/review/apply", url.Values{"selected": {"matches"}, "revision": {current.Revision()}}).WithContext(r.Context())
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
	r := packingRequest("/packing-session/"+session.ID, nil)
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
	r := withItemRoute(packingRequest("/packing-session/"+session.ID, nil), session.ID, "")
	w := httptest.NewRecorder()
	h := handler{packingStore: store}
	h.SessionDetailsPage(w, r)
	if !strings.Contains(w.Body.String(), `/packing-session/`+session.ID+`/preparation`) || !strings.Contains(w.Body.String(), `Mark done`) {
		t.Fatal("before-trip tasks are read-only: no completion controls")
	}
	submit := func(taskID, done, revision string) *httptest.ResponseRecorder {
		r := withItemRoute(packingRequest("/packing-session/"+session.ID+"/preparation", url.Values{"taskId": {taskID}, "done": {done}, "expectedRevision": {revision}}), session.ID, "")
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
