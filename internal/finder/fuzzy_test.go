package finder

import "testing"

func TestTheFinderPrefersTheFileName(test *testing.T) {
	candidates := []Candidate{
		{Label: "api/src/order/service/Helper.kt"},
		{Label: "api/src/OrderService.kt"},
		{Label: "api/src/Other.kt", Changed: true},
	}
	matches := FuzzyFind(candidates, "ordsvc", 10)
	if len(matches) != 2 || matches[0].Candidate.Label != "api/src/OrderService.kt" {
		test.Fatalf("matches: %+v", matches)
	}
	if empty := FuzzyFind(candidates, "", 10); len(empty) != 1 || !empty[0].Candidate.Changed {
		test.Errorf("an empty query lists %+v, want the changed file", empty)
	}
}
