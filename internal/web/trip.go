package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
)

func (h *handler) RenameTrip(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if _, err := h.packingStore.RenameTrip(r.Context(), id, auth.Subject(r.Context()), r.PostForm.Get("name")); err != nil {
		storeError(w, err, "Could not rename trip")
		return
	}
	http.Redirect(w, r, "/trips/"+id, http.StatusSeeOther)
}
func (h *handler) AddTripEntry(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	op := packing.PackingOperation{ID: uuid.NewString(), ItemID: uuid.NewString(), Action: "add", Name: r.PostForm.Get("name"), Category: r.PostForm.Get("category"), Kind: r.PostForm.Get("kind"), Scope: r.PostForm.Get("scope"), SaveForFuture: r.PostForm.Get("saveForFuture") == "true"}
	if _, err := h.packingStore.SyncSessionItem(r.Context(), id, auth.Subject(r.Context()), op); err != nil {
		storeError(w, err, "Could not save entry")
		return
	}
	http.Redirect(w, r, "/trips/"+id, http.StatusSeeOther)
}
