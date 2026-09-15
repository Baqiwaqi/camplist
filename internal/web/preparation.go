package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"net/http"
	"slices"
	"strconv"
)

// EditPreparationTask adds, renames, toggles or removes a task on the reusable
// list. htmx gets the preparation card back with the list revision the save
// made; a normal form post is redirected to the list.
func (h *handler) EditPreparationTask(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	task := packing.PreparationTask{Scope: r.PostForm.Get("scope"), ID: r.PostForm.Get("taskId"), Name: r.PostForm.Get("name"), Done: r.PostForm.Get("done") == "true"}
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	remove := r.PostForm.Get("action") == "remove"
	list, err := h.packingStore.EditPreparationTask(r.Context(), chi.URLParam(r, "id"), auth.Subject(r.Context()), task, remove, r.PostForm.Get("revision"))
	if err != nil {
		storeError(w, err, "Could not save preparation task")
		return
	}
	if !isHTMX(r) {
		http.Redirect(w, r, "/packing-lists/"+list.ID, http.StatusSeeOther)
		return
	}
	editing := r.PostForm.Get("editing") == "true" && len(list.Tasks) > 0
	render(w, r, templ.Join(views.ListPreparation(list, csrf.Token(r), editing, preparationFocus(r, list, task, remove)), views.ListRevision(list, true)))
}

// preparationFocus picks the field to focus once the card is swapped. Mark
// done needs none: htmx refocuses the toggle by its id. A rename keeps the
// renamed field, a removal moves to the task the form named, and an add or a
// removal of the last task returns to the new task field.
func preparationFocus(r *http.Request, list packing.PackingList, task packing.PreparationTask, remove bool) string {
	switch {
	case r.PostForm.Get("taskId") == "":
		return "new-task"
	case remove:
		next := r.PostForm.Get("next")
		if slices.ContainsFunc(list.Tasks, func(t packing.PreparationTask) bool { return t.ID == next }) {
			return "task-" + next
		}
		return "new-task"
	case r.PostForm.Get("editing") == "true":
		return "task-" + task.ID
	}
	return ""
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
		render(w, r, views.SessionPreparationSaved(session, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/trips/"+session.ID, http.StatusSeeOther)
}
