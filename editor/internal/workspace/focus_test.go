package workspace

import "testing"

func TestLinesAwayFromTheFocusAreFolded(test *testing.T) {
	rows, diff := FocusRows(20, map[int]bool{5: true, 6: true, 18: true}, 2, map[int]bool{})
	if len(diff) != 20 || diff[4].NewNumber != 5 {
		test.Fatalf("the file as a diff is %d lines", len(diff))
	}
	var shown []int
	folds := 0
	for _, row := range rows {
		if row.IsFold() {
			folds++
			continue
		}
		shown = append(shown, row.Line.NewNumber)
	}
	want := []int{3, 4, 5, 6, 7, 8, 16, 17, 18, 19, 20}
	if folds != 2 || len(shown) != len(want) {
		test.Fatalf("shown %v with %d folds, want %v and 2", shown, folds, want)
	}
	for index := range want {
		if shown[index] != want[index] {
			test.Fatalf("shown %v, want %v", shown, want)
		}
	}
	if starts := FocusStarts(rows, map[int]bool{5: true, 6: true, 18: true}); len(starts) != 2 {
		test.Errorf("the runs of lines in focus start at rows %v, want two", starts)
	}
}
