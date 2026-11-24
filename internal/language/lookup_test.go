package language

import "testing"

func TestAFileIsKnownByItsNameThenItsLongestExtensionThenItsStem(test *testing.T) {
	cases := map[string]string{
		"lighthouse/Lamp.java":   "Java",
		"build.gradle.kts":       "Gradle in Kotlin",
		"settings.kts":           "Kotlin script",
		"docs/GUIDE.Markdown":    Markdown,
		"Dockerfile":             "Dockerfile",
		"Dockerfile.release":     "Dockerfile",
		"harbour/.env":           "Environment",
		"notes/without-a-kind.q": "",
	}
	for path, want := range cases {
		if known, _ := Of(path); known.Name != want {
			test.Errorf("Of(%q) is %q, want %q", path, known.Name, want)
		}
	}
}

func TestAFenceNamesItsLanguageByWordOrByExtension(test *testing.T) {
	cases := map[string]string{"kotlin": ".kt", "Golang": ".go", "json": ".json", "yml": ".yaml", "c#": ".cs"}
	for word, want := range cases {
		known, found := OfFence(word)
		if !found || known.Extensions[0] != want {
			test.Errorf("OfFence(%q) is %v, want a language of %q", word, known.Extensions, want)
		}
	}
	if _, found := OfFence(""); found {
		test.Error("an empty fence names a language")
	}
}

func TestEachExtensionAndNameBelongsToOneLanguage(test *testing.T) {
	seen := map[string]string{}
	for _, known := range Languages {
		for _, key := range append(append([]string{}, known.Extensions...), known.FileNames...) {
			if other, taken := seen[key]; taken {
				test.Errorf("%q is both %s and %s", key, other, known.Name)
			}
			seen[key] = known.Name
		}
	}
}

func TestFindingALanguageAllocatesNothing(test *testing.T) {
	allocations := testing.AllocsPerRun(100, func() { Of("lighthouse/src/lamp.go") })
	if allocations > 0 {
		test.Errorf("finding a file's language allocates %v times", allocations)
	}
}
