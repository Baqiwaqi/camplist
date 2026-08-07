package web

import (
	"camplist/internal/packing"
	"camplist/internal/views"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
)

type handler struct {
	packingStore *packing.Store
}

func (h *handler) MainPage(w http.ResponseWriter, r *http.Request) {
	lists, err := h.packingStore.GetPackingLists(r.Context(), packing.DemoUserID)
	if err != nil {
		log.Printf("list packing lists: %v", err)
		http.Error(w, "Listing packing lists failed", http.StatusInternalServerError)
		return
	}

	render(w, r, views.PackingListPage("Camping", lists, csrf.Token(r)))
}

func (h *handler) SessionsPage(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.packingStore.ListPackingSession(r.Context(), packing.DemoUserID)
	if err != nil {

		log.Printf("list packing lists: %v", err)
		http.Error(w, "Listing packing lists failed", http.StatusInternalServerError)
		return
	}

	v := views.PackingSessionsOverviewPage("Packing Sessions", sessions, csrf.Token(r))

	render(w, r, v)
}

func (h *handler) ListDetailsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := h.packingStore.GetPackingList(r.Context(), id, packing.DemoUserID)
	if err != nil {
		log.Printf("render ui: %v", err)
		http.Error(w, "getting the list failed", http.StatusInternalServerError)
		return
	}

	form := packing.NewCreateItemForm(id)

	render(w, r, views.PackingDetails("Packing details", list, form, csrf.Token(r)))
}

func (h *handler) NewListPage(w http.ResponseWriter, r *http.Request) {
	form := packing.NewCreatePackingListForm()
	render(w, r, views.NewPackingListPage("New Packing List", form, csrf.Token(r)))
}

func (h *handler) EditListPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := h.packingStore.GetPackingList(r.Context(), id, packing.DemoUserID)
	if err != nil {
		log.Printf("render ui: %v", err)
		http.Error(w, "getting the list failed", http.StatusInternalServerError)
		return
	}

	form := packing.EditPackingListForm(list)
	render(w, r, views.NewPackingListPage("Edit Packing List", form, csrf.Token(r)))
}

func (h *handler) NewListHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	var form packing.CreatePackingListForm

	// decode the form
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false

	// validate the input
	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.NewPackingListPage("New Packing List", form, csrf.Token(r)))
		return
	}

	list := packing.NewList(packing.DemoUserID, form.Name, form.Description)

	err = h.packingStore.SavePackingList(r.Context(), list)
	if err != nil {
		form.Error = []string{"Storing packing list failed"}
		render(w, r, views.NewPackingListPage("New Packing List", form, csrf.Token(r)))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) EditListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	list, err := h.packingStore.GetPackingList(r.Context(), id, packing.DemoUserID)
	if err != nil {
		log.Printf("render ui: %v", err)
		http.Error(w, "getting the list failed", http.StatusInternalServerError)
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	var form packing.CreatePackingListForm
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err = dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		render(w, r, views.NewPackingListPage("Edit Packing List", form, csrf.Token(r)))
		return
	}

	list.Name = form.Name
	list.Description = form.Description

	err = h.packingStore.SavePackingList(r.Context(), list)
	if err != nil {
		form.Error = []string{"Storing packing list failed"}
		render(w, r, views.NewPackingListPage("Edit Packing List", form, csrf.Token(r)))
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *handler) SaveListHandler(w http.ResponseWriter, r *http.Request) {
	var req packing.CreatePackingList
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	list := packing.NewList(packing.DemoUserID, req.Name, req.Description)

	err := h.packingStore.SavePackingList(r.Context(), list)
	if err != nil {
		log.Printf("save packing list: %v", err)
		http.Error(w, "Storing packing list failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "packing list saved")
}

func (h *handler) DeleteListHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.packingStore.DeletePackingList(r.Context(), id, packing.DemoUserID)
	if err != nil {
		log.Printf("delete packing list: %v", err)
		http.Error(w, "Deleting packing list failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Packing List Items

func (h *handler) AddItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	var form packing.CreateItemForm
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	err := dec.Decode(&form, r.PostForm)
	if err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	form.Initial = false

	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		list, err := h.packingStore.GetPackingList(r.Context(), listID, packing.DemoUserID)
		if err != nil {
			log.Printf("render ui: %v", err)
			http.Error(w, "getting the list failed", http.StatusInternalServerError)
			return
		}
		form := packing.NewCreateItemForm(listID)
		render(w, r, views.PackingDetails("Packing details", list, form, csrf.Token(r)))
		return
	}

	item := packing.NewItem(form.Name, form.Category)

	err = h.packingStore.AddItem(r.Context(), listID, packing.DemoUserID, item)
	if err != nil {
		log.Printf("add item to packing list: %v", err)
		http.Error(w, "Storing item on packing list failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/packing-list/"+listID, http.StatusSeeOther)
}

func (h *handler) RemoveItemHandler(w http.ResponseWriter, r *http.Request) {
	listID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")

	err := h.packingStore.RemoveItem(r.Context(), listID, packing.DemoUserID, itemID)
	if err != nil {
		log.Printf("remove item from packing list: %v", err)
		http.Error(w, "Removing item off packing list failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// Packing Session

func (h *handler) CreateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	listID := r.FormValue("listId")
	if listID == "" {
		http.Error(w, "listId is required", http.StatusBadRequest)
		return
	}

	ses, err := h.packingStore.CreatePackingSession(r.Context(), listID, packing.DemoUserID)
	if err != nil {
		log.Printf("create packing session: %v", err)
		http.Error(w, "Creating packing session failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/packing-session/"+ses.ID)
	w.WriteHeader(http.StatusOK)
}

func (h *handler) SessionDetailsPage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ses, err := h.packingStore.GetPackingSession(r.Context(), id, packing.DemoUserID)
	if err != nil {
		log.Printf("get packing session: %v", err)
		http.Error(w, "Getting packing session failed", http.StatusInternalServerError)
		return
	}

	render(w, r, views.PackingSessionPage("Packing Session", ses, csrf.Token(r)))
}

func (h *handler) ToggleSessionItemHandler(w http.ResponseWriter, r *http.Request) {
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
	_, err := h.packingStore.ToggleSessionItem(r.Context(), sessionID, packing.DemoUserID, itemID)
	if err != nil {
		log.Printf("toggle session item: %v", err)
		http.Error(w, "Toggling session item failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (h *handler) DeletePackingSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if id == "" {
		http.Error(w, "Missing session Id to delete packing session", http.StatusBadRequest)
		return
	}

	err := h.packingStore.DeletePackingSession(r.Context(), id, packing.DemoUserID)
	if err != nil {
		http.Error(w, "Error deleting packing session from db", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
