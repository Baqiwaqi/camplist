package packing

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxListEntries bounds a list's items plus preparation tasks. Starting a
	// trip applies its own limit to the expanded entries, which person-scoped
	// entries multiply by the number of campers (see createPackingSession).
	MaxListEntries = 2000
	// MaxItemNameLength matches the limit on trip entries.
	MaxItemNameLength = 200
	// MaxCategoryLength matches the limit on trip entries and remembered categories.
	MaxCategoryLength = maxCategoryLength
	// MaxItemsPerAdd bounds one Add several paste. Add from a list copies are
	// bounded only by MaxListEntries.
	MaxItemsPerAdd = 100
)

// ErrListFull reports an add that would take a list past MaxListEntries.
var ErrListFull = errors.New("the list has reached its item limit")

// ErrItemsGone reports a copy whose selected items are no longer on the source.
var ErrItemsGone = fmt.Errorf("the selected items are no longer on the source list: %w", ErrInvalid)

// AddItemsResult reports what a bulk add wrote. List is the saved list, or the
// current one when nothing was added. Replaced is the revision the write
// replaced, so a page showing an older revision knows it is out of date.
type AddItemsResult struct {
	List     PackingList
	Replaced string
	Added    []PackingItem
	Skipped  []string
}

// itemKey is how item names compare when skipping duplicates.
func itemKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func validNewItem(item PackingItem) bool {
	name := strings.TrimSpace(item.Name)
	return name != "" && len(name) <= MaxItemNameLength && len(strings.TrimSpace(item.Category)) <= MaxCategoryLength && validScope(item.Scope, false)
}

func listFull(list PackingList, adding int) bool {
	return len(list.Items)+len(list.Tasks)+adding > MaxListEntries
}

// AddItems appends items to a list the actor can edit. Names already on the
// list, or repeated within items, are skipped (trimmed, case-insensitive) and
// reported, so resubmitting the same add writes nothing. A category adopts the
// spelling of a matching category already on the list, else MatchCategory's.
// Invalid names, categories or scopes, more than MaxItemsPerAdd items, or a
// list that would exceed MaxListEntries reject the whole add.
func (s *Store) AddItems(ctx context.Context, listID, actor string, items []PackingItem) (AddItemsResult, error) {
	if len(items) > MaxItemsPerAdd {
		return AddItemsResult{}, ErrInvalid
	}
	return s.addItems(ctx, listID, actor, items)
}

func (s *Store) addItems(ctx context.Context, listID, actor string, items []PackingItem) (AddItemsResult, error) {
	if len(items) == 0 {
		return AddItemsResult{}, ErrInvalid
	}
	for _, item := range items {
		if !validNewItem(item) {
			return AddItemsResult{}, ErrInvalid
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		list, err := s.GetPackingList(ctx, listID, actor)
		if err != nil {
			return AddItemsResult{}, err
		}
		result := AddItemsResult{Replaced: list.Revision(), Added: []PackingItem{}, Skipped: []string{}}
		seen := map[string]bool{}
		categories := map[string]string{}
		for _, existing := range list.Items {
			seen[itemKey(existing.Name)] = true
			if key := categoryKey(existing.Category); key != "" {
				if _, ok := categories[key]; !ok {
					categories[key] = strings.TrimSpace(existing.Category)
				}
			}
		}
		now := time.Now().UTC()
		for _, item := range items {
			name := strings.TrimSpace(item.Name)
			if seen[itemKey(name)] {
				result.Skipped = append(result.Skipped, name)
				continue
			}
			seen[itemKey(name)] = true
			category := MatchCategory(item.Category)
			if spelled, ok := categories[categoryKey(category)]; ok {
				category = spelled
			} else if category != "" {
				categories[categoryKey(category)] = category
			}
			result.Added = append(result.Added, PackingItem{
				ID:        uuid.NewString(),
				Name:      name,
				Category:  category,
				Scope:     item.Scope,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		if len(result.Added) == 0 {
			result.List = list
			return result, nil
		}
		if listFull(list, len(result.Added)) {
			return AddItemsResult{}, ErrListFull
		}
		list.Items = append(list.Items, result.Added...)
		etag, err := s.saveList(ctx, list)
		if preconditionFailed(err) || errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return AddItemsResult{}, err
		}
		list.etag = etag
		for i := range list.Items {
			list.Items[i].SourceRevision = etag
		}
		result.List = list
		return result, nil
	}
	return AddItemsResult{}, ErrConflict
}

// CopyItems copies the selected items of a source list the actor can read into
// a destination list the actor can edit, as new items with fresh IDs. The
// source is re-read here, so only its current items are copied, in its order;
// IDs no longer on the source are ignored. Later edits to either list do not
// reach the other. Preparation tasks are not copied. A copy is not bounded by
// MaxItemsPerAdd, only by MaxListEntries; ErrItemsGone reports a selection
// with none of its items left on the source.
func (s *Store) CopyItems(ctx context.Context, destinationID, sourceID, actor string, itemIDs []string) (AddItemsResult, error) {
	if sourceID == destinationID {
		return AddItemsResult{}, ErrInvalid
	}
	source, err := s.GetPackingList(ctx, sourceID, actor)
	if err != nil {
		return AddItemsResult{}, err
	}
	selected := map[string]bool{}
	for _, id := range itemIDs {
		selected[id] = true
	}
	var items []PackingItem
	for _, item := range source.Items {
		if selected[item.ID] {
			items = append(items, PackingItem{Name: item.Name, Category: item.Category, Scope: item.Scope})
		}
	}
	if len(items) == 0 {
		return AddItemsResult{}, ErrItemsGone
	}
	return s.addItems(ctx, destinationID, actor, items)
}

// ItemLine is one item read from pasted text.
type ItemLine struct {
	Line     int
	Name     string
	Category string
}

var (
	bullet = regexp.MustCompile(`^[-*]\s+`)
	wordy  = regexp.MustCompile(`[\p{L}\p{N}]`)
)

// ParseItemLines reads one item per line from pasted text. A simple leading
// bullet ("- " or "* ") is stripped; any other marker stays in the name, so
// "2023. Trip notes" is kept whole. A line ending in a single ":" (such as
// "Documents:") starts a category for the lines below it; other lines keep an
// empty Category. Colons inside a name ("Map 1:50,000") stay in the name.
// Blank lines and lines without letters or digits ("---") are skipped.
func ParseItemLines(text string) []ItemLine {
	var lines []ItemLine
	category := ""
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if loc := bullet.FindStringIndex(line); loc != nil {
			line = strings.TrimSpace(line[loc[1]:])
		}
		if heading, ok := strings.CutSuffix(line, ":"); ok && !strings.Contains(heading, ":") {
			if heading = strings.TrimSpace(heading); wordy.MatchString(heading) {
				category = heading
			}
			continue
		}
		if !wordy.MatchString(line) {
			continue
		}
		lines = append(lines, ItemLine{Line: i + 1, Name: line, Category: category})
	}
	return lines
}
