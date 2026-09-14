package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
)

// packingStore describes the operations the HTTP layer needs.
type packingStore interface {
	RenameTrip(context.Context, string, string, string) (packing.PackingSession, error)
	ArchiveTrip(context.Context, string, string) (packing.PackingSession, error)
	RestoreTrip(context.Context, string, string) (packing.PackingSession, error)
	SetSessionPreparationTask(context.Context, string, string, string, bool, int64) (packing.PackingSession, error)
	EditPreparationTask(context.Context, string, string, packing.PreparationTask, bool, string) (packing.PackingList, error)
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
	AddItem(context.Context, string, string, packing.PackingItem) (packing.PackingList, string, error)
	RemoveItem(context.Context, string, string, string, string) (packing.PackingList, string, error)
	UpdateItem(context.Context, string, string, packing.PackingItem) (packing.PackingList, string, error)
	CreatePackingSession(context.Context, string, string, ...string) (packing.PackingSession, error)
	StartTripWithMembers(context.Context, string, string, string, []string) (packing.PackingSession, error)
	GetPackingSession(context.Context, string, string) (packing.PackingSession, error)
	SetSessionItem(context.Context, string, string, string, bool) (packing.PackingSession, error)
	DeletePackingSession(context.Context, string, string) error
	RememberedCategories(context.Context, string) ([]string, error)
	RememberCategory(context.Context, string, string) error
	RenameCategory(context.Context, string, string, string) (packing.CategoryRename, error)
	ForgetCategory(context.Context, string, string) error
	AddItems(context.Context, string, string, []packing.PackingItem) (packing.AddItemsResult, error)
	CopyItems(context.Context, string, string, string, []string) (packing.AddItemsResult, error)
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
		sessionExpired(w)
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
		sessionExpired(w)
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
		sessionExpired(w)
		return
	}

	list, err := h.packingStore.GetPackingList(ctx, id, userID)
	if err != nil {
		log.Printf("render ui: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}

	form := packing.NewCreateItemForm(id)
	form.Categories = h.categorySuggestions(ctx, userID, list.Items)

	render(w, r, views.PackingDetails(list.Name, list, form, csrf.Token(r)))
}

func (h *handler) NewListPage(w http.ResponseWriter, r *http.Request) {
	form := packing.NewCreatePackingListForm()
	render(w, r, views.NewPackingListPage(form, csrf.Token(r)))
}

func (h *handler) EditListPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	list, err := h.packingStore.GetPackingList(r.Context(), id, userID)
	if err != nil {
		log.Printf("render ui: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}

	form := packing.EditPackingListForm(list)
	render(w, r, views.EditPackingListPage(list.Name, form, csrf.Token(r)))
}

func (h *handler) NewListHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		malformedRequest(w)
		return
	}

	form := packing.NewCreatePackingListForm()

	// decode the form
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		malformedRequest(w)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)

	// validate the input
	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.NewPackingListPage(form, csrf.Token(r)))
		return
	}
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	list := packing.NewList(userID, form.Name, form.Description)

	err = h.packingStore.SavePackingList(ctx, list)
	if err != nil {
		_, message := storeErrorDetails(err, "Storing packing list failed")
		form.Error = []string{message}
		render(w, r, views.NewPackingListPage(form, csrf.Token(r)))
		return
	}

	// Open the new, empty list: its empty state offers the ways to fill it.
	http.Redirect(w, r, "/packing-lists/"+list.ID, http.StatusSeeOther)
}

func (h *handler) EditListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
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
		malformedRequest(w)
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
		malformedRequest(w)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.EditPackingListPage(list.Name, form, csrf.Token(r)))
		return
	}

	if list.IsShared() && form.Revision != list.Revision() {
		form.Error = []string{"This list changed. Reload it and review the current version before saving."}
		render(w, r, views.EditPackingListPage(list.Name, form, csrf.Token(r)))
		return
	}
	list.Name = form.Name
	list.Description = form.Description

	err = h.packingStore.SavePackingList(ctx, list)
	if err != nil {
		_, message := storeErrorDetails(err, "Storing packing list failed")
		form.Error = []string{message}
		render(w, r, views.EditPackingListPage(list.Name, form, csrf.Token(r)))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) DeleteListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	err = h.packingStore.DeletePackingList(ctx, id, userID)
	if err != nil {
		log.Printf("delete packing list: %v", err)
		storeError(w, err, "Deleting packing list failed")
		return
	}

	// The lists page targets the card, which htmx removes on this empty 200.
	// Anywhere else, such as the list's own details page, redirect rather
	// than refresh: refreshing a deleted list would show an error.
	if r.Header.Get("HX-Target") != views.PackingListCardID(id) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	lists, err := h.packingStore.GetPackingLists(ctx, userID)
	if err != nil {
		// The list is gone; only the empty state could be stale.
		log.Printf("list packing lists after delete: %v", err)
		return
	}
	if len(lists) == 0 {
		render(w, r, views.PackingListsEmpty(true, true))
	}
}

// Packing List Items

func (h *handler) AddItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}

	form := packing.NewCreateItemForm(listID)
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		malformedRequest(w)
		return
	}

	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)
	form.Category = packing.MatchCategory(form.Category)

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		list, err := h.packingStore.GetPackingList(ctx, listID, userID)
		if err != nil {
			log.Printf("render ui: %v", err)
			storeError(w, err, "getting the list failed")
			return
		}
		form.Categories = h.categorySuggestions(ctx, userID, list.Items)
		if isHTMX(r) {
			render(w, r, views.AddItemForm(form, csrf.Token(r), true))
			return
		}
		render(w, r, views.PackingDetails(list.Name, list, form, csrf.Token(r)))
		return
	}

	item := packing.NewItem(form.Name, form.Category)
	item.Scope = form.Scope

	list, replaced, err := h.packingStore.AddItem(ctx, listID, userID, item)
	if err != nil {
		log.Printf("add item to packing list: %v", err)
		storeError(w, err, "Storing item on packing list failed")
		return
	}
	h.rememberCategory(ctx, userID, item.Category)

	if !isHTMX(r) {
		http.Redirect(w, r, "/packing-lists/"+listID, http.StatusSeeOther)
		return
	}
	// Another write landed after the page's revision, so the page is out of
	// date around the new item: reload it rather than advance its revision.
	if replaced != form.Revision {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	added, _ := list.FindItem(item.ID)
	fresh := packing.NewCreateItemForm(listID)
	fresh.Categories = h.categorySuggestions(ctx, userID, list.Items)
	render(w, r, templ.Join(
		views.AddItemForm(fresh, csrf.Token(r), true),
		views.ItemsAppended(listID, []packing.PackingItem{added}),
		listItemsChanged(list),
		views.ListAddStatus("", true),
	))
}

// RemoveItemHandler deletes an item. htmx removes the row itself; the response
// updates the parts of the page that depend on the items.
func (h *handler) RemoveItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	revision := r.Header.Get("X-Camplist-Revision")
	list, replaced, err := h.packingStore.RemoveItem(ctx, listID, userID, itemID, revision)
	if err != nil {
		log.Printf("remove item from packing list: %v", err)
		storeError(w, err, "Removing item off packing list failed")
		return
	}
	if replaced != revision {
		w.Header().Set("HX-Refresh", "true")
		return
	}

	render(w, r, listItemsChanged(list))
}

// listItemsChanged is the out-of-band update after an item write on the list
// page: the item count, empty text and the revision the write made.
func listItemsChanged(list packing.PackingList) templ.Component {
	return templ.Join(
		views.ListSummary(list.Items, true),
		views.ListEmpty(list.ID, list.Items, true),
		views.ListRevision(list, true),
	)
}

// Packing Session

func (h *handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	listID := r.FormValue("listId")
	if listID == "" {
		malformedRequest(w)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	members := r.PostForm["member"]
	if len(name) > 200 {
		h.startTripError(w, r, listID, name, members, []string{"Trip name must be at most 200 characters"}, false)
		return
	}
	var ses packing.PackingSession
	if len(members) == 0 {
		ses, err = h.packingStore.CreatePackingSession(ctx, listID, userID, auth.UserName(ctx))
	} else {
		ses, err = h.packingStore.StartTripWithMembers(ctx, listID, userID, auth.UserName(ctx), members)
	}
	if errors.Is(err, packing.ErrNotListMember) {
		h.startTripError(w, r, listID, name, members, []string{"Someone you chose is no longer a member of this list. Check who to share the trip with."}, true)
		return
	}
	if err != nil {
		log.Printf("create packing session: %v", err)
		storeError(w, err, "Creating packing session failed")
		return
	}

	if name != "" {
		if _, err = h.packingStore.RenameTrip(ctx, ses.ID, userID, name); err != nil {
			storeError(w, err, "Could not name trip")
			return
		}
	}
	// A new trip is a new page: the offline module only starts on a full load.
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/trips/"+ses.ID)
		return
	}
	http.Redirect(w, r, "/trips/"+ses.ID, http.StatusSeeOther)
}

// StartTripPrompt asks which list members join a new trip. htmx swaps the
// prompt into the list hero (choose=members) or, on Cancel, the collapsed
// form back; without scripts it is a page posting to CreateSessionHandler.
func (h *handler) StartTripPrompt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := h.packingStore.GetPackingList(ctx, chi.URLParam(r, "id"), auth.Subject(ctx))
	if err != nil {
		storeError(w, err, "getting the list failed")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = list.Name + " – " + time.Now().Format("Jan 2, 2006")
	}
	start := views.StartTrip{List: list, Name: name, Members: list.TripMemberChoices(auth.Subject(ctx))}
	if !isHTMX(r) {
		start.Prompt, start.Page = len(start.Members) > 0, true
		render(w, r, views.StartTripPage(start, csrf.Token(r)))
		return
	}
	start.Prompt = r.URL.Query().Get("choose") == "members" && len(start.Members) > 0
	start.Focus = !start.Prompt
	render(w, r, views.StartTripForm(start, csrf.Token(r)))
}

// startTripError shows the start-trip form again with its error, keeping the
// typed name and the members that are still offered.
func (h *handler) startTripError(w http.ResponseWriter, r *http.Request, listID, name string, members, errs []string, memberError bool) {
	ctx := r.Context()
	list, err := h.packingStore.GetPackingList(ctx, listID, auth.Subject(ctx))
	if err != nil {
		log.Printf("get packing list: %v", err)
		storeError(w, err, "getting the list failed")
		return
	}
	start := views.StartTrip{List: list, Name: name, Errors: errs, MemberError: memberError, Members: list.TripMemberChoices(auth.Subject(ctx)), Chosen: map[string]bool{}}
	for _, subject := range members {
		start.Chosen[subject] = true
	}
	start.Prompt = len(start.Members) > 0
	if isHTMX(r) {
		render(w, r, views.StartTripForm(start, csrf.Token(r)))
		return
	}
	start.Page = true
	render(w, r, views.StartTripPage(start, csrf.Token(r)))
}

func (h *handler) SessionDetailsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
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
	render(w, r, views.PackingSessionPage(ses.DisplayName(), ses, canReview, h.categorySuggestions(ctx, userID, ses.List.Items), csrf.Token(r)))
}

func (h *handler) SetSessionItemHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	sessionID := r.FormValue("sessionId")
	itemID := r.FormValue("itemId")
	if sessionID == "" || itemID == "" {
		malformedRequest(w)
		return
	}
	checked, err := strconv.ParseBool(r.FormValue("checked"))
	if err != nil {
		malformedRequest(w)
		return
	}
	var session packing.PackingSession
	if raw := r.PostForm.Get("expectedRevision"); raw != "" {
		revision, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || revision < 0 {
			malformedRequest(w)
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

	if isHTMX(r) {
		item, _ := session.List.FindItem(itemID)
		render(w, r, views.PackingItemSaved(session, item, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/trips/"+sessionID, http.StatusSeeOther)
}

func (h *handler) DeletePackingSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	userID, err := auth.UserID(ctx)
	if err != nil {
		sessionExpired(w)
		return
	}

	if id == "" {
		malformedRequest(w)
		return
	}

	err = h.packingStore.DeletePackingSession(ctx, id, userID)
	if err != nil {
		storeError(w, err, "Error deleting packing session from db")
		return
	}

	// static/offline/account.mjs drops or marks this device's offline copy.
	trigger, _ := json.Marshal(map[string]map[string]string{"camplist:trip-deleted": {"id": id}})
	w.Header().Set("HX-Trigger", string(trigger))
	h.tripCardRemoved(w, r, userID, r.URL.Query().Get("view") == "archive")
}
