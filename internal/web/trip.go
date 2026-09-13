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
	"time"
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

// ArchiveTrip archives a trip from its card. The card's swap removes it from
// the trips overview; the out-of-band link keeps the archive count current.
func (h *handler) ArchiveTrip(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := h.packingStore.ArchiveTrip(ctx, chi.URLParam(r, "id"), userID); err != nil {
		storeError(w, err, "Could not archive trip")
		return
	}
	sessions, err := h.packingStore.ListPackingSession(ctx, userID)
	if err != nil {
		storeError(w, err, "Could not count archived trips")
		return
	}
	_, archived := packing.PartitionSessions(sessions, time.Now().UTC())
	render(w, r, views.TripArchiveLink(len(archived), true))
}

// RestoreTrip undoes a manual archive; the card's swap removes it from the
// archive page.
func (h *handler) RestoreTrip(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := h.packingStore.RestoreTrip(ctx, chi.URLParam(r, "id"), userID); err != nil {
		storeError(w, err, "Could not restore trip")
		return
	}
	w.WriteHeader(http.StatusOK)
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
	op := packing.PackingOperation{ID: operationID, ItemID: operationID, Action: "add", Name: r.PostForm.Get("name"), Category: r.PostForm.Get("category"), Kind: r.PostForm.Get("kind"), Scope: r.PostForm.Get("scope"), SaveForFuture: r.PostForm.Get("saveForFuture") == "true"}
	session, err := h.packingStore.SyncSessionItem(r.Context(), id, auth.Subject(r.Context()), op)
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
