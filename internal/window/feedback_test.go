package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

func TestTheTimeLeftIsToldByHowLongItTookTheTimeBefore(test *testing.T) {
	for _, each := range []struct {
		last, elapsed time.Duration
		want          string
	}{
		{0, 10 * time.Second, ""},
		{72 * time.Second, 30 * time.Second, "~45 s"},
		{5 * time.Minute, time.Minute, "~4 min"},
		{time.Minute, 58 * time.Second, ""},
		{time.Minute, 2 * time.Minute, ""},
	} {
		if words := leftWords(each.last, each.elapsed); words != each.want {
			test.Errorf("%v taken, %v gone: %q, want %q", each.last, each.elapsed, words, each.want)
		}
	}
}

func TestTheStatusBarSaysAWorkingServerInAFewWords(test *testing.T) {
	application := newReader(Settings{Servers: lsp.Servers})
	importing := viewOf(lsp.Progress{Phase: "importing", Percent: -1, Done: 9, Total: 16})
	importing.name = "kotlin-lsp"
	if words := application.shortStatus(importing); words != "Kotlin · importing 9/16" {
		test.Errorf("an import says %q", words)
	}
	indexing := viewOf(lsp.Progress{Phase: "indexing", Percent: 45})
	indexing.name = "jdtls"
	if words := application.shortStatus(indexing); words != "Java · indexing 45 %" {
		test.Errorf("an index says %q", words)
	}
}

func TestAProjectLeftOutOfTheServersIsToldWithWhy(test *testing.T) {
	folder, log := makeRepository(test), filepath.Join(test.TempDir(), "log")
	addHarbour(test, folder)
	wrapper := filepath.Join(folder, "harbour", "gradle", "wrapper")
	_ = os.MkdirAll(wrapper, 0o755)
	_ = os.WriteFile(filepath.Join(wrapper, "gradle-wrapper.properties"),
		[]byte("distributionUrl=https\\://services.gradle.org/distributions/gradle-4.10.3-bin.zip\n"), 0o644)
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true,
		ReadsAll: true, OwnGradle: true, Environment: []string{"FAKE_LSP_LOG=" + log}}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: servers})
	test.Cleanup(application.stopServers)
	newWindow(test, application).Frames(2)
	if count := application.leftOutCount(); count != 1 {
		test.Errorf("the status bar counts %d projects left out, want harbour", count)
	}
	harbour := application.ProjectIndex(filepath.Join(folder, "harbour"))
	if why := application.LeftOut(harbour); !strings.HasPrefix(why, "Its build needs Gradle 4.10.3") || !strings.Contains(why, "marked ?") {
		test.Errorf("the sidebar's mark says %q", why)
	}
	if why := application.LeftOut(application.ProjectIndex(filepath.Join(folder, "lighthouse"))); why != "" {
		test.Errorf("a project the server reads is marked left out: %q", why)
	}
	views := application.serverViews()
	if len(views) == 0 || len(views[0].leftOut) != 1 || views[0].leftOut[0].name != "harbour" {
		test.Errorf("the servers panel lists as left out %+v", views)
	}
}
