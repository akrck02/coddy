package gradle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

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
	if stopped := StopDaemons(data); stopped != 1 {
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
