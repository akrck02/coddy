package english

import "testing"

func TestACountIsWrittenInTheSingularOnlyForOne(test *testing.T) {
	cases := map[string]string{
		Count(0, "project"):   "0 projects",
		Count(1, "project"):   "1 project",
		Count(22, "project"):  "22 projects",
		Count(2, "old index"): "2 old indexes",
		Count(3, "match"):     "3 matches",
	}
	for got, wanted := range cases {
		if got != wanted {
			test.Errorf("wrote %q, wanted %q", got, wanted)
		}
	}
}

func TestASentenceStartsWithACapital(test *testing.T) {
	if got := Capitalised("starting"); got != "Starting" {
		test.Errorf("wrote %q", got)
	}
	if got := Capitalised(""); got != "" {
		test.Errorf("wrote %q for nothing", got)
	}
}
