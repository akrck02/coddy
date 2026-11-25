package builds

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Gradle reads Gradle builds: the settings, for the project's name, and each build, for its
// group, what it publishes and its dependencies.
var Gradle Reader = gradle{}

type gradle struct{}

var (
	gradleGroup    = regexp.MustCompile(`(?m)^\s*group\s*=?\s*["']([^"'$]+)["']`)
	gradleName     = regexp.MustCompile(`(?m)^\s*rootProject\.name\s*=\s*["']([^"']+)["']`)
	gradleArtifact = regexp.MustCompile(`(?m)\b(?:artifactId|archivesBaseName)\s*=\s*["']([^"'$]+)["']`)
	// a dependency declared with its coordinates in a string: implementation "group:artifact:1.0"
	gradleDependency = regexp.MustCompile(`(?m)^\s*(?:api|implementation|compileOnly|runtimeOnly|testImplementation|` +
		`testCompileOnly|testRuntimeOnly|annotationProcessor|kapt|ksp|compile|testCompile|runtime)\s*\(?\s*["']([^"']+)["']`)
	// one declared otherwise: from a catalog (libs.lamp), or with a coordinate made in code
	gradleOther = regexp.MustCompile(`(?m)^\s*(?:api|implementation|compileOnly|runtimeOnly)\s*\(?\s*(libs\.[\w.]+|[a-z]\w*\([^)]*\))`)
)

// gradleFiles are Gradle's settings and build files, in Groovy and in Kotlin.
var gradleFiles = []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"}

// Files are Gradle's settings and build files, in Groovy and in Kotlin.
func (gradle) Files() []string { return gradleFiles }

// Read reads the project's name from its settings (its folder's name when they say none) and its
// group from its builds; with a group, the project publishes group:name.
func (gradle) Read(build *Build, folder string, files []File) {
	group, name := "", filepath.Base(folder)
	for _, file := range files {
		text := string(file.Content)
		if strings.HasPrefix(filepath.Base(file.Path), "settings.gradle") {
			if found := gradleName.FindStringSubmatch(text); found != nil {
				name = found[1]
			}
			continue
		}
		group = build.readGradle(text, group)
		if found := gradleArtifact.FindStringSubmatch(text); found != nil {
			build.Publishes = append(build.Publishes, group+":"+found[1])
		}
	}
	if group != "" {
		build.Publishes = append(build.Publishes, group+":"+name)
	}
}

// readGradle reads a Gradle build: its group, if it says one, and its dependencies.
func (build *Build) readGradle(text, group string) string {
	if found := gradleGroup.FindStringSubmatch(text); found != nil {
		group = found[1]
	}
	for _, found := range gradleDependency.FindAllStringSubmatch(text, -1) {
		parts := strings.Split(found[1], ":")
		if len(parts) < 2 || strings.Contains(parts[0]+parts[1], "$") {
			build.NotUnderstood = append(build.NotUnderstood, found[1])
			continue
		}
		build.Depends = append(build.Depends, parts[0]+":"+parts[1])
	}
	for _, found := range gradleOther.FindAllStringSubmatch(text, -1) {
		if !strings.HasPrefix(found[1], "project(") && !strings.HasPrefix(found[1], "platform(") {
			build.NotUnderstood = append(build.NotUnderstood, found[1])
		}
	}
	return group
}
