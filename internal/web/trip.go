package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"log"
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

// ArchiveTrip archives a trip from its card on the trips overview.
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
	h.tripCardRemoved(w, r, userID, false)
}

// RestoreTrip undoes a manual archive from its card on the archive page.
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
	h.tripCardRemoved(w, r, userID, true)
}

// tripCardRemoved answers a trip card action after the change is saved. The
// card's own swap removes it; this sends the page's empty state and archive
// count out of band. If the recount fails the change still stands, so the
// card goes and only those extras stay stale.
func (h *handler) tripCardRemoved(w http.ResponseWriter, r *http.Request, userID string, archivePage bool) {
	sessions, err := h.packingStore.ListPackingSession(r.Context(), userID)
	if err != nil {
		log.Printf("recount trips after card action: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	active, archived := packing.PartitionSessions(sessions, time.Now().UTC())
	remaining := len(active)
	if archivePage {
		remaining = len(archived)
	}
	render(w, r, views.TripCardRemoved(archivePage, remaining, len(archived)))
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
