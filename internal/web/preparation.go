package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
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
	err := h.packingStore.EditPreparationTask(r.Context(), chi.URLParam(r, "id"), auth.Subject(r.Context()), task, r.PostForm.Get("action") == "remove", r.PostForm.Get("revision"))
	if err != nil {
		storeError(w, err, "Could not save preparation task")
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
		http.Error(w, "invalid task state", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(r.PostForm.Get("expectedRevision"), 10, 64)
	if err != nil || revision < 0 {
		http.Error(w, "invalid task revision", http.StatusBadRequest)
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
