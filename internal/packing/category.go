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

// CategoryOption is one category a picker offers. Custom marks a category the
// camper remembered, which the picker lets them rename or remove.
type CategoryOption struct {
	Name   string
	Custom bool
}

// CategoryOptions marks the suggestions that are among remembered.
func CategoryOptions(suggestions, remembered []string) []CategoryOption {
	options := make([]CategoryOption, 0, len(suggestions))
	for _, name := range suggestions {
		options = append(options, CategoryOption{Name: name, Custom: !IsDefaultCategory(name) && indexCategory(remembered, name) >= 0})
	}
	return options
}

// MatchCategory trims a submitted category and adopts the spelling of the
// default it matches, so typing "shelter" files under "Shelter". A custom
// category keeps the spelling the camper submitted.
func MatchCategory(category string) string {
	category = strings.TrimSpace(category)
	for _, def := range DefaultCategories {
		if categoryKey(def) == categoryKey(category) {
			return def
		}
	}
	return category
}

// CloseCategory returns the category in known that typed is most likely a
// typo of, or "" when there is none. typed must not already be one of known
// (apart from case and spaces) and must be at least four letters; a close
// category is at most one edit away (a letter added, dropped, changed, or two
// neighbours swapped), or two when both are eight letters or longer. The
// picker in static/category-picker.js applies the same rule while typing.
func CloseCategory(typed string, known []string) string {
	typedKey := []rune(categoryKey(typed))
	if len(typedKey) < 4 || indexCategory(known, typed) >= 0 {
		return ""
	}
	best, bestDistance := "", 3
	for _, category := range known {
		key := []rune(categoryKey(category))
		allowed := 1
		if len(typedKey) >= 8 && len(key) >= 8 {
			allowed = 2
		}
		if d := editDistance(typedKey, key); d <= allowed && d < bestDistance {
			best, bestDistance = strings.TrimSpace(category), d
		}
	}
	return best
}

// editDistance counts the single-letter insertions, deletions, substitutions
// and swaps of neighbouring letters that turn a into b.
func editDistance(a, b []rune) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}

// ItemCategories lists the categories used by items in order of first use.
func ItemCategories(items []PackingItem) []string {
	used := make([]string, 0, len(items))
	for _, item := range items {
		used = append(used, item.Category)
	}
	return uniqueCategories(used)
}

// IsDefaultCategory reports whether category is one of the defaults in any
// capitalisation.
func IsDefaultCategory(category string) bool {
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
// owns, to fill the document when it is first created.
func (s *Store) seedCategories(ctx context.Context, userID string) ([]string, error) {
	lists, err := s.ownPackingLists(ctx, userID)
	if err != nil {
		return nil, err
	}
	var used []PackingItem
	for _, list := range lists {
		used = append(used, list.Items...)
	}
	return slices.DeleteFunc(ItemCategories(used), IsDefaultCategory), nil
}

// loadCategories reads userID's document, creating it from their own lists the
// first time so later reads are a single point read.
func (s *Store) loadCategories(ctx context.Context, userID string) (rememberedCategories, string, error) {
	doc, etag, err := s.readCategories(ctx, userID)
	if !isMissing(err) {
		return doc, etag, err
	}
	seeded, err := s.seedCategories(ctx, userID)
	if err != nil {
		return rememberedCategories{}, "", err
	}
	body, err := json.Marshal(rememberedCategories{ID: categoriesID, UserID: userID, Type: categoriesType, Categories: seeded, UpdatedAt: s.clock().UTC()})
	if err != nil {
		return rememberedCategories{}, "", err
	}
	if _, err := s.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(userID), body, nil); err != nil && !conflicted(err) {
		return rememberedCategories{}, "", err
	}
	return s.readCategories(ctx, userID)
}

// RememberedCategories returns the custom categories userID has used.
func (s *Store) RememberedCategories(ctx context.Context, userID string) ([]string, error) {
	doc, _, err := s.loadCategories(ctx, userID)
	return doc.Categories, err
}

// RememberCategory records a custom category for userID. Defaults and blanks
// change nothing; a category already remembered in another capitalisation
// takes the spelling just submitted.
func (s *Store) RememberCategory(ctx context.Context, userID, category string) error {
	category = strings.TrimSpace(category)
	if userID == "" || category == "" || len(category) > maxCategoryLength || IsDefaultCategory(category) {
		return nil
	}
	return s.changeCategories(ctx, userID, func(categories []string) ([]string, bool) {
		switch i := indexCategory(categories, category); {
		case i < 0:
			categories = append(categories, category)
		case categories[i] == category:
			return categories, false
		default:
			categories[i] = category
		}
		if extra := len(categories) - maxRemembered; extra > 0 {
			categories = categories[extra:]
		}
		return categories, true
	})
}

// ForgetCategory drops a remembered custom category from userID's
// suggestions. Items keep their category text. Defaults cannot be forgotten;
// forgetting one that is not remembered changes nothing.
func (s *Store) ForgetCategory(ctx context.Context, userID, category string) error {
	category = strings.TrimSpace(category)
	if category == "" || IsDefaultCategory(category) {
		return ErrInvalid
	}
	return s.changeCategories(ctx, userID, func(categories []string) ([]string, bool) {
		i := indexCategory(categories, category)
		if i < 0 {
			return categories, false
		}
		return slices.Delete(categories, i, i+1), true
	})
}

// CategoryRename reports what RenameCategory changed.
type CategoryRename struct {
	// Category is the name the items carry now: the new name, or the spelling
	// of the default or remembered category it was merged into.
	Category string
	Items    int
	Lists    []RenamedList
}

// RenamedList is a list RenameCategory saved, with the revision it replaced.
type RenamedList struct {
	List     PackingList
	Replaced string
}

// RenameCategory renames a remembered custom category for userID and moves the
// items filed under it, in any capitalisation, on the lists userID owns. A new
// name that matches a default or another remembered category merges into it.
// Lists shared with userID by someone else and trips already started keep
// their own copy. Defaults cannot be renamed.
func (s *Store) RenameCategory(ctx context.Context, userID, from, to string) (CategoryRename, error) {
	from = strings.TrimSpace(from)
	to = MatchCategory(to)
	if from == "" || IsDefaultCategory(from) || to == "" || len(to) > maxCategoryLength {
		return CategoryRename{}, ErrInvalid
	}
	doc, _, err := s.loadCategories(ctx, userID)
	if err != nil {
		return CategoryRename{}, err
	}
	i := indexCategory(doc.Categories, from)
	if i < 0 {
		return CategoryRename{}, ErrNotFound
	}
	from = doc.Categories[i]
	if j := indexCategory(doc.Categories, to); j >= 0 && j != i {
		to = doc.Categories[j]
	}
	result := CategoryRename{Category: to}

	lists, err := s.ownPackingLists(ctx, userID)
	if err != nil {
		return CategoryRename{}, err
	}
	for _, list := range lists {
		if countCategory(list.Items, from, to) == 0 {
			continue
		}
		renamed, items, err := s.renameInList(ctx, list.ID, userID, from, to)
		if err != nil {
			return CategoryRename{}, err
		}
		if items > 0 {
			result.Items += items
			result.Lists = append(result.Lists, renamed)
		}
	}

	// The lists come first, so a failure leaves the old name to rename again.
	err = s.changeCategories(ctx, userID, func(categories []string) ([]string, bool) {
		i := indexCategory(categories, from)
		if i < 0 {
			return categories, false
		}
		if IsDefaultCategory(to) || slices.ContainsFunc(categories, func(c string) bool { return c != categories[i] && categoryKey(c) == categoryKey(to) }) {
			return slices.Delete(categories, i, i+1), true
		}
		categories[i] = to
		return categories, true
	})
	return result, err
}

// renameInList files the items under from as to on one of userID's lists,
// retrying when another save lands first.
func (s *Store) renameInList(ctx context.Context, listID, userID, from, to string) (RenamedList, int, error) {
	for attempt := 0; attempt < 5; attempt++ {
		list, err := s.GetPackingList(ctx, listID, userID)
		if isMissing(err) {
			return RenamedList{}, 0, nil
		}
		if err != nil {
			return RenamedList{}, 0, err
		}
		if list.UserID != userID {
			return RenamedList{}, 0, nil
		}
		changed := countCategory(list.Items, from, to)
		if changed == 0 {
			return RenamedList{}, 0, nil
		}
		now := s.clock().UTC()
		for i, item := range list.Items {
			if categoryKey(item.Category) == categoryKey(from) && item.Category != to {
				list.Items[i].Category = to
				list.Items[i].UpdatedAt = now
			}
		}
		replaced := list.Revision()
		etag, err := s.saveList(ctx, list)
		if preconditionFailed(err) || errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return RenamedList{}, 0, err
		}
		list.setRevision(etag)
		return RenamedList{List: list, Replaced: replaced}, changed, nil
	}
	return RenamedList{}, 0, ErrConflict
}

// countCategory counts the items filed under from that renaming to to changes.
func countCategory(items []PackingItem, from, to string) int {
	count := 0
	for _, item := range items {
		if categoryKey(item.Category) == categoryKey(from) && item.Category != to {
			count++
		}
	}
	return count
}

func indexCategory(categories []string, category string) int {
	return slices.IndexFunc(categories, func(c string) bool { return categoryKey(c) == categoryKey(category) })
}

// changeCategories applies change to userID's remembered categories and saves
// the result when change reports it changed something, retrying when another
// save lands first.
func (s *Store) changeCategories(ctx context.Context, userID string, change func([]string) ([]string, bool)) error {
	for attempt := 0; attempt < 5; attempt++ {
		doc, etag, err := s.loadCategories(ctx, userID)
		if err != nil {
			return err
		}
		var changed bool
		if doc.Categories, changed = change(doc.Categories); !changed {
			return nil
		}
		doc.UpdatedAt = s.clock().UTC()
		body, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		match := azcore.ETag(etag)
		_, err = s.container.ReplaceItem(ctx, azcosmos.NewPartitionKeyString(userID), categoriesID, body, &azcosmos.ItemOptions{IfMatchEtag: &match})
		if preconditionFailed(err) {
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
