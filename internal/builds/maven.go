package builds

import (
	"encoding/xml"
	"strings"
)

// Maven reads Maven builds: each pom.xml, for what it publishes and its dependencies.
var Maven Reader = maven{}

type maven struct{}

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

// mavenFiles is Maven's one build file.
var mavenFiles = []string{"pom.xml"}

// Files is Maven's one build file.
func (maven) Files() []string { return mavenFiles }

// Read reads each pom of the project; the folder is not needed, as a pom names its artifact.
func (maven) Read(build *Build, folder string, files []File) {
	for _, file := range files {
		build.readPom(file.Content)
	}
}

// readPom reads a Maven build: what it publishes, its group from its parent's when it has none.
// A pom that is not XML says nothing.
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
