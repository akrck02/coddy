package gradle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAServerGradleHomeTimesItsDaemonsOutAndSharesThePersonsCaches(test *testing.T) {
	persons := test.TempDir()
	_ = os.MkdirAll(filepath.Join(persons, "caches", "modules-2"), 0o755)
	_ = os.MkdirAll(filepath.Join(persons, "wrapper", "dists"), 0o755)
	test.Setenv("GRADLE_USER_HOME", persons)
	data := test.TempDir()
	environment := PrepareHome(data)
	home := filepath.Join(data, "gradle")
	if !slices.Equal(environment, []string{"GRADLE_USER_HOME=" + home}) {
		test.Errorf("the environment is %v", environment)
	}
	properties, _ := os.ReadFile(filepath.Join(home, "gradle.properties"))
	if !strings.Contains(string(properties), "org.gradle.daemon.idletimeout=1800000") {
		test.Errorf("gradle.properties holds %q", properties)
	}
	for _, name := range []string{"caches", "wrapper", "jdks", "native"} {
		if target, err := os.Readlink(filepath.Join(home, name)); err != nil || target != filepath.Join(persons, name) {
			test.Errorf("%s links to %q (%v)", name, target, err)
		}
	}
}

func TestAGradleHomeOfTheOldKindGivesUpItsOwnCaches(test *testing.T) {
	persons := test.TempDir()
	test.Setenv("GRADLE_USER_HOME", persons)
	data := test.TempDir()
	made := filepath.Join(data, "gradle", "caches", "8.14", "generated-gradle-jars")
	_ = os.MkdirAll(made, 0o755)
	_ = os.MkdirAll(filepath.Join(data, "gradle", "daemon", "8.14"), 0o755)
	PrepareHome(data)
	if target, err := os.Readlink(filepath.Join(data, "gradle", "caches")); err != nil || target != filepath.Join(persons, "caches") {
		test.Errorf("caches links to %q (%v)", target, err)
	}
	if _, err := os.Stat(filepath.Join(persons, "caches", "8.14")); err == nil {
		test.Error("its own caches went into the person's")
	}
	if _, err := os.Stat(filepath.Join(data, "gradle", "daemon", "8.14")); err != nil {
		test.Error("its daemons' folder went")
	}
}
