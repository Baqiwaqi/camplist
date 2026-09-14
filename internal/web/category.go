package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"

	"github.com/a-h/templ"
	"github.com/gorilla/csrf"
)

// categorySuggestions lists what a category picker offers: the defaults, the
// categories on the items in view, then the ones the signed-in camper has
// remembered. When the remembered ones cannot be read the picker still offers
// the rest.
func (h *handler) categorySuggestions(ctx context.Context, userID string, items []packing.PackingItem) []packing.CategoryOption {
	remembered, err := h.packingStore.RememberedCategories(ctx, userID)
	if err != nil {
		log.Printf("read remembered categories: %v", err)
	}
	return packing.CategoryOptions(packing.CategorySuggestions(packing.ItemCategories(items), remembered), remembered)
}

// rememberCategory keeps a category the camper just saved so their other lists
// offer it too. The item is already stored, so a failure is only logged.
func (h *handler) rememberCategory(ctx context.Context, userID, category string) {
	if err := h.packingStore.RememberCategory(ctx, userID, category); err != nil {
		log.Printf("remember category: %v", err)
	}
}

// closeCategory returns the existing category item's category looks like a
// typo of, when the item was just filed under it as a new one: the defaults,
// the other items' categories and the remembered ones other than itself.
func (h *handler) closeCategory(ctx context.Context, userID string, items []packing.PackingItem, item packing.PackingItem) string {
	if item.Category == "" || packing.IsDefaultCategory(item.Category) {
		return ""
	}
	others := slices.DeleteFunc(slices.Clone(items), func(other packing.PackingItem) bool { return other.ID == item.ID })
	remembered, err := h.packingStore.RememberedCategories(ctx, userID)
	if err != nil {
		log.Printf("read remembered categories: %v", err)
	}
	remembered = slices.DeleteFunc(slices.Clone(remembered), func(category string) bool {
		return strings.EqualFold(strings.TrimSpace(category), strings.TrimSpace(item.Category))
	})
	return packing.CloseCategory(item.Category, packing.CategorySuggestions(packing.ItemCategories(others), remembered))
}

// listPathChecking is the list page after a save without scripts. When the
// item was filed under a category close to an existing one, the page asks
// about it (views.CategoryHint).
func listPathChecking(listID, itemID, closeMatch string) string {
	if closeMatch == "" {
		return "/packing-lists/" + listID
	}
	return "/packing-lists/" + listID + "?check=" + url.QueryEscape(itemID)
}

// CategoriesPage lists the signed-in camper's remembered categories to rename
// or remove without scripts.
func (h *handler) CategoriesPage(w http.ResponseWriter, r *http.Request) {
	h.renderCategories(w, r, http.StatusOK, "", nil)
}

func (h *handler) renderCategories(w http.ResponseWriter, r *http.Request, status int, message string, errs []string) {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		sessionExpired(w)
		return
	}
	categories, err := h.packingStore.RememberedCategories(r.Context(), userID)
	if err != nil {
		log.Printf("read remembered categories: %v", err)
		storeError(w, err, "Reading your categories failed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, views.CategoriesPage(categories, message, errs, csrf.Token(r)))
}

// RenameCategory renames one of the camper's remembered categories and the
// items under it on the lists they own. htmx gets a categories-changed event
// for the pickers and, when the list in view changed, its new category tags
// and revision out of band.
func (h *handler) RenameCategory(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		sessionExpired(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	from := strings.TrimSpace(r.PostForm.Get("from"))
	result, err := h.packingStore.RenameCategory(r.Context(), userID, from, r.PostForm.Get("to"))
	if err != nil {
		status, message := categoryError(err, from, "Renaming the category failed")
		if !errors.Is(err, packing.ErrInvalid) && !errors.Is(err, packing.ErrNotFound) {
			log.Printf("rename category: %v", err)
		}
		h.categoryFailed(w, r, status, message)
		return
	}
	message := renameMessage(from, result)
	if !isHTMX(r) {
		h.renderCategories(w, r, http.StatusOK, message, nil)
		return
	}
	if err := setCategoriesChanged(w, from, result.Category, message); err != nil {
		log.Printf("encode categories-changed: %v", err)
	}
	listID := listInView(r)
	for _, renamed := range result.Lists {
		if renamed.List.ID != listID {
			continue
		}
		// The page showed an older revision than the one renamed: reload it
		// rather than advance its revision past changes it never showed.
		if r.PostForm.Get("revision") != renamed.Replaced {
			w.Header().Set("HX-Refresh", "true")
			return
		}
		var parts []templ.Component
		for _, item := range renamed.List.Items {
			if item.Category == result.Category {
				parts = append(parts, views.ItemCategoryTag(item, true))
			}
		}
		render(w, r, templ.Join(append(parts, views.ListRevision(renamed.List, true))...))
	}
}

// RemoveCategory drops a remembered category from the camper's suggestions.
// Items keep their category.
func (h *handler) RemoveCategory(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.UserID(r.Context())
	if err != nil {
		sessionExpired(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		malformedRequest(w)
		return
	}
	name := strings.TrimSpace(r.PostForm.Get("name"))
	if err := h.packingStore.ForgetCategory(r.Context(), userID, name); err != nil {
		status, message := categoryError(err, name, "Removing the category failed")
		if !errors.Is(err, packing.ErrInvalid) {
			log.Printf("forget category: %v", err)
		}
		h.categoryFailed(w, r, status, message)
		return
	}
	message := "Removed “" + name + "” from your suggestions. Items keep their category."
	if !isHTMX(r) {
		h.renderCategories(w, r, http.StatusOK, message, nil)
		return
	}
	if err := setCategoriesChanged(w, name, "", message); err != nil {
		log.Printf("encode categories-changed: %v", err)
	}
}

func (h *handler) categoryFailed(w http.ResponseWriter, r *http.Request, status int, message string) {
	if isHTMX(r) {
		http.Error(w, message, status)
		return
	}
	h.renderCategories(w, r, status, "", []string{message})
}

func categoryError(err error, category, fallback string) (int, string) {
	switch {
	case packing.IsDefaultCategory(category):
		return http.StatusBadRequest, "Built-in categories cannot be renamed or removed."
	case errors.Is(err, packing.ErrInvalid):
		return http.StatusBadRequest, "Type a new name of up to 100 characters."
	case errors.Is(err, packing.ErrNotFound):
		return http.StatusNotFound, "That category is no longer in your suggestions. Reload the page to see them."
	}
	return storeErrorDetails(err, fallback)
}

// setCategoriesChanged asks htmx to fire categories-changed, which
// static/category-picker.js handles: from is replaced by to, or removed when
// to is empty, and message is shown.
func setCategoriesChanged(w http.ResponseWriter, from, to, message string) error {
	detail := map[string]any{"from": from, "to": to, "custom": to != "" && !packing.IsDefaultCategory(to), "message": message}
	body, err := json.Marshal(map[string]any{"categories-changed": detail})
	if err != nil {
		return err
	}
	w.Header().Set("HX-Trigger", asciiJSON(body))
	return nil
}

// asciiJSON escapes every non-ASCII character in body as \uXXXX: browsers
// read header values as Latin-1, which garbles UTF-8 such as curly quotes.
func asciiJSON(body []byte) string {
	var out strings.Builder
	for _, r := range string(body) {
		switch {
		case r < 0x80:
			out.WriteRune(r)
		case r > 0xFFFF:
			hi, lo := utf16.EncodeRune(r)
			fmt.Fprintf(&out, "\\u%04x\\u%04x", hi, lo)
		default:
			fmt.Fprintf(&out, "\\u%04x", r)
		}
	}
	return out.String()
}

func renameMessage(from string, result packing.CategoryRename) string {
	message := "Renamed “" + from + "” to “" + result.Category + "”"
	if result.Items == 0 {
		return message + "."
	}
	return message + " on " + countNoun(result.Items, "item") + " in " + countNoun(len(result.Lists), "list") + "."
}

func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// listInView returns the id of the packing list page an htmx request came
// from, or "" for any other page.
func listInView(r *http.Request) string {
	current, err := url.Parse(r.Header.Get("HX-Current-URL"))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(current.Path, "/"), "/")
	if len(parts) == 2 && (parts[0] == "packing-lists" || parts[0] == "packing-list") && parts[1] != "new" {
		return parts[1]
	}
	return ""
}
