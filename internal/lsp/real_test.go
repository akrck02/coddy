package lsp

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
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
	requestContext, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := Start(requestContext, server, root, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	use := filepath.Join(root, "internal", "window", "uses.go")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(requestContext, use, placeOf(test, use, "PathOf("))
	if err != nil || len(declared) == 0 || !strings.HasSuffix(PathOf(declared[0].URI), filepath.Join("lsp", "protocol.go")) {
		test.Fatalf("PathOf is declared at %+v (%v), want internal/lsp/protocol.go", declared, err)
	}
	used, err := client.References(requestContext, use, placeOf(test, use, "PathOf("))
	if err != nil || len(used) < 3 {
		test.Errorf("PathOf is used at %d places (%v), want several", len(used), err)
	}
	if hover, _ := client.Hover(requestContext, use, placeOf(test, use, "PathOf(")); !strings.Contains(hover, "PathOf(uri string) string") {
		test.Errorf("the hover of PathOf says %q", hover)
	}
}

// jdtls follows a name across the files of a project. Across projects it needs a build that links
// them (Gradle's, in the big workspace): two plain folders are two projects that know nothing of each other.
func TestJdtlsFollowsANameAcrossFiles(test *testing.T) {
	server := realServer(test, "jdtls")
	folder := project(test, map[string]string{
		// a Maven project, as an imported one is where jdtls would write .project and .classpath
		"pom.xml": "<project><modelVersion>4.0.0</modelVersion><groupId>lamp</groupId><artifactId>lamp</artifactId>" +
			"<version>1</version><build><sourceDirectory>src</sourceDirectory></build></project>\n",
		"src/lamp/Lamp.java":     "package lamp;\n\npublic class Lamp {\n    public static int beam() { return 2; }\n}\n",
		"src/keeper/Keeper.java": "package keeper;\n\nimport lamp.Lamp;\n\npublic class Keeper {\n    int light = Lamp.beam();\n}\n",
	})
	before := filesIn(test, folder)
	requestContext, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client, err := Start(requestContext, server, "", []string{folder}, test.TempDir())
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	use := filepath.Join(folder, "src", "keeper", "Keeper.java")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(requestContext, use, placeOf(test, use, "beam("))
	if err != nil || len(declared) == 0 || filepath.Base(PathOf(declared[0].URI)) != "Lamp.java" {
		test.Errorf("beam is declared at %+v (%v), want Lamp.java", declared, err)
	}
	if written := newFiles(before, filesIn(test, folder)); len(written) > 0 {
		test.Errorf("jdtls wrote in the project: %v", written)
	}
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
	requestContext, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	data := test.TempDir()
	client, err := Start(requestContext, server, "", []string{folder}, data)
	if err != nil {
		test.Fatal(err)
	}
	defer theDaemonsStopped(test, client, data)
	use := filepath.Join(folder, "src", "main", "kotlin", "keeper", "Keeper.kt")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(requestContext, use, placeOf(test, use, "beam("))
	if err != nil || len(declared) == 0 || filepath.Base(PathOf(declared[0].URI)) != "Lamp.kt" {
		test.Errorf("beam is declared at %+v (%v), want Lamp.kt", declared, err)
	}
	test.Logf("kotlin-lsp wrote: %v", newFiles(before, filesIn(test, folder)))
	// its index in the folder the reader gives it, one for every set of projects; not in the
	// person's caches, where the reader removes nothing
	if indexes, _ := filepath.Glob(filepath.Join(data, server.Index.Folder, "*")); len(indexes) == 0 {
		test.Errorf("kotlin-lsp built no index in its data folder's %s: it went elsewhere (%v)", server.Index.Folder, entriesOf(data))
	}
}

// kotlin-lsp imports a project with Gradle the first time and keeps its model; the second time it
// reads it from the model, with no Gradle daemon, and follows a name as well.
func TestKotlinLspReadsAProjectFromItsModelTheSecondTime(test *testing.T) {
	server := realServer(test, "kotlin-lsp")
	folder := project(test, map[string]string{
		"settings.gradle.kts":              "rootProject.name = \"lamp\"\n",
		"build.gradle.kts":                 "plugins { kotlin(\"jvm\") version \"2.1.0\" }\n\nrepositories { mavenCentral() }\n",
		"src/main/kotlin/lamp/Lamp.kt":     "package lamp\n\nobject Lamp {\n    fun beam() = 2\n}\n",
		"src/main/kotlin/keeper/Keeper.kt": "package keeper\n\nimport lamp.Lamp\n\nclass Keeper {\n    val light = Lamp.beam()\n}\n",
	})
	data := test.TempDir()
	first := startedAndFollowing(test, server, folder, data)
	for wait := 0; wait < 120; wait++ {
		if _, _, kept, _ := first.Models(); !kept.IsZero() {
			break
		}
		time.Sleep(time.Second)
	}
	fromModel, imported, kept, why := first.Models()
	first.Stop()
	if kept.IsZero() || why != "" || imported != 1 || fromModel != 0 {
		test.Fatalf("first start: %d from a model, %d imported, kept at %v (%s)", fromModel, imported, kept, why)
	}
	daemons, _ := filepath.Glob(filepath.Join(data, "gradle", "daemon", "*", "daemon-*.out.log"))
	second := startedAndFollowing(test, server, folder, data)
	defer second.Stop()
	if fromModel, imported, _, _ := second.Models(); fromModel != 1 || imported != 0 {
		test.Errorf("second start: %d from a model, %d imported", fromModel, imported)
	}
	if again, _ := filepath.Glob(filepath.Join(data, "gradle", "daemon", "*", "daemon-*.out.log")); len(again) != len(daemons) {
		test.Errorf("reading from the model ran %d Gradle daemons", len(again)-len(daemons))
	}
}

// startedAndFollowing starts a server for a project and checks it follows Keeper's use of beam to
// Lamp.kt, once it is ready.
func startedAndFollowing(test *testing.T, server Server, folder, data string) *Client {
	requestContext, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	client, err := Start(requestContext, server, "", []string{folder}, data)
	if err != nil {
		test.Fatal(err)
	}
	use := filepath.Join(folder, "src", "main", "kotlin", "keeper", "Keeper.kt")
	content, _ := os.ReadFile(use)
	client.Tell(use, string(content), 1)
	declared, err := client.Definition(requestContext, use, placeOf(test, use, "beam("))
	if err != nil || len(declared) == 0 || filepath.Base(PathOf(declared[0].URI)) != "Lamp.kt" {
		test.Errorf("beam is declared at %+v (%v), want Lamp.kt", declared, err)
	}
	return client
}

// theDaemonsStopped stops a server and checks it imported with its own Gradle home, whose daemons
// stopped with it.
func theDaemonsStopped(test *testing.T, client *Client, data string) {
	client.Stop()
	logs, _ := filepath.Glob(filepath.Join(data, "gradle", "daemon", "*", "daemon-*.out.log"))
	if len(logs) == 0 {
		test.Error("no Gradle daemon ran in the server's own Gradle home")
	}
	if stopped := gradle.Daemons(data); stopped > 0 {
		test.Errorf("%d Gradle daemons outlived the server", stopped)
	}
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

// jdtls imports every folder it starts with: given them only in the parameters, it imported the
// first alone, and a file of another project read as a lone file, its imports unknown.
func TestJdtlsImportsEveryProjectItStartsWith(test *testing.T) {
	server := realServer(test, "jdtls")
	lamp := project(test, map[string]string{"pom.xml": pomNamed("lamp"), "src/lamp/Lamp.java": "package lamp;\npublic class Lamp {}\n"})
	keeper := project(test, map[string]string{"pom.xml": pomNamed("keeper"), "src/keeper/Keeper.java": "package keeper;\npublic class Keeper {}\n"})
	data := test.TempDir()
	requestContext, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client, err := Start(requestContext, server, "", []string{lamp, keeper}, data)
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	imported := filepath.Join(data, ".metadata", ".plugins", "org.eclipse.core.resources", ".projects")
	for wait := 0; wait < 300 && len(entriesOf(imported)) < 2; wait++ {
		time.Sleep(200 * time.Millisecond)
	}
	if names := entriesOf(imported); !slices.Contains(names, "lamp") || !slices.Contains(names, "keeper") {
		test.Errorf("jdtls imported %v", names)
	}
}

// pomNamed is the Maven build of a project of one artifact, its sources in src.
func pomNamed(name string) string {
	return "<project><modelVersion>4.0.0</modelVersion><groupId>lamp</groupId><artifactId>" + name +
		"</artifactId><version>1</version><build><sourceDirectory>src</sourceDirectory></build></project>\n"
}

// entriesOf are the names in a folder, none when it is not there.
func entriesOf(folder string) []string {
	entries, _ := os.ReadDir(folder)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
