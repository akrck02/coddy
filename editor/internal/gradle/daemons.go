package gradle

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// daemonLog is the log file a Gradle daemon writes, named by its process.
var daemonLog = regexp.MustCompile(`^daemon-(\d+)\.out\.log$`)

// daemonStopWait is how long a daemon told to stop gets before it is killed.
const daemonStopWait = 5 * time.Second

// daemonStopCheck is how often the daemons told to stop are looked at.
const daemonStopCheck = 100 * time.Millisecond

// StopDaemons stops the Gradle daemons of a server's Gradle home still running, waiting for them,
// and tells how many it stopped.
func StopDaemons(dataFolder string) int {
	daemons := runningDaemons(dataFolder)
	for _, process := range daemons {
		_ = syscall.Kill(process, syscall.SIGTERM) // one that already ended needs no stopping
	}
	for deadline := time.Now().Add(daemonStopWait); time.Now().Before(deadline); time.Sleep(daemonStopCheck) {
		if !anyRuns(daemons) {
			return len(daemons)
		}
	}
	for _, process := range daemons {
		_ = syscall.Kill(process, syscall.SIGKILL) // one that ended meanwhile needs no killing
	}
	return len(daemons)
}

// Daemons tells how many Gradle daemons of a server's own Gradle home run now.
func Daemons(dataFolder string) int { return len(runningDaemons(dataFolder)) }

// runningDaemons are the Gradle daemons of a server's Gradle home still running. Each logs to a
// file named by its process; a process of that number that is no Gradle daemon is left out.
func runningDaemons(dataFolder string) []int {
	logs, _ := filepath.Glob(filepath.Join(homeOf(dataFolder), "daemon", "*", "daemon-*.out.log")) // the pattern is fixed and valid
	var daemons []int
	for _, log := range logs {
		found := daemonLog.FindStringSubmatch(filepath.Base(log))
		if found == nil {
			continue
		}
		command, err := exec.Command("ps", "-o", "command=", "-p", found[1]).Output()
		if err == nil && strings.Contains(string(command), "GradleDaemon") {
			process, _ := strconv.Atoi(found[1]) // the pattern matched digits only
			daemons = append(daemons, process)
		}
	}
	return daemons
}

// anyRuns tells whether any of some processes still runs.
func anyRuns(processes []int) bool {
	for _, process := range processes {
		if syscall.Kill(process, 0) == nil {
			return true
		}
	}
	return false
}
