package packing

import "testing"

func TestSessionStartsUncheckedAndIndependent(t *testing.T) {
	list := NewList("user", "Camping", "")
	list.Items = []PackingItem{NewItem("Tent", "Shelter")}
	list.Items[0].Checked = true
	session := NewPackingSession(list)
	if session.List.Items[0].Checked {
		t.Error("new session should start unpacked")
	}
	session.List.Items[0].Name = "Different tent"
	if list.Items[0].Name != "Tent" {
		t.Error("session shares mutable items with template")
	}
	if !list.Items[0].Checked {
		t.Error("session reset changed template")
	}
}

func TestWhitespaceNamesInvalid(t *testing.T) {
	if len((CreatePackingListForm{Name: " \t "}).Validate()) == 0 {
		t.Error("blank list name accepted")
	}
	if len((CreateItemForm{Name: " \t "}).Validate()) == 0 {
		t.Error("blank item name accepted")
	}
}
