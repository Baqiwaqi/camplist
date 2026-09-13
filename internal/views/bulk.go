package views

import (
	"fmt"
	"strconv"
	"strings"

	"camplist/internal/packing"

	"github.com/a-h/templ"
)

// AddFromList is the state of the Add from a list picker's second step: the
// destination list, the source list whose items can be copied, and what the
// camper ticked when the step comes back with an error or a result.
type AddFromList struct {
	List, Source packing.PackingList
	Selected     map[string]bool
	Errors       []string
	// Result is the summary of an add, shown when the page posts without scripts.
	Result string
}

// onList reports the names already on list, compared as AddItems compares them.
func onList(list packing.PackingList) map[string]bool {
	names := map[string]bool{}
	for _, item := range list.Items {
		names[strings.ToLower(strings.TrimSpace(item.Name))] = true
	}
	return names
}

func (a AddFromList) alreadyOnList(item packing.PackingItem) bool {
	return onList(a.List)[strings.ToLower(strings.TrimSpace(item.Name))]
}

// addable counts the source items not yet on the destination list.
func (a AddFromList) addable() int {
	names := onList(a.List)
	count := 0
	for _, item := range a.Source.Items {
		if !names[strings.ToLower(strings.TrimSpace(item.Name))] {
			count++
		}
	}
	return count
}

// ticked is whether an item's box starts ticked: every addable item on first
// view, the camper's own choice when the step comes back.
func (a AddFromList) ticked(item packing.PackingItem) bool {
	if a.alreadyOnList(item) {
		return false
	}
	if a.Selected == nil {
		return true
	}
	return a.Selected[item.ID]
}

func (a AddFromList) tickedCount() int {
	count := 0
	for _, item := range a.Source.Items {
		if a.ticked(item) {
			count++
		}
	}
	return count
}

// sourceSummary is the line under the source list's name, e.g. "5 items, 1
// already on this list".
func (a AddFromList) sourceSummary() string {
	summary := itemCount(len(a.Source.Items))
	if already := len(a.Source.Items) - a.addable(); already > 0 {
		summary += fmt.Sprintf(", %d already on this list", already)
	}
	return summary
}

func itemCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// addItemsLabel is the submit label for a count, e.g. "Add 3 items".
func addItemsLabel(n int) string {
	return "Add " + itemCount(n)
}

func addSeveralPath(listID string) string { return "/packing-lists/" + listID + "/add-several" }
func addFromListPath(listID string) string {
	return "/packing-lists/" + listID + "/add-from"
}
func addFromSourcePath(listID, sourceID string) string {
	return addFromListPath(listID) + "/" + sourceID
}

// bulkAddLoad loads a bulk add step into the list page's dialog.
func bulkAddLoad(path string) templ.Attributes {
	return templ.Attributes{"hx-get": path, "hx-target": "#bulk-add-panel", "hx-swap": "innerHTML"}
}

// bulkAddPost posts a bulk add step from the dialog. The answer is the step
// again when it needs another look, or nothing, which closes the dialog, with
// the new rows and summary out of band. hx-sync drops a repeat press.
func bulkAddPost(path string) templ.Attributes {
	return templ.Attributes{"hx-post": path, "hx-target": "#bulk-add-panel", "hx-swap": "innerHTML", "hx-sync": "this:drop"}
}

func itoa(n int) string { return strconv.Itoa(n) }

// groupName labels a category group; a list without categories has one
// unnamed group.
func groupName(group itemGroup) string {
	if group.Name == "" {
		return "Items"
	}
	return group.Name
}

// firstFilled is the index of the first list with items, which takes focus.
func firstFilled(lists []packing.PackingList) int {
	for i, list := range lists {
		if len(list.Items) > 0 {
			return i
		}
	}
	return -1
}

func groupAddable(a AddFromList, group itemGroup) bool {
	for _, item := range group.Items {
		if !a.alreadyOnList(item) {
			return true
		}
	}
	return false
}
