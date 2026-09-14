package views

import (
	"testing"

	"camplist/internal/packing"

	"golang.org/x/net/html"
)

func pickerDoc(t *testing.T) *html.Node {
	t.Helper()
	return renderDoc(t, CategoryPicker("item-category", "item-category-options", "", "Pick or type one", []packing.CategoryOption{
		{Name: "Shelter"},
		{Name: "Fishing", Custom: true},
	}))
}

// ARIA forbids interactive descendants inside an option, so a screen reader
// would never reach the Edit button there. The popup is a grid instead: the
// category is one cell and its Edit button another beside it.
func TestCategoryPickerKeepsEditOutOfTheOption(t *testing.T) {
	doc := pickerDoc(t)

	if options := findAll(doc, hasAttr("role", "option")); len(options) != 0 {
		t.Errorf("options %d, want none: the popup is a grid", len(options))
	}
	grids := findAll(doc, hasAttr("role", "grid"))
	if len(grids) != 1 {
		t.Fatalf("grids %d, want 1", len(grids))
	}
	if !hasClass(grids[0], "popup") {
		t.Error("the picker popup does not use the shared popup style")
	}
	rows := findAll(doc, hasAttr("role", "row"))
	if len(rows) != 1 {
		t.Fatalf("rows %d, want 1 (the x-for row that every match is drawn from)", len(rows))
	}
	cells := findAll(rows[0], hasAttr("role", "gridcell"))
	if len(cells) != 2 {
		t.Fatalf("cells %d, want the category and its Edit button", len(cells))
	}
	if !hasClass(cells[0], "option") {
		t.Error("the category cell does not use the shared option style")
	}
	if len(findAll(cells[0], byTag("button"))) != 0 {
		t.Error("the category cell must hold no control of its own")
	}
	buttons := findAll(cells[1], byTag("button"))
	if len(buttons) != 1 || text(buttons[0]) != "Edit" {
		t.Fatalf("Edit buttons in the second cell %v", buttons)
	}
	if !hasClass(buttons[0], "option-action") {
		t.Error("the Edit button does not use the shared option action style")
	}
}

// The input is the combobox: it says what its popup is and which cell the
// keyboard is on.
func TestCategoryPickerComboboxPointsAtTheGrid(t *testing.T) {
	doc := pickerDoc(t)
	inputs := findAll(doc, hasAttr("role", "combobox"))
	if len(inputs) != 1 {
		t.Fatalf("comboboxes %d, want 1", len(inputs))
	}
	if got, _ := attr(inputs[0], "aria-haspopup"); got != "grid" {
		t.Errorf("aria-haspopup %q, want grid", got)
	}
	if !hasClass(inputs[0], "field") {
		t.Error("the picker input does not use the shared field style")
	}
	if got, _ := attr(inputs[0], "x-bind:aria-activedescendant"); got != "activeID()" {
		t.Errorf("aria-activedescendant binding %q", got)
	}
	if got, _ := attr(inputs[0], "name"); got != "category" {
		t.Errorf("the picker input must stay the category field, got name %q", got)
	}
}
