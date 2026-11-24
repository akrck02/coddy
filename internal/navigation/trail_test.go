package navigation

import "testing"

func TestGoingBackAndForwardReturnsToEachPlace(test *testing.T) {
	var trail Trail
	first, second := Spot{Path: "/sea/a.go", Line: 10}, Spot{Path: "/sea/b.go", Line: 20}
	trail.Leave(first)
	back, found := trail.Next(-1)
	if !found || back != first {
		test.Fatalf("back leads to %v (%t), want %v", back, found, first)
	}
	trail.Take(-1, second, true)
	forward, found := trail.Next(1)
	if !found || forward != second {
		test.Errorf("forward leads to %v (%t), want %v", forward, found, second)
	}
	if trail.Behind() != 0 {
		test.Errorf("the trail keeps %d places back after going back, want 0", trail.Behind())
	}
}

func TestLeavingAPlaceForgetsTheWayForward(test *testing.T) {
	var trail Trail
	trail.Leave(Spot{Path: "/sea/a.go", Line: 1})
	trail.Take(-1, Spot{Path: "/sea/b.go", Line: 2}, true)
	trail.Leave(Spot{Path: "/sea/c.go", Line: 3})
	if _, found := trail.Next(1); found {
		test.Error("the way forward is kept after leaving for a new place")
	}
}

func TestTheTrailKeepsItsLengthAtMost(test *testing.T) {
	var trail Trail
	for line := 1; line <= trailLength+10; line++ {
		trail.Leave(Spot{Path: "/sea/a.go", Line: line})
	}
	if trail.Behind() != trailLength {
		test.Errorf("the trail keeps %d places back, want %d", trail.Behind(), trailLength)
	}
}
