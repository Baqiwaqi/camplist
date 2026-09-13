package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
)

// packingStore describes the operations the HTTP layer needs.
type packingStore interface {
	RenameTrip(context.Context, string, string, string) (packing.PackingSession, error)
	SetSessionPreparationTask(context.Context, string, string, string, bool, int64) (packing.PackingSession, error)
	EditPreparationTask(context.Context, string, string, packing.PreparationTask, bool, string) error
	GetSharing(context.Context, string, string, string) (packing.SharingView, error)
	CreateInvitation(context.Context, string, string, string) (packing.InvitationLink, error)
	InvitationStatus(context.Context, packing.InvitationLink, string) (string, error)
	RequestAccess(context.Context, packing.InvitationLink, packing.Member) error
	DecideInvitation(context.Context, string, string, string, string, bool) error
	ListTrips(context.Context, string, string) ([]packing.PackingSession, error)
	ApproveInvitationWithTrips(context.Context, string, string, string, []string) error
	RemoveMember(context.Context, string, string, string, string) error
	AddReviewEntry(context.Context, string, string, packing.ReviewEntry) (packing.PackingSession, error)
	ApplyReview(context.Context, string, string, string, []string) (packing.PackingList, error)
	RecoverReviewTemplate(context.Context, string, string) (packing.PackingList, error)
	SetPreparationTask(context.Context, string, string, string, bool, string) (packing.PackingList, error)
	SyncSessionItem(context.Context, string, string, packing.PackingOperation) (packing.PackingSession, error)
	GetPackingLists(context.Context, string) ([]packing.PackingList, error)
	ListPackingSession(context.Context, string) ([]packing.PackingSession, error)
	GetPackingList(context.Context, string, string) (packing.PackingList, error)
	SavePackingList(context.Context, packing.PackingList) error
	DeletePackingList(context.Context, string, string) error
	AddItem(context.Context, string, string, packing.PackingItem) error
	RemoveItem(context.Context, string, string, string, ...string) error
	UpdateItem(context.Context, string, string, packing.PackingItem) error
	CreatePackingSession(context.Context, string, string, ...string) (packing.PackingSession, error)
	GetPackingSession(context.Context, string, string) (packing.PackingSession, error)
	SetSessionItem(context.Context, string, string, string, bool) (packing.PackingSession, error)
	DeletePackingSession(context.Context, string, string) error
}

type handler struct {
	packingStore packingStore
}

func (h *handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.Login("Sign in"))
}

func (h *handler) MainPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	lists, err := h.packingStore.GetPackingLists(r.Context(), userID)
	if err != nil {
		log.Printf("list packing lists: %v", err)
		storeError(w, err, "Listing packing lists failed")
		return
	}

	render(w, r, views.PackingListPage("Your lists", lists, csrf.Token(r)))
}

func (h *handler) SessionsPage(w http.ResponseWriter, r *http.Request) {
	h.sessionsPage(w, r, false)
}

func (h *handler) ArchivedSessionsPage(w http.ResponseWriter, r *http.Request) {
	h.sessionsPage(w, r, true)
}

func (h *handler) sessionsPage(w http.ResponseWriter, r *http.Request, archive bool) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.packingStore.ListPackingSession(r.Context(), userID)
	if err != nil {

		log.Printf("list packing lists: %v", err)
		storeError(w, err, "Listing packing lists failed")
		return
	}

	active, archived := packing.PartitionSessions(sessions, time.Now().UTC())
	if archive {
		render(w, r, views.ArchivedPackingSessionsPage("Trip archive", archived, csrf.Token(r)))
		return
	}

	render(w, r, views.PackingSessionsOverviewPage("Sessions", active, len(archived), csrf.Token(r)))
}

func (h *handler) ListDetailsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	list, err := h.packingStore.GetPackingList(ctx, id, userID)
	if err != nil {
		log.Printf("render ui: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}

	form := packing.NewCreateItemForm(id)

	render(w, r, views.PackingDetails(list.Name, list, form, csrf.Token(r)))
}

func (h *handler) NewListPage(w http.ResponseWriter, r *http.Request) {
	form := packing.NewCreatePackingListForm()
	render(w, r, views.NewPackingListPage("New list", "Start a new list", form, csrf.Token(r)))
}

func (h *handler) EditListPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	list, err := h.packingStore.GetPackingList(r.Context(), id, userID)
	if err != nil {
		log.Printf("render ui: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}

	form := packing.EditPackingListForm(list)
	render(w, r, views.NewPackingListPage("Edit list", list.Name, form, csrf.Token(r)))
}

func (h *handler) NewListHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form := packing.NewCreatePackingListForm()

	// decode the form
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)

	// validate the input
	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.NewPackingListPage("New list", "Start a new list", form, csrf.Token(r)))
		return
	}
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	list := packing.NewList(userID, form.Name, form.Description)

	err = h.packingStore.SavePackingList(ctx, list)
	if err != nil {
		_, message := storeErrorDetails(err, "Storing packing list failed")
		form.Error = []string{message}
		render(w, r, views.NewPackingListPage("New list", "Start a new list", form, csrf.Token(r)))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) EditListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	list, err := h.packingStore.GetPackingList(ctx, id, userID)
	if err != nil {
		log.Printf("render ui: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form := packing.EditPackingListForm(list)
	// Decode editable values from the submission, retaining only server-owned metadata.
	form.Name = ""
	form.Revision = ""
	form.Description = ""
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.NewPackingListPage("Edit list", list.Name, form, csrf.Token(r)))
		return
	}

	if list.IsShared() && form.Revision != list.Revision() {
		form.Error = []string{"This list changed. Reload it and review the current version before saving."}
		render(w, r, views.NewPackingListPage("Edit list", list.Name, form, csrf.Token(r)))
		return
	}
	list.Name = form.Name
	list.Description = form.Description

	err = h.packingStore.SavePackingList(ctx, list)
	if err != nil {
		_, message := storeErrorDetails(err, "Storing packing list failed")
		form.Error = []string{message}
		render(w, r, views.NewPackingListPage("Edit list", list.Name, form, csrf.Token(r)))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) DeleteListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.packingStore.DeletePackingList(ctx, id, userID)
	if err != nil {
		log.Printf("delete packing list: %v", err)
		storeError(w, err, "Deleting packing list failed")
		return
	}

	// Redirect rather than refresh: the list's own details page can also
	// delete it, and refreshing a deleted list would show an error.
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

// Packing List Items

func (h *handler) AddItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form := packing.NewCreateItemForm(listID)
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs

		list, err := h.packingStore.GetPackingList(ctx, listID, userID)
		if err != nil {
			log.Printf("render ui: %v", err)
			storeError(w, err, "getting the list failed")
			return
		}
		render(w, r, views.PackingDetails(list.Name, list, form, csrf.Token(r)))
		return
	}

	item := packing.NewItem(form.Name, form.Category)
	item.Scope = form.Scope

	err = h.packingStore.AddItem(ctx, listID, userID, item)
	if err != nil {
		log.Printf("add item to packing list: %v", err)
		storeError(w, err, "Storing item on packing list failed")
		return
	}

	http.Redirect(w, r, "/packing-lists/"+listID, http.StatusSeeOther)
}

func (h *handler) RemoveItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.packingStore.RemoveItem(ctx, listID, userID, itemID, r.Header.Get("X-Camplist-Revision"))
	if err != nil {
		log.Printf("remove item from packing list: %v", err)
		storeError(w, err, "Removing item off packing list failed")
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Packing Session

func (h *handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	listID := r.FormValue("listId")
	if listID == "" {
		http.Error(w, "listId is required", http.StatusBadRequest)
		return
	}

	if len(strings.TrimSpace(r.FormValue("name"))) > 200 {
		http.Error(w, "Trip name must be at most 200 characters", 400)
		return
	}
	ses, err := h.packingStore.CreatePackingSession(ctx, listID, userID, auth.UserName(ctx))
	if err != nil {
		log.Printf("create packing session: %v", err)
		storeError(w, err, "Creating packing session failed")
		return
	}

	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		if _, err = h.packingStore.RenameTrip(ctx, ses.ID, userID, name); err != nil {
			storeError(w, err, "Could not name trip")
			return
		}
	}
	w.Header().Set("HX-Redirect", "/trips/"+ses.ID)
	w.WriteHeader(http.StatusOK)
}

func (h *handler) SessionDetailsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ses, err := h.packingStore.GetPackingSession(r.Context(), id, userID)
	if err != nil {
		log.Printf("get packing session: %v", err)
		storeError(w, err, "Getting packing session failed")
		return
	}

	canReview := ses.UserID == userID
	if !canReview {
		_, templateErr := h.packingStore.GetPackingList(ctx, ses.TemplateID(), userID)
		canReview = templateErr == nil
	}
	render(w, r, views.PackingSessionPage(ses.DisplayName(), ses, canReview, csrf.Token(r)))
}

func (h *handler) SetSessionItemHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	sessionID := r.FormValue("sessionId")
	itemID := r.FormValue("itemId")
	if sessionID == "" || itemID == "" {
		http.Error(w, "sessionId and itemId are required", http.StatusBadRequest)
		return
	}
	checked, err := strconv.ParseBool(r.FormValue("checked"))
	if err != nil {
		http.Error(w, "checked must be true or false", http.StatusBadRequest)
		return
	}
	var session packing.PackingSession
	if raw := r.PostForm.Get("expectedRevision"); raw != "" {
		revision, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || revision < 0 {
			http.Error(w, "Invalid item revision", 400)
			return
		}
		session, err = h.packingStore.SyncSessionItem(ctx, sessionID, userID, packing.PackingOperation{ID: uuid.NewString(), ItemID: itemID, Checked: checked, ExpectedRevision: revision})
	} else {
		session, err = h.packingStore.SetSessionItem(ctx, sessionID, userID, itemID, checked)
	}
	if err != nil {
		log.Printf("set session item: %v", err)
		storeError(w, err, "Updating packed item failed")
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		render(w, r, views.PackingChecklist(session, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/trips/"+sessionID, http.StatusSeeOther)
}

func (h *handler) DeletePackingSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if id == "" {
		http.Error(w, "Missing session Id to delete packing session", http.StatusBadRequest)
		return
	}

	err = h.packingStore.DeletePackingSession(ctx, id, userID)
	if err != nil {
		storeError(w, err, "Error deleting packing session from db")
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
