package settings

import "testing"

func TestACardCountsFilesInRoundHundreds(test *testing.T) {
	cases := map[int]string{0: "100", 93: "100", 187: "200", 655: "700", 1310: "1,300", 2621: "2,600", 12049: "12,000"}
	for count, want := range cases {
		if got := roughly(count); got != want {
			test.Errorf("%d files are written %q, want %q", count, got, want)
		}
	}
}
