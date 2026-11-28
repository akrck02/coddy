package navigation

import "testing"

func TestANameIsFoundEachTimeItStandsAsAWordInALine(test *testing.T) {
	columns := wordColumns("    Lamp lamp = new Lamp(); // Lamps", "Lamp")
	if len(columns) != 2 || columns[0] != 4 || columns[1] != 20 {
		test.Errorf("Lamp stands at %v, want [4 20]", columns)
	}
}

func TestAFileNamesADeclarationByItsImportOrItsPackage(test *testing.T) {
	for text, want := range map[string]bool{
		"package org.harbour;\nimport org.lighthouse.Lamp;\n":      true,
		"package org.harbour\nimport org.lighthouse.*\n":           true,
		"package org.lighthouse;\n":                                true,
		"package org.harbour;\nimport org.lighthouse.LampStore;\n": false,
		"package org.harbour;\nimport org.quay.Lamp;\n":            false,
	} {
		if got := namesDeclaration(text, "org.lighthouse", "Lamp"); got != want {
			test.Errorf("%q names Lamp: %t, want %t", text, got, want)
		}
	}
}
