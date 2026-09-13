package packing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// DefaultCategories are offered on every item form, before any category the
// camper has typed themselves.
var DefaultCategories = []string{
	"Shelter",
	"Sleeping",
	"Kitchen and cooking",
	"Food and drink",
	"Clothing",
	"Hygiene and first aid",
	"Tools and repair",
	"Electronics and lighting",
	"Navigation and documents",
	"Other",
}

const (
	categoriesType = "item-categories"
	// categoriesID names the one remembered-categories document in each
	// user's partition.
	categoriesID      = "item-categories"
	maxCategoryLength = 100
	maxRemembered     = 200
)

// categoryKey is how categories compare: "cooking" and " Cooking" are the same.
func categoryKey(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

// CategorySuggestions merges the defaults with further categories, in order,
// trimmed and without case-insensitive duplicates. The first spelling wins,
// so a default keeps its own capitalisation.
func CategorySuggestions(more ...[]string) []string {
	return uniqueCategories(append([][]string{DefaultCategories}, more...)...)
}

func uniqueCategories(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range groups {
		for _, category := range group {
			category = strings.TrimSpace(category)
			if category == "" || len(category) > maxCategoryLength || seen[categoryKey(category)] {
				continue
			}
			seen[categoryKey(category)] = true
			out = append(out, category)
		}
	}
	return out
}

// MatchCategory trims a submitted category and adopts the spelling of the
// suggestion it matches, so typing "cooking" next to "Cooking" adds no second
// category.
func MatchCategory(category string, suggestions []string) string {
	category = strings.TrimSpace(category)
	for _, suggestion := range suggestions {
		if categoryKey(suggestion) == categoryKey(category) {
			return suggestion
		}
	}
	return category
}

// ItemCategories lists the categories used by items in order of first use.
func ItemCategories(items []PackingItem) []string {
	used := make([]string, 0, len(items))
	for _, item := range items {
		used = append(used, item.Category)
	}
	return uniqueCategories(used)
}

func isDefaultCategory(category string) bool {
	return slices.ContainsFunc(DefaultCategories, func(d string) bool { return categoryKey(d) == categoryKey(category) })
}

// rememberedCategories is the per-user document holding the custom categories
// a camper has typed, so every list they open can offer them again.
type rememberedCategories struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	Type       string    `json:"type"`
	Categories []string  `json:"categories"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (s *Store) readCategories(ctx context.Context, userID string) (rememberedCategories, string, error) {
	res, err := s.container.ReadItem(ctx, azcosmos.NewPartitionKeyString(userID), categoriesID, nil)
	if err != nil {
		return rememberedCategories{}, "", err
	}
	var doc rememberedCategories
	if err := json.Unmarshal(res.Value, &doc); err != nil {
		return rememberedCategories{}, "", fmt.Errorf("unmarshal categories: %w", err)
	}
	if doc.Type != categoriesType || doc.UserID != userID {
		return rememberedCategories{}, "", ErrNotFound
	}
	return doc, string(res.ETag), nil
}

// seedCategories collects the custom categories already on the lists the user
// owns. It stands in for the document until the first new category is saved.
func (s *Store) seedCategories(ctx context.Context, userID string) ([]string, error) {
	lists, err := s.GetPackingLists(ctx, userID)
	if err != nil {
		return nil, err
	}
	var used []PackingItem
	for _, list := range lists {
		if list.UserID == userID {
			used = append(used, list.Items...)
		}
	}
	return slices.DeleteFunc(ItemCategories(used), isDefaultCategory), nil
}

// RememberedCategories returns the custom categories userID has used.
func (s *Store) RememberedCategories(ctx context.Context, userID string) ([]string, error) {
	doc, _, err := s.readCategories(ctx, userID)
	if isMissing(err) {
		return s.seedCategories(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	return doc.Categories, nil
}

// RememberCategory records a custom category for userID. Defaults, blanks and
// categories already remembered in any capitalisation change nothing.
func (s *Store) RememberCategory(ctx context.Context, userID, category string) error {
	category = strings.TrimSpace(category)
	if userID == "" || category == "" || len(category) > maxCategoryLength || isDefaultCategory(category) {
		return nil
	}
	for attempt := 0; attempt < 5; attempt++ {
		doc, etag, err := s.readCategories(ctx, userID)
		if isMissing(err) {
			seeded, seedErr := s.seedCategories(ctx, userID)
			if seedErr != nil {
				return seedErr
			}
			doc = rememberedCategories{ID: categoriesID, UserID: userID, Type: categoriesType, Categories: seeded}
			etag = ""
		} else if err != nil {
			return err
		}
		if slices.ContainsFunc(doc.Categories, func(c string) bool { return categoryKey(c) == categoryKey(category) }) {
			if etag != "" {
				return nil
			}
		} else {
			doc.Categories = append(doc.Categories, category)
		}
		if extra := len(doc.Categories) - maxRemembered; extra > 0 {
			doc.Categories = doc.Categories[extra:]
		}
		doc.UpdatedAt = s.clock().UTC()
		body, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		pk := azcosmos.NewPartitionKeyString(userID)
		if etag == "" {
			_, err = s.container.CreateItem(ctx, pk, body, nil)
		} else {
			match := azcore.ETag(etag)
			_, err = s.container.ReplaceItem(ctx, pk, categoriesID, body, &azcosmos.ItemOptions{IfMatchEtag: &match})
		}
		if preconditionFailed(err) || conflicted(err) {
			continue
		}
		return err
	}
	return ErrConflict
}

func conflicted(err error) bool {
	var response *azcore.ResponseError
	return errors.As(err, &response) && response.StatusCode == 409
}
