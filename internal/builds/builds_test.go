package builds

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// project writes files in a folder and lists them, as the reader lists a project's files.
func project(test *testing.T, files map[string]string) (string, []string) {
	folder := test.TempDir()
	var paths []string
	for path, text := range files {
		full := filepath.Join(folder, filepath.FromSlash(path))
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			test.Fatal(err)
		}
		paths = append(paths, path)
	}
	return folder, paths
}

func TestAGradleBuildSaysWhatItPublishesAndDependsOn(test *testing.T) {
	folder, files := project(test, map[string]string{
		"settings.gradle": "rootProject.name = 'lamp'\n",
		"build.gradle": "group 'org.lighthouse'\nversion lampVersion\n\ndependencies {\n" +
			"    implementation \"org.harbour:quay:${quayVersion}\"\n" +
			"    api('org.harbour:buoy:1.0')\n" +
			"    implementation \"${keeperGroup}:keeper:1\"\n" +
			"    implementation libs.rope\n}\n",
		"build/tmp/build.gradle": "group 'written.by.a.build'\n",
	})
	build := Read(folder, files)
	if !slices.Equal(build.Publishes, []string{"org.lighthouse:lamp"}) {
		test.Errorf("it publishes %v", build.Publishes)
	}
	if !slices.Equal(build.Depends, []string{"org.harbour:buoy", "org.harbour:quay"}) {
		test.Errorf("it depends on %v", build.Depends)
	}
	if !slices.Equal(build.NotUnderstood, []string{"${keeperGroup}:keeper:1", "libs.rope"}) {
		test.Errorf("it did not understand %v", build.NotUnderstood)
	}
}

func TestAMavenBuildTakesItsGroupFromItsParent(test *testing.T) {
	folder, files := project(test, map[string]string{
		"pom.xml": "<project><parent><groupId>org.harbour</groupId></parent><artifactId>quay</artifactId>" +
			"<dependencies><dependency><groupId>org.lighthouse</groupId><artifactId>lamp</artifactId></dependency></dependencies></project>",
	})
	build := Read(folder, files)
	if !slices.Equal(build.Publishes, []string{"org.harbour:quay"}) || !slices.Equal(build.Depends, []string{"org.lighthouse:lamp"}) {
		test.Errorf("it publishes %v and depends on %v", build.Publishes, build.Depends)
	}
}

func TestProjectsAreLinkedByWhatTheyPublish(test *testing.T) {
	links := Links([]Build{
		{Publishes: []string{"org.lighthouse:lamp"}},
		{Publishes: []string{"org.harbour:quay"}, Depends: []string{"org.lighthouse:lamp", "org.other:rope"}},
		{Depends: []string{"org.harbour:quay", "org.lighthouse:lamp"}},
	})
	if len(links[0]) != 0 || !slices.Equal(links[1], []int{0}) || !slices.Equal(links[2], []int{0, 1}) {
		test.Errorf("the links are %v", links)
	}
}
