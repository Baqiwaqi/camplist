package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

func (h *handler) EditPreparationTask(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	task := packing.PreparationTask{ID: r.PostForm.Get("taskId"), Name: r.PostForm.Get("name"), Done: r.PostForm.Get("done") == "true"}
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	err := h.packingStore.EditPreparationTask(r.Context(), chi.URLParam(r, "id"), auth.Subject(r.Context()), task, r.PostForm.Get("action") == "remove", r.PostForm.Get("revision"))
	if err != nil {
		storeError(w, err, "Could not save preparation task")
		return
	}
	http.Redirect(w, r, "/packing-list/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
