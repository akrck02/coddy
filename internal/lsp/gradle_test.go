package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAServerGradleHomeTimesItsDaemonsOutAndSharesThePersonsCaches(test *testing.T) {
	persons := test.TempDir()
	_ = os.MkdirAll(filepath.Join(persons, "caches", "modules-2"), 0o755)
	_ = os.MkdirAll(filepath.Join(persons, "wrapper", "dists"), 0o755)
	test.Setenv("GRADLE_USER_HOME", persons)
	data := test.TempDir()
	environment := prepareGradleHome(data)
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
	prepareGradleHome(data)
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

// sleeping starts a process that waits, named as a Gradle daemon is when named so, and writes the
// log a daemon of a Gradle home writes.
func sleeping(test *testing.T, data, name string) *exec.Cmd {
	command := exec.Command("/bin/bash", "-c", "exec -a "+name+" /bin/sleep 60")
	if err := command.Start(); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = command.Process.Kill() })
	folder := filepath.Join(data, "gradle", "daemon", "8.14")
	_ = os.MkdirAll(folder, 0o755)
	_ = os.WriteFile(filepath.Join(folder, "daemon-"+strconv.Itoa(command.Process.Pid)+".out.log"), nil, 0o644)
	return command
}

func TestTheDaemonsOfAServerGradleHomeStopWithItAndNoOtherProcess(test *testing.T) {
	data := test.TempDir()
	daemon := sleeping(test, data, "GradleDaemon")
	other := sleeping(test, data, "lighthouse")
	ended := make(chan error, 1)
	go func() { ended <- daemon.Wait() }() // a process of the test's own ends once waited for
	if stopped := StopGradleDaemons(data); stopped != 1 {
		test.Errorf("%d daemons were stopped, want 1", stopped)
	}
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		test.Error("the daemon still runs")
	}
	if err := other.Process.Signal(syscall.Signal(0)); err != nil {
		test.Errorf("a process that is no daemon was stopped: %v", err)
	}
}

func TestTheJavaServerReadsEveryJavaProjectOfTheWindow(test *testing.T) {
	for _, server := range Servers {
		if server.Name == "jdtls" && !server.ReadsAll {
			test.Error("jdtls reads only the projects in sight: it would drop the others from its workspace and import them again")
		}
	}
}

func TestAServerIsDoneImportingOnceReadyAndQuietAWhile(test *testing.T) {
	now := time.Now()
	client := &Client{ready: make(chan struct{}), working: map[string]*work{}, imported: map[string]time.Time{}}
	if client.settled(now) {
		test.Error("a server not ready yet is done importing")
	}
	close(client.ready)
	client.readyAt = now.Add(-time.Second)
	if client.settled(now) {
		test.Error("a server ready a second ago is done importing: it may report work again")
	}
	client.readyAt = now.Add(-time.Minute)
	client.working["import"] = &work{title: "Importing"}
	if client.settled(now) {
		test.Error("a server still importing is done importing")
	}
	delete(client.working, "import")
	client.idleAt = now.Add(-5 * time.Second)
	if client.settled(now) {
		test.Error("a server whose import ended 5 seconds ago is done importing")
	}
	client.idleAt = now.Add(-time.Minute)
	if !client.settled(now) {
		test.Error("a server ready and quiet for a minute is not done importing")
	}
	client.imported["lamp"] = now.Add(-3 * time.Second)
	if client.settled(now) {
		test.Error("a server that began importing a project 3 seconds ago is done importing")
	}
}

// kotlin-lsp answers with nothing until its first import is done: it is ready once that work ends.
func TestAServerReadyAfterWorkIsReadyOnceItsFirstWorkEnds(test *testing.T) {
	client := &Client{Server: Server{ReadyAfterWork: true}, ready: make(chan struct{}), working: map[string]*work{},
		imported: map[string]time.Time{}, changes: make(chan struct{}, 1)}
	client.noteProgress([]byte(`{"token":"import","value":{"kind":"begin","title":"Importing"}}`))
	if client.isReady() {
		test.Error("a server is ready while its first work runs")
	}
	client.noteProgress([]byte(`{"token":"import","value":{"kind":"end"}}`))
	if !client.isReady() {
		test.Error("a server is not ready once its first work ended")
	}
}
