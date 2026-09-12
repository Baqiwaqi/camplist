package views

import (
	"fmt"
	"strconv"

	"camplist/internal/packing"
)

// itemSummary describes how many items a list holds, e.g. "7 items".
func itemSummary(items []packing.PackingItem) string {
	switch len(items) {
	case 0:
		return "No items yet"
	case 1:
		return "1 item"
	default:
		return strconv.Itoa(len(items)) + " items"
	}
}

// categories returns the distinct categories in order of first use, at most limit.
func categories(items []packing.PackingItem, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if item.Category == "" || seen[item.Category] {
			continue
		}
		seen[item.Category] = true
		out = append(out, item.Category)
		if len(out) == limit {
			break
		}
	}
	return out
}

type itemGroup struct {
	Name  string
	Items []packing.PackingItem
}

// groupByCategory groups items by category in order of first use. Items without
// a category come last and are labelled "Other" only when other groups exist.
func groupByCategory(items []packing.PackingItem) []itemGroup {
	index := map[string]int{}
	var groups []itemGroup
	var other []packing.PackingItem
	for _, item := range items {
		if item.Category == "" {
			other = append(other, item)
			continue
		}
		i, ok := index[item.Category]
		if !ok {
			i = len(groups)
			index[item.Category] = i
			groups = append(groups, itemGroup{Name: item.Category})
		}
		groups[i].Items = append(groups[i].Items, item)
	}
	if len(other) > 0 {
		name := ""
		if len(groups) > 0 {
			name = "Other"
		}
		groups = append(groups, itemGroup{Name: name, Items: other})
	}
	return groups
}

// allPacked reports whether every item in a non-empty session is packed.
func allPacked(checked, total int) bool {
	return total > 0 && checked >= total
}

// progressWidth is the inline width of the progress fill.
func progressWidth(checked, total int) string {
	if total == 0 {
		return "width: 0%"
	}
	return fmt.Sprintf("width: %d%%", checked*100/total)
}

func groupByParticipant(items []packing.PackingItem) []itemGroup {
	var groups []itemGroup
	indexes := map[string]int{}
	for _, item := range items {
		key := item.Assignee
		name := item.AssigneeName
		if key == "" {
			name = "Shared"
		}
		i, ok := indexes[key]
		if !ok {
			i = len(groups)
			indexes[key] = i
			groups = append(groups, itemGroup{Name: name})
		}
		groups[i].Items = append(groups[i].Items, item)
	}
	return groups
}
