package sidebar

import "testing"

func TestAnEmptySidebarHasNoRowSelected(test *testing.T) {
	sidebar := New()
	if _, found := sidebar.SelectedRow(); found {
		test.Error("an empty sidebar has a row selected")
	}
}

func TestTheSelectedRowIsTheOneAtTheSelection(test *testing.T) {
	sidebar := New()
	sidebar.Rows = []Row{{Kind: ProjectRow, Label: "lighthouse"}, {Kind: FileRow, Label: "lamp.go"}}
	sidebar.Selected = 1
	if row, found := sidebar.SelectedRow(); !found || row.Label != "lamp.go" {
		test.Errorf("the selected row is %+v, want lamp.go", row)
	}
}
