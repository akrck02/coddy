package gradle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildOf writes the files of a build in a new project folder.
func buildOf(test *testing.T, files map[string]string) string {
	test.Helper()
	project := test.TempDir()
	for name, content := range files {
		path := filepath.Join(project, name)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return project
}

const wrapperOf = "distributionUrl=https\\://services.gradle.org/distributions/gradle-%s-bin.zip\n"

func TestAGradle4BuildCannotBeImported(test *testing.T) {
	old := buildOf(test, map[string]string{"gradle/wrapper/gradle-wrapper.properties": strings.Replace(wrapperOf, "%s", "4.10.3", 1)})
	if why := CannotImport(old); !strings.Contains(why, "Gradle 4.10.3") {
		test.Errorf("a Gradle 4.10.3 build is left in, or for %q", why)
	}
	recent := buildOf(test, map[string]string{"gradle/wrapper/gradle-wrapper.properties": strings.Replace(wrapperOf, "%s", "7.5.1", 1)})
	if why := CannotImport(recent); why != "" {
		test.Errorf("a Gradle 7.5.1 build is left out: %s", why)
	}
}

func TestAnAndroidBuildIsImportedOnlyWithItsSDK(test *testing.T) {
	test.Setenv("ANDROID_HOME", "")
	test.Setenv("ANDROID_SDK_ROOT", "")
	app := buildOf(test, map[string]string{"app/build.gradle": "plugins { id 'com.android.application' }\n"})
	if why := CannotImport(app); !strings.Contains(why, "Android SDK") {
		test.Errorf("an Android build with no SDK is left in, or for %q", why)
	}
	sdk := test.TempDir()
	_ = os.WriteFile(filepath.Join(app, "local.properties"), []byte("sdk.dir="+sdk+"\n"), 0o644)
	if why := CannotImport(app); why != "" {
		test.Errorf("an Android build whose local.properties names its SDK is left out: %s", why)
	}
	plain := buildOf(test, map[string]string{"build.gradle.kts": "plugins { kotlin(\"jvm\") }\n"})
	if why := CannotImport(plain); why != "" {
		test.Errorf("a build that is not Android's is left out: %s", why)
	}
}
