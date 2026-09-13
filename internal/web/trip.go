package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
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
	operationID := r.PostForm.Get("operationId")
	if _, err := uuid.Parse(operationID); err != nil {
		malformedRequest(w)
		return
	}
	user := auth.Subject(r.Context())
	// The category is stored as sent: a retry must replay the same operation.
	op := packing.PackingOperation{ID: operationID, ItemID: operationID, Action: "add", Name: r.PostForm.Get("name"), Category: r.PostForm.Get("category"), Kind: r.PostForm.Get("kind"), Scope: r.PostForm.Get("scope"), SaveForFuture: r.PostForm.Get("saveForFuture") == "true"}
	session, err := h.packingStore.SyncSessionItem(r.Context(), id, user, op)
	if err == nil || errors.Is(err, packing.ErrFutureSave) {
		h.rememberCategory(r.Context(), user, op.Category)
	}
	if errors.Is(err, packing.ErrFutureSave) {
		render(w, r, views.FutureSaveResult(session, op, csrf.Token(r)))
		return
	}
	if err != nil {
		storeError(w, err, "Could not save entry")
		return
	}
	http.Redirect(w, r, "/trips/"+id, http.StatusSeeOther)
}
