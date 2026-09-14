package views

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func hasClass(n *html.Node, want string) bool {
	value, _ := attr(n, "class")
	return slicesContains(strings.Fields(value), want)
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func byClass(class string) func(*html.Node) bool {
	return func(n *html.Node) bool { return hasClass(n, class) }
}

func byTag(tag string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Data == tag }
}

// A page head is the way back, the title and one meta line, with the page's
// actions beside them.
func TestPageHeadRendersBackLinkTitleMetaAndActions(t *testing.T) {
	doc := renderDoc(t, PageHead(PageHeading{
		Title:     "Trip archive",
		Meta:      "Trips you archive yourself can be restored.",
		BackHref:  "/trips",
		BackLabel: "Trips",
	}))

	back := findAll(doc, byClass("back-link"))
	if len(back) != 1 {
		t.Fatalf("back links %d, want 1", len(back))
	}
	if href, _ := attr(back[0], "href"); href != "/trips" {
		t.Errorf("back link href %q, want /trips", href)
	}
	if got := text(back[0]); got != "Trips" {
		t.Errorf("back link text %q, want Trips", got)
	}
	if heads := findAll(doc, byTag("h1")); len(heads) != 1 || text(heads[0]) != "Trip archive" {
		t.Errorf("h1 %v, want one reading Trip archive", heads)
	}
	if meta := findAll(doc, byClass("page-meta")); len(meta) != 1 {
		t.Fatalf("meta lines %d, want 1", len(meta))
	}
	if len(findAll(doc, byClass("page-head-actions"))) != 1 {
		t.Error("page head has no actions slot")
	}
}

// A top-level page has no way back beyond the nav, and no meta line when it
// has nothing to say.
func TestPageHeadLeavesOutTheBackLinkAndMetaWhenUnset(t *testing.T) {
	doc := renderDoc(t, PageHead(PageHeading{Title: "Trips in progress"}))
	if got := findAll(doc, byClass("back-link")); len(got) != 0 {
		t.Errorf("back links %d, want 0", len(got))
	}
	if got := findAll(doc, byClass("page-meta")); len(got) != 0 {
		t.Errorf("meta lines %d, want 0", len(got))
	}
}

func TestSectionHeadPairsTheTitleWithItsCount(t *testing.T) {
	doc := renderDoc(t, SectionHead("Gear", "16 items"))
	if heads := findAll(doc, byTag("h2")); len(heads) != 1 || text(heads[0]) != "Gear" {
		t.Fatalf("h2 %v, want one reading Gear", heads)
	}
	count := findAll(doc, byClass("section-count"))
	if len(count) != 1 || text(count[0]) != "16 items" {
		t.Fatalf("section count %v, want one reading 16 items", count)
	}
	if got := findAll(renderDoc(t, SectionHead("Gear", "")), byClass("section-count")); len(got) != 0 {
		t.Errorf("section counts %d without one, want 0", len(got))
	}
}

// One field component covers inputs and textareas: the label names the
// control, the hint describes it, and an invalid field says so.
func TestFieldLabelsTheControlAndDescribesItWithTheHint(t *testing.T) {
	doc := renderDoc(t, Field(TextField{
		ID:      "name",
		Name:    "name",
		Label:   "Name",
		Value:   "Tarp",
		Hint:    "The name campers see.",
		Invalid: true,
	}))

	inputs := findAll(doc, byTag("input"))
	if len(inputs) != 1 {
		t.Fatalf("inputs %d, want 1", len(inputs))
	}
	if !hasClass(inputs[0], "field") {
		t.Error("the control does not carry the shared field class")
	}
	if got, _ := attr(inputs[0], "aria-invalid"); got != "true" {
		t.Errorf("aria-invalid %q, want true", got)
	}
	if got, _ := attr(inputs[0], "aria-describedby"); got != "name-hint" {
		t.Errorf("aria-describedby %q, want name-hint", got)
	}
	labels := findAll(doc, byTag("label"))
	if len(labels) != 1 {
		t.Fatalf("labels %d, want 1", len(labels))
	}
	if got, _ := attr(labels[0], "for"); got != "name" {
		t.Errorf("label for %q, want name", got)
	}
	hints := findAll(doc, hasAttr("id", "name-hint"))
	if len(hints) != 1 || text(hints[0]) != "The name campers see." {
		t.Fatalf("hint %v, want one reading the hint", hints)
	}
}

func TestFieldRendersATextareaWithItsValueInside(t *testing.T) {
	doc := renderDoc(t, Field(TextField{ID: "note", Name: "note", Label: "Notes", Value: "Bring a spare", Multiline: true}))
	areas := findAll(doc, byTag("textarea"))
	if len(areas) != 1 {
		t.Fatalf("textareas %d, want 1", len(areas))
	}
	if text(areas[0]) != "Bring a spare" {
		t.Errorf("textarea text %q, want Bring a spare", text(areas[0]))
	}
	if hasAttrKey(areas[0], "aria-describedby") {
		t.Error("a field without a hint must not describe itself")
	}
}

// A switch is a checkbox: it posts like one without scripts and reads like
// one to a screen reader. The pill beside it is decoration.
func TestSwitchKeepsTheCheckboxAsTheSubmittedField(t *testing.T) {
	doc := renderDoc(t, Switch("saveForFuture", "Also save for future trips", true, nil))
	inputs := findAll(doc, byTag("input"))
	if len(inputs) != 1 {
		t.Fatalf("inputs %d, want 1", len(inputs))
	}
	if got, _ := attr(inputs[0], "type"); got != "checkbox" {
		t.Errorf("type %q, want checkbox", got)
	}
	if !hasAttrKey(inputs[0], "checked") {
		t.Error("a switch that is on must render checked")
	}
	if got, _ := attr(inputs[0], "name"); got != "saveForFuture" {
		t.Errorf("name %q, want saveForFuture", got)
	}
	pills := findAll(doc, byClass("switch"))
	if len(pills) != 1 {
		t.Fatalf("switch pills %d, want 1", len(pills))
	}
	if got, _ := attr(pills[0], "aria-hidden"); got != "true" {
		t.Error("the switch pill must stay out of the accessible name")
	}
	if got := text(doc); got != "Also save for future trips" {
		t.Errorf("switch text %q, want the label alone", got)
	}
}

func TestChipIsACheckbox(t *testing.T) {
	inputs := findAll(renderDoc(t, Chip("forgotten", "true", "Forgotten", true)), byTag("input"))
	if len(inputs) != 1 {
		t.Fatalf("inputs %d, want 1", len(inputs))
	}
	if got, _ := attr(inputs[0], "type"); got != "checkbox" {
		t.Errorf("type %q, want checkbox", got)
	}
	if !hasAttrKey(inputs[0], "checked") {
		t.Error("a chosen chip must render checked")
	}
	unchosen := findAll(renderDoc(t, Chip("unused", "true", "Unused", false)), byTag("input"))
	if hasAttrKey(unchosen[0], "checked") {
		t.Error("an unchosen chip must not render checked")
	}
}

// The link variant switches pages, so it must work as plain links and mark
// the page being shown.
func TestTabLinksMarkTheCurrentPage(t *testing.T) {
	doc := renderDoc(t, TabLinks("Trips", []Tab{
		{Label: "In progress", Count: "2", Href: "/trips", Current: true},
		{Label: "Archive", Count: "1", Href: "/trips/archive"},
	}))
	tabs := findAll(doc, byClass("tab"))
	if len(tabs) != 2 {
		t.Fatalf("tabs %d, want 2", len(tabs))
	}
	for _, tab := range tabs {
		if tab.Data != "a" {
			t.Errorf("tab is a %q, want a link", tab.Data)
		}
	}
	if got, _ := attr(tabs[0], "aria-current"); got != "page" {
		t.Errorf("current tab aria-current %q, want page", got)
	}
	if hasAttrKey(tabs[1], "aria-current") {
		t.Error("only the current tab carries aria-current")
	}
	if counts := findAll(doc, byClass("tab-count")); len(counts) != 2 {
		t.Errorf("tab counts %d, want 2", len(counts))
	}
}

// Each tab points at its panel and each panel back at its tab, and the strip
// stays hidden until Alpine runs so a page without scripts keeps every panel.
func TestTabGroupPairsEachTabWithItsPanel(t *testing.T) {
	doc := renderDoc(t, TabGroup("trip", "Trip", []Tab{{Label: "Packing", Count: "6/16"}, {Label: "Before you go"}}))

	strips := findAll(doc, hasAttr("role", "tablist"))
	if len(strips) != 1 {
		t.Fatalf("tablists %d, want 1", len(strips))
	}
	if !hasAttrKey(strips[0], "x-cloak") {
		t.Error("the strip must stay hidden until the script that drives it runs")
	}
	tabs := findAll(doc, hasAttr("role", "tab"))
	if len(tabs) != 2 {
		t.Fatalf("tabs %d, want 2", len(tabs))
	}
	for i, tab := range tabs {
		id, _ := attr(tab, "id")
		controls, _ := attr(tab, "aria-controls")
		if id != tabID("trip", i) || controls != tabPanelID("trip", i) {
			t.Errorf("tab %d is %q controlling %q", i, id, controls)
		}
	}
	if got, _ := attr(tabs[0], "aria-selected"); got != "true" {
		t.Errorf("first tab aria-selected %q, want true", got)
	}
	if got, _ := attr(tabs[1], "aria-selected"); got != "false" {
		t.Errorf("second tab aria-selected %q, want false", got)
	}
}

func TestTabPanelNamesItselfAfterItsTab(t *testing.T) {
	doc := renderDoc(t, TabPanel("trip", 1))
	panels := findAll(doc, hasAttr("role", "tabpanel"))
	if len(panels) != 1 {
		t.Fatalf("panels %d, want 1", len(panels))
	}
	id, _ := attr(panels[0], "id")
	labelledby, _ := attr(panels[0], "aria-labelledby")
	if id != tabPanelID("trip", 1) || labelledby != tabID("trip", 1) {
		t.Errorf("panel %q labelled by %q", id, labelledby)
	}
}

// The confirm dialog is the centred variant of the same modal the bulk add
// and category sheets use.
func TestDialogSheetOnlyWhenAsked(t *testing.T) {
	centred := findAll(renderDoc(t, Dialog("confirm-dialog", false, nil)), byTag("dialog"))
	if len(centred) != 1 {
		t.Fatalf("dialogs %d, want 1", len(centred))
	}
	if !hasClass(centred[0], "dialog") || hasClass(centred[0], "dialog-sheet") {
		class, _ := attr(centred[0], "class")
		t.Errorf("centred dialog class %q", class)
	}
	sheet := findAll(renderDoc(t, Dialog("bulk-add", true, nil)), byTag("dialog"))
	if !hasClass(sheet[0], "dialog-sheet") {
		t.Error("a sheet dialog must carry dialog-sheet")
	}
}

// The failure toast keeps the ids the offline scripts fill in by name.
func TestRequestErrorToastIsTheRedToastOfTheStack(t *testing.T) {
	doc := renderDoc(t, RequestErrorToast())
	if len(findAll(doc, byClass("toast-stack"))) != 1 {
		t.Fatal("the toast is not in a stack")
	}
	toasts := findAll(doc, hasAttr("id", "request-error"))
	if len(toasts) != 1 {
		t.Fatalf("toasts %d, want 1", len(toasts))
	}
	if !hasClass(toasts[0], "toast") || !hasClass(toasts[0], "toast-danger") {
		class, _ := attr(toasts[0], "class")
		t.Errorf("toast class %q, want the red toast", class)
	}
	if !hasAttrKey(toasts[0], "hidden") {
		t.Error("the toast must start hidden")
	}
	if got, _ := attr(toasts[0], "role"); got != "alert" {
		t.Errorf("role %q, want alert", got)
	}
	if len(findAll(doc, hasAttr("id", "request-error-message"))) != 1 {
		t.Error("the message element the offline scripts fill in is missing")
	}
}
