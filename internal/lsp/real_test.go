package lsp

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// realServer is a server of the table found on the PATH; the test skips when it is not installed.
func realServer(test *testing.T, name string) Server {
	server, _ := serverNamed(name)
	if _, err := exec.LookPath(server.Command); err != nil {
		test.Skipf("%s is not installed (%s)", name, server.Install)
	}
	return server
}

func serverNamed(name string) (Server, bool) {
	for _, server := range Servers {
		if server.Name == name {
			return server, true
		}
	}
	return Server{}, false
}

// placeOf is where a text first appears in a file, as the protocol counts it.
func placeOf(test *testing.T, path, text string) Position {
	content, err := os.ReadFile(path)
	if err != nil {
		test.Fatal(err)
	}
	for number, line := range strings.Split(string(content), "\n") {
		if at := strings.Index(line, text); at >= 0 {
			return Position{Line: number, Character: UTF16Column(line, len([]rune(line[:at])))}
		}
	}
	test.Fatalf("%s does not hold %q", path, text)
	return Position{}
}

func TestGoplsFollowsANameInThisRepository(test *testing.T) {
	server := realServer(test, "gopls")
	root, _ := filepath.Abs("../..")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := Start(ctx, server, root, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	use := filepath.Join(root, "internal", "window", "uses.go")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(ctx, use, placeOf(test, use, "PathOf("))
	if err != nil || len(declared) == 0 || !strings.HasSuffix(PathOf(declared[0].URI), filepath.Join("lsp", "protocol.go")) {
		test.Fatalf("PathOf is declared at %+v (%v), want internal/lsp/protocol.go", declared, err)
	}
	used, err := client.References(ctx, use, placeOf(test, use, "PathOf("))
	if err != nil || len(used) < 3 {
		test.Errorf("PathOf is used at %d places (%v), want several", len(used), err)
	}
	if hover, _ := client.Hover(ctx, use, placeOf(test, use, "PathOf(")); !strings.Contains(hover, "PathOf(uri string) string") {
		test.Errorf("the hover of PathOf says %q", hover)
	}
}

// jdtls follows a name across the files of a project. Across projects it needs a build that links
// them (Gradle's, in the big workspace): two plain folders are two projects that know nothing of each other.
func TestJdtlsFollowsANameAcrossFiles(test *testing.T) {
	server := realServer(test, "jdtls")
	folder := project(test, map[string]string{
		"src/lamp/Lamp.java":     "package lamp;\n\npublic class Lamp {\n    public static int beam() { return 2; }\n}\n",
		"src/keeper/Keeper.java": "package keeper;\n\nimport lamp.Lamp;\n\npublic class Keeper {\n    int light = Lamp.beam();\n}\n",
	})
	before := filesIn(test, folder)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client, err := Start(ctx, server, "", []string{folder}, test.TempDir())
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	use := filepath.Join(folder, "src", "keeper", "Keeper.java")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(ctx, use, placeOf(test, use, "beam("))
	if err != nil || len(declared) == 0 || filepath.Base(PathOf(declared[0].URI)) != "Lamp.java" {
		test.Errorf("beam is declared at %+v (%v), want Lamp.java", declared, err)
	}
	test.Logf("jdtls wrote: %v", newFiles(before, filesIn(test, folder)))
}

// kotlin-lsp follows a name across the files of a Gradle project, once it has imported it.
func TestKotlinLspFollowsANameAcrossFiles(test *testing.T) {
	server := realServer(test, "kotlin-lsp")
	folder := project(test, map[string]string{
		"settings.gradle.kts":              "rootProject.name = \"lamp\"\n",
		"build.gradle.kts":                 "plugins { kotlin(\"jvm\") version \"2.1.0\" }\n\nrepositories { mavenCentral() }\n",
		"src/main/kotlin/lamp/Lamp.kt":     "package lamp\n\nobject Lamp {\n    fun beam() = 2\n}\n",
		"src/main/kotlin/keeper/Keeper.kt": "package keeper\n\nimport lamp.Lamp\n\nclass Keeper {\n    val light = Lamp.beam()\n}\n",
	})
	before := filesIn(test, folder)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	client, err := Start(ctx, server, "", []string{folder}, test.TempDir())
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	use := filepath.Join(folder, "src", "main", "kotlin", "keeper", "Keeper.kt")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(ctx, use, placeOf(test, use, "beam("))
	if err != nil || len(declared) == 0 || filepath.Base(PathOf(declared[0].URI)) != "Lamp.kt" {
		test.Errorf("beam is declared at %+v (%v), want Lamp.kt", declared, err)
	}
	test.Logf("kotlin-lsp wrote: %v", newFiles(before, filesIn(test, folder)))
}

// filesIn lists every file under some folders.
func filesIn(test *testing.T, folders ...string) []string {
	var found []string
	for _, folder := range folders {
		_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				found = append(found, path)
			}
			return nil
		})
	}
	sort.Strings(found)
	return found
}

// newFiles are the files of after that are not in before.
func newFiles(before, after []string) []string {
	known := map[string]bool{}
	for _, path := range before {
		known[path] = true
	}
	var added []string
	for _, path := range after {
		if !known[path] {
			added = append(added, path)
		}
	}
	return added
}
