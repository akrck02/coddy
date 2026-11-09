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
	if !strings.Contains(string(properties), "org.gradle.daemon.idletimeout=120000") {
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
