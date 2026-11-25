package lsp

import "testing"

func TestColumnsCountUTF16UnitsBothWays(test *testing.T) {
	line := "a😀b"
	if units := UTF16Column(line, 2); units != 3 {
		test.Errorf("two characters are %d units, want 3", units)
	}
	if characters := CharacterColumn(line, 3); characters != 2 {
		test.Errorf("three units are %d characters, want 2", characters)
	}
}

func TestAFileURIRoundTrips(test *testing.T) {
	path := "/work/my repo/Main.java"
	if back := PathOf(FileURI(path)); back != path {
		test.Errorf("%q came back as %q", path, back)
	}
	if PathOf("jdt://contents/rt.jar/java.lang/String.class") != "" {
		test.Error("a jar's URI was taken for a file")
	}
}

func TestAJarLocationNamesItsClass(test *testing.T) {
	uri := "jdt://contents/lamp-1.0-SNAPSHOT.jar/org.lighthouse.lamp/Lamp.java?=keeper/%5C/home%5C/.m2%5C/lamp.jar=/%3Corg.lighthouse.lamp%28Lamp.class"
	kotlin := "jar:///home/.gradle/caches/lamp-1.0.jar!/org/lighthouse/lamp/Lamp$Beam.class"
	for _, each := range []string{uri, kotlin} {
		if packageName, class, found := ClassInJar(each); !found || packageName != "org.lighthouse.lamp" || class != "Lamp" {
			test.Errorf("the class of %s is %q %q (%t)", each, packageName, class, found)
		}
	}
	if _, _, found := ClassInJar("file:///lamp/Lamp.java"); found {
		test.Error("a file is taken for a class in a jar")
	}
}
