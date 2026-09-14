package packing

import (
	"fmt"
	"strings"
)

// maxPasteLength bounds the pasted text before it is parsed: MaxItemsPerAdd
// full-length names plus room for category headings.
const maxPasteLength = 32000

// AddSeveralForm is the Add several form: pasted lines, an optional category
// for lines under no heading, and the list revision the page showed.
type AddSeveralForm struct {
	Action     string           `schema:"-"`
	Revision   string           `schema:"revision"`
	Lines      string           `schema:"lines"`
	Category   string           `schema:"category"`
	Categories []CategoryOption `schema:"-"`
	Error      []string         `schema:"-"`
}

func NewAddSeveralForm(list PackingList) AddSeveralForm {
	return AddSeveralForm{Action: "/packing-lists/" + list.ID + "/add-several", Revision: list.Revision()}
}

// Items reads the pasted lines into new items, or explains what to fix.
func (f AddSeveralForm) Items() ([]PackingItem, []string) {
	if len(f.Lines) > maxPasteLength {
		return nil, []string{"That is too much text to add at once. Paste at most 32,000 characters, so split it in two."}
	}
	fallback := MatchCategory(f.Category)
	if len(fallback) > MaxCategoryLength {
		return nil, []string{fmt.Sprintf("The category is longer than %d characters.", MaxCategoryLength)}
	}
	lines := ParseItemLines(f.Lines)
	switch {
	case len(lines) == 0:
		return nil, []string{"Type or paste at least one item, one per line."}
	case len(lines) > MaxItemsPerAdd:
		return nil, []string{fmt.Sprintf("Add at most %d items at a time. This has %d, so split it in two.", MaxItemsPerAdd, len(lines))}
	}
	var items []PackingItem
	var errs []string
	for _, line := range lines {
		category := line.Category
		if category == "" {
			category = fallback
		}
		switch {
		case len(line.Name) > MaxItemNameLength:
			errs = append(errs, fmt.Sprintf("Line %d is longer than %d characters.", line.Line, MaxItemNameLength))
		case len(category) > MaxCategoryLength:
			errs = append(errs, fmt.Sprintf("The category heading above line %d is longer than %d characters.", line.Line, MaxCategoryLength))
		}
		items = append(items, PackingItem{Name: line.Name, Category: category})
	}
	if len(errs) > 3 {
		errs = append(errs[:3], fmt.Sprintf("%d more lines are too long.", len(errs)-3))
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return items, nil
}

// Summary describes a bulk add for the camper, for example "Added 2 items to
// Documents. Skipped 1 already on this list or repeated: Passport." from, such
// as "from Climbing", names where the items came from.
func (r AddItemsResult) Summary(from string) string {
	var b strings.Builder
	noun := "items"
	if len(r.Added) == 1 {
		noun = "item"
	}
	fmt.Fprintf(&b, "Added %d %s", len(r.Added), noun)
	if from != "" {
		b.WriteString(" " + from)
	} else if category := sharedCategory(r.Added); category != "" {
		b.WriteString(" to " + category)
	}
	b.WriteString(".")
	if len(r.Skipped) > 0 {
		const shown = 5
		names := r.Skipped
		more := ""
		if len(names) > shown {
			more = fmt.Sprintf(" and %d more", len(names)-shown)
			names = names[:shown]
		}
		fmt.Fprintf(&b, " Skipped %d already on this list or repeated: %s%s.", len(r.Skipped), strings.Join(names, ", "), more)
	}
	return b.String()
}

func sharedCategory(items []PackingItem) string {
	if len(items) == 0 {
		return ""
	}
	for _, item := range items[1:] {
		if item.Category != items[0].Category {
			return ""
		}
	}
	return items[0].Category
}
