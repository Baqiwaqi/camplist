package views

import (
	"encoding/json"
	"fmt"
	"strconv"

	"camplist/internal/packing"

	"github.com/a-h/templ"
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

// confirmDelete builds the htmx attributes for a delete action: the CSRF
// token travels in a header because Go ignores DELETE bodies, and the
// layout's dialog reads the confirm data attributes.
func confirmDelete(url, csrfToken, question, action, detail string) templ.Attributes {
	return templ.Attributes{
		"hx-delete":           url,
		"hx-headers":          `{"X-CSRF-Token":"` + csrfToken + `"}`,
		"hx-confirm":          question,
		"data-confirm-action": action,
		"data-confirm-detail": detail,
	}
}

// removeTaskAttrs builds the htmx attributes that remove a preparation task
// through the existing edit endpoint, with the confirm dialog texts.
func removeTaskAttrs(list packing.PackingList, task packing.PreparationTask, csrfToken string) templ.Attributes {
	vals, _ := json.Marshal(map[string]string{"_csrf": csrfToken, "revision": list.Revision(), "taskId": task.ID, "action": "remove"})
	return templ.Attributes{
		"hx-post":             "/packing-list/" + list.ID + "/preparation/edit",
		"hx-vals":             string(vals),
		"hx-confirm":          "Remove this preparation task?",
		"data-confirm-action": "Remove task",
		"data-confirm-detail": "This removes the task from the reusable list.",
	}
}
