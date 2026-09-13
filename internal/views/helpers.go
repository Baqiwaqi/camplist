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

// progressCount is the "3 of 5 items packed" text of the progress count and status.
func progressCount(checked, total int) string {
	return fmt.Sprintf("%d of %d items packed", checked, total)
}

// progressWidth is the inline width of the progress fill.
func progressWidth(checked, total int) string {
	if total == 0 {
		return "width: 0%"
	}
	return fmt.Sprintf("width: %d%%", checked*100/total)
}

// csrfHeaders sets the CSRF token as an htmx request header on Layout's body.
// A header rather than a form field, because Go ignores DELETE bodies.
func csrfHeaders(csrfToken string) templ.Attributes {
	headers, _ := json.Marshal(map[string]string{"X-CSRF-Token": csrfToken})
	return templ.Attributes{"hx-headers": string(headers)}
}

// confirmDelete builds the htmx attributes for a delete action. The CSRF
// header comes from Layout's body; the layout's dialog reads the confirm data
// attributes.
func confirmDelete(url, question, action, detail string) templ.Attributes {
	return templ.Attributes{
		"hx-delete":           url,
		"hx-confirm":          question,
		"data-confirm-action": action,
		"data-confirm-detail": detail,
	}
}

// PackingListCardID is the element id of a list's card on the lists page.
func PackingListCardID(listID string) string {
	return "list-" + listID
}

// confirmDeleteCard deletes in place: htmx removes the card with the given
// element id once the request succeeds, and sends that id as HX-Target so the
// handler answers with a fragment instead of a redirect. The card dims while
// the request runs, and hx-sync drops a repeated delete.
func confirmDeleteCard(url, cardID, question, action, detail string) templ.Attributes {
	attrs := confirmDelete(url, question, action, detail)
	attrs["hx-target"] = "#" + cardID
	attrs["hx-swap"] = "delete"
	attrs["hx-indicator"] = "#" + cardID
	attrs["hx-sync"] = "this:drop"
	return attrs
}

// tripCardAction posts an action from a trip card and removes the card, for
// actions that move the trip between the overview and the archive.
func tripCardAction(url string) templ.Attributes {
	return removesTripCard(templ.Attributes{
		"hx-post": url,
	})
}

// deleteTripAttrs deletes a trip from its card. On the archive page the
// request says so, so the response reveals that page's empty state.
func deleteTripAttrs(tripID string, archived bool) templ.Attributes {
	url := "/trips/" + tripID
	if archived {
		url += "?view=archive"
	}
	return removesTripCard(confirmDelete(url, "Delete this trip?", "Delete trip", "Packing progress for this trip is lost. The list itself stays."))
}

// removesTripCard swaps away only the action's own card, so the rest of the
// page keeps its state, and drops a second action on that card while one is
// in flight. static/trip-cards.js moves focus off the removed card.
func removesTripCard(attrs templ.Attributes) templ.Attributes {
	attrs["hx-target"] = "closest [data-saved-trip]"
	attrs["hx-swap"] = "delete"
	attrs["hx-sync"] = "closest [data-saved-trip]:drop"
	return attrs
}

// removeTaskAttrs builds the htmx attributes that remove a preparation task
// through the existing edit endpoint, with the confirm dialog texts.
func removeTaskAttrs(list packing.PackingList, task packing.PreparationTask) templ.Attributes {
	vals, _ := json.Marshal(map[string]string{"revision": list.Revision(), "taskId": task.ID, "action": "remove"})
	return templ.Attributes{
		"hx-post":             "/packing-lists/" + list.ID + "/preparation/edit",
		"hx-vals":             string(vals),
		"hx-confirm":          "Remove this preparation task?",
		"data-confirm-action": "Remove task",
		"data-confirm-detail": "This removes the task from the reusable list.",
	}
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

type preparationGroup struct {
	Name  string
	Tasks []packing.PreparationTask
}

func groupPreparation(tasks []packing.PreparationTask) []preparationGroup {
	var groups []preparationGroup
	indexes := map[string]int{}
	for _, task := range tasks {
		key := task.Assignee
		name := task.AssigneeName
		if key == "" {
			name = "Shared"
		}
		i, ok := indexes[key]
		if !ok {
			i = len(groups)
			indexes[key] = i
			groups = append(groups, preparationGroup{Name: name})
		}
		groups[i].Tasks = append(groups[i].Tasks, task)
	}
	return groups
}

// oobSwap marks a fragment's element for htmx's out-of-band swap by id.
func oobSwap(oob bool) templ.Attributes {
	if !oob {
		return nil
	}
	return templ.Attributes{"hx-swap-oob": "true"}
}

// tripEntryRequest posts a trip entry without replacing the form. The
// response's checklist and preparation sections swap by id.
func tripEntryRequest(tripID string) templ.Attributes {
	return templ.Attributes{
		"hx-post":       "/trips/" + tripID + "/entries",
		"hx-swap":       "none",
		"hx-select-oob": "#packing-checklist,#session-preparation",
		"hx-sync":       "this:drop",
	}
}
