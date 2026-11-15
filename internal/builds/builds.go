// Package builds reads what a project's Gradle or Maven build says it publishes and depends on,
// as text: fast, writing nothing and running nothing, and enough for the plain declarations a
// build mostly has. What it cannot read, a coordinate made in code, it says it did not understand.
package builds

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Build is what a project's build files say: the coordinates it publishes ("group:artifact"),
// those it depends on, and the declarations it could not read.
type Build struct {
	Publishes     []string
	Depends       []string
	NotUnderstood []string
}

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

// Read reads a project's build files among its files (paths from its folder, with slashes): the
// Gradle settings and builds, and Maven's pom.xml, at any depth but in a build's output.
func Read(folder string, files []string) Build {
	var build Build
	group, name := "", filepath.Base(folder)
	for _, path := range files {
		if !isBuildFile(path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		text := string(content)
		switch base := filepath.Base(path); {
		case base == "pom.xml":
			build.readPom(content)
		case strings.HasPrefix(base, "settings.gradle"):
			if found := gradleName.FindStringSubmatch(text); found != nil {
				name = found[1]
			}
		default:
			group = build.readGradle(text, group)
			if found := gradleArtifact.FindStringSubmatch(text); found != nil {
				build.Publishes = append(build.Publishes, group+":"+found[1])
			}
		}
	}
	if group != "" {
		build.Publishes = append(build.Publishes, group+":"+name)
	}
	build.tidy()
	return build
}

// isBuildFile tells whether a file of a project is a build file to read, not one a build wrote.
func isBuildFile(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "build" || part == "target" || part == "node_modules" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	base := filepath.Base(path)
	return base == "pom.xml" || base == "build.gradle" || base == "build.gradle.kts" ||
		base == "settings.gradle" || base == "settings.gradle.kts"
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

// pom is what the reader takes of a Maven build.
type pom struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Parent     struct {
		GroupID string `xml:"groupId"`
	} `xml:"parent"`
	Dependencies []struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
	} `xml:"dependencies>dependency"`
}

// readPom reads a Maven build: what it publishes, its group from its parent's when it has none.
func (build *Build) readPom(content []byte) {
	var read pom
	if xml.Unmarshal(content, &read) != nil {
		return
	}
	group := read.GroupID
	if group == "" {
		group = read.Parent.GroupID
	}
	if read.ArtifactID != "" {
		build.Publishes = append(build.Publishes, group+":"+read.ArtifactID)
	}
	for _, dependency := range read.Dependencies {
		coordinate := dependency.GroupID + ":" + dependency.ArtifactID
		if strings.Contains(coordinate, "$") {
			build.NotUnderstood = append(build.NotUnderstood, coordinate)
			continue
		}
		build.Depends = append(build.Depends, coordinate)
	}
}

// tidy sorts each list and drops what is twice in it.
func (build *Build) tidy() {
	for _, list := range []*[]string{&build.Publishes, &build.Depends, &build.NotUnderstood} {
		sort.Strings(*list)
		*list = compact(*list)
	}
}

// compact drops the repeats of a sorted list.
func compact(list []string) []string {
	var kept []string
	for index, each := range list {
		if index == 0 || each != list[index-1] {
			kept = append(kept, each)
		}
	}
	return kept
}

// Links are, for each project (by its index among those given), the projects it depends on: those
// that publish a coordinate it depends on.
func Links(projects []Build) [][]int {
	publisher := map[string]int{}
	for index, build := range projects {
		for _, coordinate := range build.Publishes {
			publisher[coordinate] = index
		}
	}
	links := make([][]int, len(projects))
	for index, build := range projects {
		seen := map[int]bool{}
		for _, coordinate := range build.Depends {
			if other, found := publisher[coordinate]; found && other != index && !seen[other] {
				seen[other] = true
				links[index] = append(links[index], other)
			}
		}
		sort.Ints(links[index])
	}
	return links
}
