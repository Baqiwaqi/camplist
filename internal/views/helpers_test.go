package views

import (
	"slices"
	"testing"

	"camplist/internal/packing"
)

func groupNames(groups []itemGroup) map[string][]string {
	out := map[string][]string{}
	for _, group := range groups {
		for _, item := range group.Items {
			out[group.Name] = append(out[group.Name], item.Name)
		}
	}
	return out
}

func TestGroupByCategoryFilesUncategorisedItemsUnderAnExistingOther(t *testing.T) {
	groups := groupByCategory([]packing.PackingItem{packing.NewItem("Tent", ""), packing.NewItem("Lamp", "other"), packing.NewItem("Stove", "Kitchen and cooking"), packing.NewItem("Rope", "")})
	if len(groups) != 2 || !slices.Equal(groupNames(groups)["other"], []string{"Lamp", "Tent", "Rope"}) {
		t.Errorf("groups = %q", groupNames(groups))
	}

	groups = groupByCategory([]packing.PackingItem{packing.NewItem("Stove", "Kitchen and cooking"), packing.NewItem("Tent", "")})
	if len(groups) != 2 || groups[1].Name != "Other" || !slices.Equal(groupNames(groups)["Other"], []string{"Tent"}) {
		t.Errorf("without an Other category groups = %q", groupNames(groups))
	}
}
