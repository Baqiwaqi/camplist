package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"net/http"
	"strconv"
)

func (h *handler) EditPreparationTask(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	task := packing.PreparationTask{Scope: r.PostForm.Get("scope"), ID: r.PostForm.Get("taskId"), Name: r.PostForm.Get("name"), Done: r.PostForm.Get("done") == "true"}
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	revision := r.PostForm.Get("revision")
	list, err := h.packingStore.EditPreparationTask(r.Context(), chi.URLParam(r, "id"), auth.Subject(r.Context()), task, r.PostForm.Get("action") == "remove", revision)
	if err != nil {
		storeError(w, err, "Could not save preparation task")
		return
	}
	// Mark done swaps only the task list; remove still reloads the page.
	if isHTMX(r) && r.Header.Get("HX-Target") == "preparation-tasks" {
		// Controls elsewhere on the page still carry the submitted revision;
		// static/revision.js moves exactly those to the revision this save made.
		trigger, _ := json.Marshal(map[string]map[string]string{"list-revision": {"from": revision, "to": list.Revision()}})
		w.Header().Set("HX-Trigger", string(trigger))
		render(w, r, views.PreparationTasks(list, csrf.Token(r)))
		return
	}
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/packing-lists/"+chi.URLParam(r, "id"))
		return
	}
	http.Redirect(w, r, "/packing-lists/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}

func (h *handler) SetSessionPreparationTask(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	done, err := strconv.ParseBool(r.PostForm.Get("done"))
	if err != nil {
		malformedRequest(w)
		return
	}
	revision, err := strconv.ParseInt(r.PostForm.Get("expectedRevision"), 10, 64)
	if err != nil || revision < 0 {
		malformedRequest(w)
		return
	}
	session, err := h.packingStore.SetSessionPreparationTask(r.Context(), chi.URLParam(r, "id"), auth.Subject(r.Context()), r.PostForm.Get("taskId"), done, revision)
	if err != nil {
		storeError(w, err, "Could not update preparation task")
		return
	}
	if isHTMX(r) {
		render(w, r, views.SessionPreparation(session, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/trips/"+session.ID, http.StatusSeeOther)
}
