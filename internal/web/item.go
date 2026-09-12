package web

import (
	"log"
	"net/http"
	"strings"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
)

// isHTMX reports whether htmx asked for a fragment. Boosted navigations want full pages.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true"
}

// loadItem reads the list and item named in the URL, writing the error response when either is missing.
func (h *handler) loadItem(w http.ResponseWriter, r *http.Request) (userID string, list packing.PackingList, item packing.PackingItem, ok bool) {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", list, item, false
	}
	list, err = h.packingStore.GetPackingList(r.Context(), chi.URLParam(r, "id"), userID)
	if err != nil {
		log.Printf("get packing list: %v", err)
		storeError(w, err, "getting the list failed")
		return "", list, item, false
	}
	item, ok = list.FindItem(chi.URLParam(r, "itemId"))
	if !ok {
		http.Error(w, "item not found", http.StatusNotFound)
		return "", list, item, false
	}
	return userID, list, item, true
}

// EditItemPage shows the edit form: a row fragment for htmx, otherwise its own page.
func (h *handler) EditItemPage(w http.ResponseWriter, r *http.Request) {
	_, list, item, ok := h.loadItem(w, r)
	if !ok {
		return
	}
	h.renderItemForm(w, r, list, item, packing.EditItemForm(list.ID, item))
}

// ItemRowHandler returns the read-only row, which is how an inline edit is cancelled.
func (h *handler) ItemRowHandler(w http.ResponseWriter, r *http.Request) {
	_, list, item, ok := h.loadItem(w, r)
	if !ok {
		return
	}
	if isHTMX(r) {
		render(w, r, views.ItemRow(list.ID, item, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/packing-list/"+list.ID, http.StatusSeeOther)
}

// EditItemHandler saves a renamed or recategorised item.
func (h *handler) EditItemHandler(w http.ResponseWriter, r *http.Request) {
	userID, list, item, ok := h.loadItem(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	form := packing.EditItemForm(list.ID, item)
	form.Revision = ""
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	if err := dec.Decode(&form, r.PostForm); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	form.Initial = false
	form.Name = strings.TrimSpace(form.Name)
	form.Category = strings.TrimSpace(form.Category)
	if errs := form.Validate(); len(errs) > 0 {
		form.Error = errs
		h.renderItemForm(w, r, list, item, form)
		return
	}

	if list.IsShared() && form.Revision != list.Revision() {
		form.Error = []string{"This list changed. Cancel and reopen the item to review the current version."}
		h.renderItemForm(w, r, list, item, form)
		return
	}
	item.SourceRevision = form.Revision
	item.Name = form.Name
	item.Category = form.Category
	if err := h.packingStore.UpdateItem(r.Context(), list.ID, userID, item); err != nil {
		log.Printf("update item: %v", err)
		_, message := storeErrorDetails(err, "Saving the item failed")
		form.Error = []string{message}
		h.renderItemForm(w, r, list, item, form)
		return
	}
	if isHTMX(r) {
		// Read the committed row so subsequent actions use its current ETag.
		h.ItemRowHandler(w, r)
		return
	}
	http.Redirect(w, r, "/packing-list/"+list.ID, http.StatusSeeOther)
}

func (h *handler) renderItemForm(w http.ResponseWriter, r *http.Request, list packing.PackingList, item packing.PackingItem, form packing.CreateItemForm) {
	if isHTMX(r) {
		render(w, r, views.ItemEditRow(list.ID, item.ID, form, csrf.Token(r)))
		return
	}
	render(w, r, views.EditItemPage(list, form, csrf.Token(r)))
}
