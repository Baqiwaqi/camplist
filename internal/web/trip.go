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
	session, err := h.packingStore.RenameTrip(r.Context(), id, auth.Subject(r.Context()), r.PostForm.Get("name"))
	if err != nil {
		storeError(w, err, "Could not rename trip")
		return
	}
	if isHTMX(r) {
		// The offline copy keeps its own snapshot; the event makes it fetch the new name.
		w.Header().Set("HX-Trigger", "camplist:trip-renamed")
		render(w, r, views.TripRenamed(session))
		return
	}
	http.Redirect(w, r, "/trips/"+id, http.StatusSeeOther)
}

// ArchiveTrip archives a trip from its card on the trips overview, or from
// the trip page's More menu.
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
// card's own swap removes it; this sends the page's empty state and tab
// counts out of band. If the recount fails the change still stands, so the
// card goes and only those extras stay stale.
//
// From the trip's own page (from=trip) there is no card: the camper goes to
// the trips page, where the overview reconciles any copy on this device.
func (h *handler) tripCardRemoved(w http.ResponseWriter, r *http.Request, userID string, archivePage bool) {
	if r.URL.Query().Get("from") == "trip" {
		w.Header().Set("HX-Redirect", "/trips")
		w.WriteHeader(http.StatusOK)
		return
	}
	sessions, err := h.packingStore.ListPackingSession(r.Context(), userID)
	if err != nil {
		log.Printf("recount trips after card action: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}
	active, archived := packing.PartitionSessions(sessions, time.Now().UTC())
	render(w, r, views.TripCardRemoved(archivePage, len(active), len(archived)))
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
	if op.Kind != "task" && (err == nil || errors.Is(err, packing.ErrFutureSave)) {
		h.rememberCategory(r.Context(), user, op.Category)
	}
	if errors.Is(err, packing.ErrFutureSave) {
		if isHTMX(r) {
			render(w, r, views.TripEntrySaved(session, op, h.categorySuggestions(r.Context(), user, session.List.Items), true, csrf.Token(r)))
			return
		}
		render(w, r, views.FutureSaveResult(session, op, csrf.Token(r)))
		return
	}
	if err != nil {
		storeError(w, err, "Could not save entry")
		return
	}
	if isHTMX(r) {
		render(w, r, views.TripEntrySaved(session, op, h.categorySuggestions(r.Context(), user, session.List.Items), false, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/trips/"+id, http.StatusSeeOther)
}
