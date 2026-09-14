package views

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"camplist/internal/packing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	if n.Type == html.ElementNode && match(n) {
		out = append(out, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, findAll(c, match)...)
	}
	return out
}

func text(n *html.Node) string {
	var b strings.Builder
	for _, t := range findAllText(n) {
		b.WriteString(t)
	}
	return strings.TrimSpace(b.String())
}

func findAllText(n *html.Node) []string {
	if n.Type == html.TextNode {
		return []string{n.Data}
	}
	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, findAllText(c)...)
	}
	return out
}

// The native select is the submitted field and the no-JS fallback; the
// combobox and listbox must point at each other and at the label.
func TestScopeSelectPairsNativeFieldWithListbox(t *testing.T) {
	doc := renderDoc(t, ScopeSelect("scope", "item-scope-new", "person", false, false))

	native := findElement(doc, hasAttr("id", "item-scope-new"))
	if native == nil || native.Data != "select" {
		t.Fatal("scope select has no native select to submit")
	}
	if v, _ := attr(native, "name"); v != "scope" {
		t.Errorf("native select posts as %q, want scope", v)
	}
	if v, _ := attr(native, "class"); !strings.Contains(v, "select-native") {
		t.Error("native select is not hidden behind the listbox")
	}
	var values, selected []string
	for _, option := range findAll(native, func(n *html.Node) bool { return n.Data == "option" }) {
		value, _ := attr(option, "value")
		values = append(values, value)
		if _, ok := attr(option, "selected"); ok {
			selected = append(selected, value)
		}
	}
	if strings.Join(values, ",") != "shared,person" {
		t.Errorf("list scope options = %v, want shared and person only", values)
	}
	if strings.Join(selected, ",") != "person" {
		t.Errorf("selected options = %v, want person", selected)
	}

	label := findElement(doc, hasAttr("id", "item-scope-new-label"))
	if label == nil || label.Data != "label" {
		t.Fatal("scope select has no label")
	}
	if v, _ := attr(label, "for"); v != "item-scope-new" {
		t.Errorf("label points at %q", v)
	}
	if v, _ := attr(label, "class"); strings.Contains(v, "sr-only") {
		t.Error("visible label is hidden")
	}

	trigger := findElement(doc, hasAttr("role", "combobox"))
	if trigger == nil || trigger.Data != "button" {
		t.Fatal("scope select has no combobox button")
	}
	for key, want := range map[string]string{
		"type":            "button",
		"aria-haspopup":   "listbox",
		"aria-expanded":   "false",
		"aria-labelledby": "item-scope-new-label",
		"aria-controls":   "item-scope-new-listbox",
	} {
		if v, _ := attr(trigger, key); v != want {
			t.Errorf("combobox %s = %q, want %q", key, v, want)
		}
	}
	if got := text(trigger); got != "For each person" {
		t.Errorf("combobox shows %q before scripts run, want the selected label", got)
	}

	listbox := findElement(doc, hasAttr("id", "item-scope-new-listbox"))
	if listbox == nil {
		t.Fatal("combobox controls a missing listbox")
	}
	if v, _ := attr(listbox, "role"); v != "listbox" {
		t.Errorf("popup role = %q, want listbox", v)
	}
	if _, ok := attr(listbox, "hidden"); !ok {
		t.Error("listbox is not hidden until opened")
	}
	options := findAll(listbox, hasAttr("role", "option"))
	if len(options) != len(values) {
		t.Fatalf("listbox has %d options, native select %d", len(options), len(values))
	}
	if v, _ := attr(options[1], "aria-selected"); v != "true" {
		t.Error("selected option is not marked aria-selected")
	}
	if v, _ := attr(options[1], "id"); v != "item-scope-new-option-1" {
		t.Errorf("option id = %q, which aria-activedescendant cannot reach", v)
	}
}

func TestScopeSelectDefaultsAndCompactLabel(t *testing.T) {
	doc := renderDoc(t, ScopeSelect("scope", "task-scope-1", "", true, true))
	selected := findElement(doc, func(n *html.Node) bool {
		_, ok := attr(n, "selected")
		return n.Data == "option" && ok
	})
	if selected == nil {
		t.Fatal("no option selected for an unset scope")
	}
	if v, _ := attr(selected, "value"); v != "shared" {
		t.Errorf("unset scope selects %q, want shared", v)
	}
	if findElement(doc, hasAttr("value", "mine")) == nil {
		t.Error("trip scope select lacks Just for me")
	}
	label := findElement(doc, hasAttr("id", "task-scope-1-label"))
	if v, _ := attr(label, "class"); !strings.Contains(v, "sr-only") {
		t.Error("compact row label is visible")
	}
}

// Every dropdown field on the item, task and trip entry forms is a Select.
func TestFormsHaveNoBareNativeSelects(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	task := packing.PreparationTask{ID: "task", Name: "Buy fuel", Scope: "person"}
	session := packing.NewPackingSession(list)
	for name, component := range map[string]templ.Component{
		"add item":      AddItemForm(packing.NewCreateItemForm(list.ID), "token", false),
		"edit item row": ItemEditRow(list.ID, "item", packing.CreateItemForm{Scope: "person"}),
		"task row":      PreparationTaskRow(list, task, "token", true, "", ""),
		"trip entry":    TripEntryForm(session, nil, "token"),
	} {
		t.Run(name, func(t *testing.T) {
			doc := renderDoc(t, component)
			selects := findAll(doc, func(n *html.Node) bool { return n.Data == "select" })
			if len(selects) == 0 {
				t.Fatal("form has no dropdown field")
			}
			for _, s := range selects {
				id, _ := attr(s, "id")
				if v, _ := attr(s, "class"); !strings.Contains(v, "select-native") {
					t.Errorf("select %q is a bare browser dropdown", id)
				}
				if findElement(doc, hasAttr("aria-controls", id+"-listbox")) == nil {
					t.Errorf("select %q has no listbox trigger", id)
				}
			}
		})
	}
}

// The offline shell is static HTML, so its trip entry dropdowns are copies of
// the rendered Select markup and must not drift from it.
func TestOfflineShellSelectsMatchSelectField(t *testing.T) {
	shell, err := os.ReadFile("../../static/offline/offline.html")
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(s string) string {
		s = strings.ReplaceAll(s, "&#39;", "'")
		return regexp.MustCompile(`>\s+<`).ReplaceAllString(s, "><")
	}
	page := normalize(string(shell))
	for name, component := range map[string]templ.Component{
		"type":  SelectField("entry-kind", "kind", "Type", false, entryKindOptions, ""),
		"scope": ScopeSelect("scope", "entry-scope", "", true, false),
	} {
		var out bytes.Buffer
		if err := component.Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(page, normalize(out.String())) {
			t.Errorf("offline shell %s dropdown differs from the rendered Select", name)
		}
	}
}

func TestShellLoadsSelectBehaviourWithNoScriptFallback(t *testing.T) {
	var out bytes.Buffer
	if err := Shell("Lists").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	body := out.String()
	selectScript := strings.Index(body, `<script src="/static/select.js" defer></script>`)
	alpine := strings.Index(body, `<script src="/static/alpine.min.js" defer></script>`)
	if selectScript == -1 || selectScript > alpine {
		t.Error("select behaviour is not registered before Alpine starts")
	}
	if !strings.Contains(body, `.select-native { display: block !important; } .select-trigger { display: none !important; }`) {
		t.Error("without JavaScript the native select stays hidden")
	}
}
