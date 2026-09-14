package web

import (
	"context"
	"errors"
	"log"
	"net/http"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
)

// bulkList reads the list named in the URL for a bulk add, writing the error
// response when the camper cannot open it.
func (h *handler) bulkList(w http.ResponseWriter, r *http.Request) (string, packing.PackingList, bool) {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		sessionExpired(w)
		return "", packing.PackingList{}, false
	}
	list, err := h.packingStore.GetPackingList(r.Context(), chi.URLParam(r, "id"), userID)
	if err != nil {
		log.Printf("get packing list: %v", err)
		storeError(w, err, "getting the list failed")
		return "", packing.PackingList{}, false
	}
	return userID, list, true
}

// AddSeveralPage shows the Add several form: the dialog's content for htmx,
// otherwise a page of its own.
func (h *handler) AddSeveralPage(w http.ResponseWriter, r *http.Request) {
	userID, list, ok := h.bulkList(w, r)
	if !ok {
		return
	}
	form := packing.NewAddSeveralForm(list)
	form.Categories = h.categorySuggestions(r.Context(), userID, list.Items)
	h.renderAddSeveral(w, r, list, form, "")
}

// AddSeveralHandler adds one item per pasted line. Names already on the list
// are skipped and reported, so a repeated submit adds nothing.
func (h *handler) AddSeveralHandler(w http.ResponseWriter, r *http.Request) {
	userID, list, ok := h.bulkList(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	form := packing.NewAddSeveralForm(list)
	form.Revision = ""
	dec := schema.NewDecoder()
	dec.IgnoreUnknownKeys(true)
	if err := dec.Decode(&form, r.PostForm); err != nil {
		malformedRequest(w)
		return
	}
	form.Categories = h.categorySuggestions(r.Context(), userID, list.Items)
	items, errs := form.Items()
	if len(errs) > 0 {
		form.Error = errs
		h.renderAddSeveral(w, r, list, form, "")
		return
	}
	result, err := h.packingStore.AddItems(r.Context(), list.ID, userID, items)
	if err != nil {
		log.Printf("add several items: %v", err)
		status, message := storeErrorDetails(err, "Adding the items failed")
		if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
			http.Error(w, message, status)
			return
		}
		form.Error = []string{message}
		h.renderAddSeveral(w, r, list, form, "")
		return
	}
	h.rememberCategories(r.Context(), userID, result.Added)
	summary := result.Summary("")
	if isHTMX(r) {
		h.renderBulkAdded(w, r, form.Revision, result, summary)
		return
	}
	next := packing.NewAddSeveralForm(result.List)
	next.Category = form.Category
	next.Categories = h.categorySuggestions(r.Context(), userID, result.List.Items)
	h.renderAddSeveral(w, r, result.List, next, summary)
}

func (h *handler) renderAddSeveral(w http.ResponseWriter, r *http.Request, list packing.PackingList, form packing.AddSeveralForm, result string) {
	if isHTMX(r) {
		render(w, r, views.AddSeveralPanel(list, form, csrf.Token(r), false, result))
		return
	}
	render(w, r, views.BulkAddPage(list, "Add several items", csrf.Token(r), views.AddSeveralPanel(list, form, csrf.Token(r), true, result)))
}

// AddFromListPage shows the camper's other lists to copy items from.
func (h *handler) AddFromListPage(w http.ResponseWriter, r *http.Request) {
	userID, list, ok := h.bulkList(w, r)
	if !ok {
		return
	}
	lists, err := h.packingStore.GetPackingLists(r.Context(), userID)
	if err != nil {
		log.Printf("get packing lists: %v", err)
		storeError(w, err, "Reading your lists failed")
		return
	}
	sources := make([]packing.PackingList, 0, len(lists))
	for _, source := range lists {
		if source.ID != list.ID {
			sources = append(sources, source)
		}
	}
	if isHTMX(r) {
		render(w, r, views.AddFromListSources(list, sources, false))
		return
	}
	render(w, r, views.BulkAddPage(list, "Add from a list", csrf.Token(r), views.AddFromListSources(list, sources, true)))
}

// sourceList reads the source list named in the URL with the camper's own
// access, writing the error response when they cannot read it.
func (h *handler) sourceList(w http.ResponseWriter, r *http.Request, userID string, list packing.PackingList) (packing.PackingList, bool) {
	sourceID := chi.URLParam(r, "sourceId")
	if sourceID == list.ID {
		http.Error(w, "Pick another list to copy items from.", http.StatusBadRequest)
		return packing.PackingList{}, false
	}
	source, err := h.packingStore.GetPackingList(r.Context(), sourceID, userID)
	if err != nil {
		log.Printf("get source list: %v", err)
		storeError(w, err, "Reading that list failed")
		return packing.PackingList{}, false
	}
	return source, true
}

// AddFromListItemsPage shows a source list's items to tick.
func (h *handler) AddFromListItemsPage(w http.ResponseWriter, r *http.Request) {
	userID, list, ok := h.bulkList(w, r)
	if !ok {
		return
	}
	source, ok := h.sourceList(w, r, userID, list)
	if !ok {
		return
	}
	h.renderAddFromList(w, r, views.AddFromList{List: list, Source: source})
}

// AddFromListHandler copies the ticked items of a source list into this list.
// The source is read again, so only items still on it are copied; names
// already on this list are skipped, so a repeated submit adds nothing.
func (h *handler) AddFromListHandler(w http.ResponseWriter, r *http.Request) {
	userID, list, ok := h.bulkList(w, r)
	if !ok {
		return
	}
	source, ok := h.sourceList(w, r, userID, list)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	selected := r.PostForm["item"]
	step := views.AddFromList{List: list, Source: source, Selected: map[string]bool{}}
	for _, id := range selected {
		step.Selected[id] = true
	}
	if len(selected) == 0 {
		step.Errors = []string{"Tick at least one item to add."}
		h.renderAddFromList(w, r, step)
		return
	}
	result, err := h.packingStore.CopyItems(r.Context(), list.ID, source.ID, userID, selected)
	if err != nil {
		log.Printf("copy items: %v", err)
		status, message := storeErrorDetails(err, "Adding the items failed")
		if status != http.StatusBadRequest && status != http.StatusUnprocessableEntity {
			http.Error(w, message, status)
			return
		}
		if errors.Is(err, packing.ErrItemsGone) {
			message = "Those items are no longer on " + source.Name + "."
		} else if status == http.StatusBadRequest {
			message = "Some of those items cannot be copied. Check their names and categories on " + source.Name + "."
		}
		step.Errors = []string{message}
		h.renderAddFromList(w, r, step)
		return
	}
	h.rememberCategories(r.Context(), userID, result.Added)
	summary := result.Summary("from " + source.Name)
	if isHTMX(r) {
		h.renderBulkAdded(w, r, r.PostForm.Get("revision"), result, summary)
		return
	}
	h.renderAddFromList(w, r, views.AddFromList{List: result.List, Source: source, Selected: step.Selected, Result: summary})
}

func (h *handler) renderAddFromList(w http.ResponseWriter, r *http.Request, step views.AddFromList) {
	if isHTMX(r) {
		render(w, r, views.AddFromListItems(step, csrf.Token(r), false))
		return
	}
	render(w, r, views.BulkAddPage(step.List, "From "+step.Source.Name, csrf.Token(r), views.AddFromListItems(step, csrf.Token(r), true)))
}

// rememberCategories keeps the custom categories of items just added.
func (h *handler) rememberCategories(ctx context.Context, userID string, items []packing.PackingItem) {
	seen := map[string]bool{}
	for _, item := range items {
		if item.Category != "" && !seen[item.Category] {
			seen[item.Category] = true
			h.rememberCategory(ctx, userID, item.Category)
		}
	}
}

// renderBulkAdded answers a bulk add from the list page's dialog. Everything
// in the answer is out of band, so the dialog's panel is left empty, which
// closes it: the summary, the new rows, and the item count, empty state and
// revision the write made. When another write landed after the revision the
// page showed, the page is out of date around the new rows, so it reloads
// instead, as a single add does.
func (h *handler) renderBulkAdded(w http.ResponseWriter, r *http.Request, submitted string, result packing.AddItemsResult, summary string) {
	if len(result.Added) > 0 && result.Replaced != submitted {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	parts := []templ.Component{views.ListAddStatus(summary, true)}
	if len(result.Added) > 0 {
		added := make([]packing.PackingItem, 0, len(result.Added))
		for _, item := range result.Added {
			saved, _ := result.List.FindItem(item.ID)
			added = append(added, saved)
		}
		parts = append(parts, views.ItemsAppended(result.List.ID, added), listItemsChanged(result.List))
	}
	render(w, r, templ.Join(parts...))
}
