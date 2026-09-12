package views

import (
	"strconv"
	"strings"
)

// demoItem is sample gear for the public pages. Nothing here is saved:
// Alpine keeps the packed state in the browser until the page closes.
type demoItem struct {
	Name     string
	Category string
	Packed   bool
}

// heroItems is the working session in the landing page hero.
var heroItems = []demoItem{
	{"Tent", "Shelter", true},
	{"Sleeping bag", "Sleep", true},
	{"Sleeping pad", "Sleep", true},
	{"Stove and fuel", "Kitchen", false},
	{"Headlamp", "Light", false},
	{"Water filter", "Water", false},
	{"First aid kit", "Safety", false},
}

// templateItems appear twice in the explainer: as the reusable list and as
// the finished session copied from it.
var templateItems = []demoItem{
	{"Tent", "Shelter", true},
	{"Sleeping bag", "Sleep", true},
	{"Camp chairs", "Comfort", true},
	{"Cooler", "Kitchen", true},
	{"Lantern", "Light", true},
}

// demoSessionItems fill the full demo page, grouped by category like a real session.
var demoSessionItems = []demoItem{
	{"Tent", "Shelter", true},
	{"Tarp", "Shelter", true},
	{"Ground sheet", "Shelter", false},
	{"Sleeping bag", "Sleep", true},
	{"Sleeping pad", "Sleep", true},
	{"Pillow", "Sleep", false},
	{"Stove and fuel", "Kitchen", true},
	{"Lighter", "Kitchen", false},
	{"Pot and pan", "Kitchen", false},
	{"Cooler", "Kitchen", false},
	{"Coffee", "Kitchen", false},
	{"Rain jacket", "Clothing", false},
	{"Warm layer", "Clothing", false},
	{"Spare socks", "Clothing", false},
	{"Headlamp", "Safety", false},
	{"First aid kit", "Safety", false},
	{"Water filter", "Safety", false},
	{"Map", "Safety", false},
}

// packedCount counts the items packed in the initial state.
func packedCount(items []demoItem) int {
	n := 0
	for _, item := range items {
		if item.Packed {
			n++
		}
	}
	return n
}

// demoState is the Alpine x-data for a demo checklist: the packed flags, the
// initial flags for reset, and the derived count. The server renders the same
// initial state so nothing changes when Alpine starts.
func demoState(items []demoItem) string {
	flags := make([]string, len(items))
	for i, item := range items {
		flags[i] = strconv.FormatBool(item.Packed)
	}
	list := "[" + strings.Join(flags, ",") + "]"
	return "{ initial: " + list + ", packed: " + list +
		", get count() { return this.packed.filter(Boolean).length }" +
		", get done() { return this.count === this.packed.length }" +
		", toggle(i) { this.packed[i] = !this.packed[i] }" +
		", reset() { this.packed = this.initial.slice() } }"
}

// demoEntry pairs an item with its index in the Alpine packed array.
type demoEntry struct {
	Index int
	Item  demoItem
}

type demoGroup struct {
	Name  string
	Items []demoEntry
}

// groupDemo groups demo items by category in order of first use, keeping the
// index each row needs to toggle its flag.
func groupDemo(items []demoItem) []demoGroup {
	index := map[string]int{}
	var groups []demoGroup
	for i, item := range items {
		g, ok := index[item.Category]
		if !ok {
			g = len(groups)
			index[item.Category] = g
			groups = append(groups, demoGroup{Name: item.Category})
		}
		groups[g].Items = append(groups[g].Items, demoEntry{Index: i, Item: item})
	}
	return groups
}
