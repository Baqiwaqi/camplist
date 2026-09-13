package web

import (
	"context"
	"log"

	"camplist/internal/packing"
)

// categorySuggestions lists what a category picker offers: the defaults, the
// categories on the items in view, then the ones the signed-in camper has
// remembered. When the remembered ones cannot be read the picker still offers
// the rest.
func (h *handler) categorySuggestions(ctx context.Context, userID string, items []packing.PackingItem) []string {
	remembered, err := h.packingStore.RememberedCategories(ctx, userID)
	if err != nil {
		log.Printf("read remembered categories: %v", err)
	}
	return packing.CategorySuggestions(packing.ItemCategories(items), remembered)
}

// rememberCategory keeps a category the camper just saved so their other lists
// offer it too. The item is already stored, so a failure is only logged.
func (h *handler) rememberCategory(ctx context.Context, userID, category string) {
	if err := h.packingStore.RememberCategory(ctx, userID, category); err != nil {
		log.Printf("remember category: %v", err)
	}
}
