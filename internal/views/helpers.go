package views

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

// listSummary is the meta line of a list page, e.g. "16 items in 7
// categories". Items without a category are counted but name no category.
func listSummary(items []packing.PackingItem) string {
	summary := itemSummary(items)
	switch n := len(categories(items, 0)); n {
	case 0:
		return summary
	case 1:
		return summary + " in 1 category"
	default:
		return summary + " in " + strconv.Itoa(n) + " categories"
	}
}

// gearCount is the count beside the Gear section's title, e.g. "16 items".
func gearCount(items []packing.PackingItem) string {
	if len(items) == 1 {
		return "1 item"
	}
	return strconv.Itoa(len(items)) + " items"
}

// sharedLabel is the tag of a shared list. Only the owner sees who the members
// are, so the owner reads their name or number; everyone else reads "Shared".
func sharedLabel(list packing.PackingList, actor string) string {
	if !list.IsShared() {
		return ""
	}
	return sharingLabel(list.Sharing, list.UserID == actor)
}

// sharingLabel names who a shared list or trip is shared with, for its owner.
func sharingLabel(sharing packing.Sharing, owner bool) string {
	if owner {
		if n := len(sharing.Members); n > 1 {
			return "Shared with " + strconv.Itoa(n) + " people"
		}
		for _, member := range sharing.Members {
			if member.Name != "" {
				return "Shared with " + member.Name
			}
		}
	}
	return "Shared"
}

// categories returns the distinct categories in order of first use, at most
// limit, or all of them when limit is 0. Categories compare trimmed and
// case-insensitively, and the first spelling is shown.
func categories(items []packing.PackingItem, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		name := strings.TrimSpace(item.Category)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
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

// groupByCategory groups items by category in order of first use, for the
// list page's gear and the add-from-list picker. Items without a category come
// last and are labelled "Other" only when other groups exist. Categories
// compare trimmed and case-insensitively; a group shows its first spelling.
func groupByCategory(items []packing.PackingItem) []itemGroup {
	index := map[string]int{}
	var groups []itemGroup
	var other []packing.PackingItem
	for _, item := range items {
		name := strings.TrimSpace(item.Category)
		if name == "" {
			other = append(other, item)
			continue
		}
		key := strings.ToLower(name)
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, itemGroup{Name: name})
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

// startDateLayout matches the date the store appends to a new trip's name.
const startDateLayout = "Jan 2, 2006"

// tripCardTitle is the trip's name without the start date that default names
// end with, because the card's meta line already shows it. A name that is
// nothing but the date keeps it.
func tripCardTitle(s packing.PackingSession) string {
	name := s.DisplayName()
	title := strings.TrimSuffix(name, " – "+s.CreatedAt.Format(startDateLayout))
	if strings.TrimSpace(title) == "" {
		return name
	}
	return title
}

// tripStatus names where packing stands, above the card's progress bar.
func tripStatus(s packing.PackingSession) string {
	if allPacked(s.List.CountChecked(), len(s.List.Items)) {
		return "All packed"
	}
	return "Keep packing"
}

// tripMeta is the line under a trip's title, e.g. "Started Sep 13, 2026.
// Shared with Sam Rivera."
func tripMeta(s packing.PackingSession, actor string) string {
	meta := "Started " + s.CreatedAt.Format(startDateLayout) + "."
	if s.IsShared() {
		meta += " " + sharingLabel(s.Sharing, s.UserID == actor) + "."
	}
	return meta
}

// tripTabs are the trip page's tabs, each with its done count, e.g. "6/16".
// A trip without preparation tasks shows no count on Before you go.
func tripTabs(s packing.PackingSession) []Tab {
	return []Tab{
		{Label: "Packing", Count: packingTabCount(s), CountID: "trip-packing-count"},
		{Label: "Before you go", Count: tasksTabCount(s), CountID: "trip-tasks-count"},
	}
}

func packingTabCount(s packing.PackingSession) string {
	return fmt.Sprintf("%d/%d", s.List.CountChecked(), len(s.List.Items))
}

func tasksTabCount(s packing.PackingSession) string {
	if len(s.List.Tasks) == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", doneCount(s.List.Tasks), len(s.List.Tasks))
}

// groupCount is the count beside a checklist group's name, "2 of 5", or
// "All packed" (tasks: "All done") once every entry in it is.
func groupCount(done, total int, task bool) string {
	switch {
	case total == 0 || done < total:
		return fmt.Sprintf("%d of %d", done, total)
	case task:
		return "All done"
	}
	return "All packed"
}

func checkedCount(items []packing.PackingItem) int {
	done := 0
	for _, item := range items {
		if item.Checked {
			done++
		}
	}
	return done
}

// groupCountClass marks a finished group's count.
func groupCountClass(done, total int) string {
	if total > 0 && done == total {
		return "group-count group-done"
	}
	return "group-count"
}

// packCountID names a group count in the trip checklist, so a toggle can
// update it out of band: person i, or category j inside person i.
func packCountID(prefix string, ids ...int) string {
	id := prefix
	for _, i := range ids {
		id += "-" + strconv.Itoa(i)
	}
	return id
}

// changedByCaption names someone else's change under a checklist row, e.g.
// "Packed by Sam Rivera". Empty for the viewer's own changes.
func changedByCaption(changedBy, changedByID, actor string, checked, task bool) string {
	if changedBy == "" || changedByID == actor {
		return ""
	}
	switch {
	case task && checked:
		return "Done by " + changedBy
	case task:
		return "Marked not done by " + changedBy
	case checked:
		return "Packed by " + changedBy
	}
	return "Unpacked by " + changedBy
}

// personName names whose part of a shared trip a group is.
func personName(assignee, name, actor string) string {
	switch {
	case assignee == "":
		return "Shared"
	case assignee == actor:
		return "Yours"
	case name == "":
		return "Another camper"
	}
	return name
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

// tripPageAction posts an action from the trip page's More menu. The page has
// no card to remove, so from=trip makes the handler send the camper to the
// trips page instead.
func tripPageAction(tripID, action string) templ.Attributes {
	return templ.Attributes{
		"hx-post": "/trips/" + tripID + "/" + action + "?from=trip",
		"hx-swap": "none",
		"hx-sync": "this:drop",
	}
}

// tripPageDelete deletes the trip from its own page, like tripPageAction.
func tripPageDelete(tripID string) templ.Attributes {
	attrs := confirmDeleteTrip("/trips/" + tripID + "?from=trip")
	attrs["hx-swap"] = "none"
	attrs["hx-sync"] = "this:drop"
	return attrs
}

// deleteTripAttrs deletes a trip from its card. On the archive page the
// request says so, so the response reveals that page's empty state.
func deleteTripAttrs(tripID string, archived bool) templ.Attributes {
	url := "/trips/" + tripID
	if archived {
		url += "?view=archive"
	}
	return removesTripCard(confirmDeleteTrip(url))
}

func confirmDeleteTrip(url string) templ.Attributes {
	return confirmDelete(url, "Delete this trip?", "Delete trip", "Packing progress for this trip is lost. The list itself stays.")
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

// preparationSwap builds the htmx attributes shared by every control in the
// preparation card: post to the preparation endpoint and swap the card. hx-sync
// drops a repeat press while that control is saving.
func preparationSwap(list packing.PackingList) templ.Attributes {
	return templ.Attributes{
		"hx-post":   "/packing-lists/" + list.ID + "/preparation/edit",
		"hx-target": "#list-preparation",
		"hx-swap":   "outerHTML",
		"hx-sync":   "this:drop",
	}
}

// removeTaskAttrs builds the Remove button in a task's rename form. It posts
// that form with action=remove, names the task whose field takes focus once
// the card is swapped, and sets the confirm dialog texts.
func removeTaskAttrs(list packing.PackingList, next string) templ.Attributes {
	vals, _ := json.Marshal(map[string]string{"action": "remove", "next": next})
	attrs := preparationSwap(list)
	attrs["hx-vals"] = string(vals)
	attrs["hx-sync"] = "closest form:drop"
	attrs["hx-confirm"] = "Remove this preparation task?"
	attrs["data-confirm-action"] = "Remove task"
	attrs["data-confirm-detail"] = "This removes the task from the reusable list."
	return attrs
}

// nextTaskID is the task that takes focus when tasks[i] is removed: the one
// after it, else the one before it, else none.
func nextTaskID(tasks []packing.PreparationTask, i int) string {
	switch {
	case i+1 < len(tasks):
		return tasks[i+1].ID
	case i > 0:
		return tasks[i-1].ID
	}
	return ""
}

// preparationSummary counts done tasks, e.g. "1 of 3 done".
func preparationSummary(tasks []packing.PreparationTask) string {
	return fmt.Sprintf("%d of %d done", doneCount(tasks), len(tasks))
}

// groupByPerson splits a shared trip's items by who they are for: the shared
// items, the viewer's ("Yours") and each other person's, in order of first
// use. A trip that is private, or has no personal items, stays one unnamed
// group, whose personal items carry a Yours tag instead.
func groupByPerson(items []packing.PackingItem, shared bool, actor string) []itemGroup {
	if !splitByPerson(items, shared) {
		return []itemGroup{{Items: items}}
	}
	var groups []itemGroup
	index := map[string]int{}
	for _, item := range items {
		i, ok := index[item.Assignee]
		if !ok {
			i = len(groups)
			index[item.Assignee] = i
			groups = append(groups, itemGroup{Name: personName(item.Assignee, item.AssigneeName, actor)})
		}
		groups[i].Items = append(groups[i].Items, item)
	}
	return groups
}

// splitByPerson reports whether a trip's checklist groups its items by person.
func splitByPerson(items []packing.PackingItem, shared bool) bool {
	for _, item := range items {
		if shared && item.Assignee != "" {
			return true
		}
	}
	return false
}

type preparationGroup struct {
	Name  string
	Tasks []packing.PreparationTask
}

// groupPreparationByPerson splits a trip's preparation tasks the way
// groupByPerson splits its items.
func groupPreparationByPerson(tasks []packing.PreparationTask, shared bool, actor string) []preparationGroup {
	byPerson := false
	for _, task := range tasks {
		byPerson = byPerson || shared && task.Assignee != ""
	}
	if !byPerson {
		return []preparationGroup{{Tasks: tasks}}
	}
	var groups []preparationGroup
	index := map[string]int{}
	for _, task := range tasks {
		i, ok := index[task.Assignee]
		if !ok {
			i = len(groups)
			index[task.Assignee] = i
			groups = append(groups, preparationGroup{Name: personName(task.Assignee, task.AssigneeName, actor)})
		}
		groups[i].Tasks = append(groups[i].Tasks, task)
	}
	return groups
}

func doneCount(tasks []packing.PreparationTask) int {
	done := 0
	for _, task := range tasks {
		if task.Done {
			done++
		}
	}
	return done
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

// categoryFieldID names the rename field of the i-th category on the categories page.
func categoryFieldID(i int) string {
	return "category-rename-" + strconv.Itoa(i)
}

// tabID and tabPanelID name the tab and panel of one segment, so each tab
// points at its panel and each panel back at its tab.
func tabID(group string, index int) string {
	return group + "-tab-" + strconv.Itoa(index)
}

func tabPanelID(group string, index int) string {
	return group + "-panel-" + strconv.Itoa(index)
}

// tabIsCurrent is the Alpine expression that is true while a tab's panel is
// the one on show.
func tabIsCurrent(index int) string {
	return "current === " + strconv.Itoa(index)
}

// tabIndex renders a tab's position for an Alpine call.
func tabIndex(index int) string {
	return strconv.Itoa(index)
}

// boolAttr renders the server's first paint of an attribute Alpine then binds,
// so the markup is right before scripts run and without them.
func boolAttr(value bool) string {
	return strconv.FormatBool(value)
}

// confirmDialogAttrs holds the state of the layout's confirm dialog, which
// answers htmx:confirm for every element carrying hx-confirm.
func confirmDialogAttrs() templ.Attributes {
	return templ.Attributes{
		"x-data":                   "{ question: '', detail: '', action: 'Delete', issue: null }",
		"x-on:htmx:confirm.window": "if (!$event.detail.question) return; $event.preventDefault(); question = $event.detail.question; detail = $event.detail.elt.dataset.confirmDetail || ''; action = $event.detail.elt.dataset.confirmAction || 'Delete'; issue = $event.detail.issueRequest; $el.showModal()",
		"x-on:close":               "issue = null",
	}
}
