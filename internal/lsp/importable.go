package lsp

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Some builds cannot be imported, and a server that imports Gradle builds tries them again each
// time it starts, failing the same way, minutes of daemons for nothing: on the big workspace, the Android apps
// without the Android SDK, and the builds of Gradle 4, which runs on Java 8 to 10 only. The reader
// leaves them out of such a server and says why; text search still reads them.

// oldestGradle is the oldest Gradle a server imports: Gradle 4 runs on Java 8 to 10, and the
// servers run Gradle with a newer one.
const oldestGradle = 5

// wrapperVersion finds the Gradle version a wrapper's properties name: gradle-4.10.3-bin.zip.
var wrapperVersion = regexp.MustCompile(`gradle-(\d+)\.(\d+)(?:\.(\d+))?-(?:bin|all)\.zip`)

// CannotImport says why a project's build cannot be imported by a server that imports Gradle
// builds; "" when it can, or when it has no Gradle build.
func CannotImport(project string) string {
	if version, major, found := gradleVersion(project); found && major < oldestGradle {
		return "its build needs Gradle " + version + ", which runs on Java 8 to 10 only"
	}
	if usesAndroid(project) && !hasAndroidSDK(project) {
		return "it is an Android build, and the Android SDK is not installed (ANDROID_HOME, or sdk.dir in local.properties)"
	}
	return ""
}

// gradleVersion is the Gradle version a project's wrapper names, and its major number.
func gradleVersion(project string) (version string, major int, found bool) {
	properties, err := os.ReadFile(filepath.Join(project, "gradle", "wrapper", "gradle-wrapper.properties"))
	if err != nil {
		return "", 0, false
	}
	parts := wrapperVersion.FindStringSubmatch(string(properties))
	if parts == nil {
		return "", 0, false
	}
	major, _ = strconv.Atoi(parts[1])
	version = parts[1] + "." + parts[2]
	if parts[3] != "" {
		version += "." + parts[3]
	}
	return version, major, true
}

// usesAndroid tells whether a build file of the project, at its top or one folder down (app/),
// applies an Android plugin.
func usesAndroid(project string) bool {
	for _, pattern := range []string{"build.gradle*", "*/build.gradle*"} {
		files, _ := filepath.Glob(filepath.Join(project, pattern))
		for _, file := range files {
			if content, err := os.ReadFile(file); err == nil && strings.Contains(string(content), "com.android.") {
				return true
			}
		}
	}
	return false
}

// hasAndroidSDK tells whether Gradle would find the Android SDK for a project: a folder named by
// ANDROID_HOME or ANDROID_SDK_ROOT, or by sdk.dir in the project's local.properties.
func hasAndroidSDK(project string) bool {
	folders := []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT")}
	if properties, err := os.ReadFile(filepath.Join(project, "local.properties")); err == nil {
		for _, line := range strings.Split(string(properties), "\n") {
			if value, found := strings.CutPrefix(strings.TrimSpace(line), "sdk.dir="); found {
				folders = append(folders, strings.TrimSpace(value))
			}
		}
	}
	for _, folder := range folders {
		if info, err := os.Stat(folder); folder != "" && err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
