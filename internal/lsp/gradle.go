package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// A server that imports Gradle builds (kotlin-lsp) leaves Gradle daemons behind, one per Gradle
// version, each waiting three hours for another build. Given a Gradle home of its own, in its data
// folder, its daemons stop after two minutes without a build, and the reader stops them with it,
// never touching those of the person's builds or IDE. That home reads the dependencies the person
// already downloaded, and the Gradle versions too, so nothing is downloaded twice.

// gradleIdle is how long a daemon of a server's Gradle home waits for another build, in milliseconds.
const gradleIdle = 120000

// gradleHome is the Gradle home of a server, in its data folder.
func gradleHome(dataFolder string) string { return filepath.Join(dataFolder, "gradle") }

// prepareGradleHome makes a server's Gradle home and gives the environment that points the server
// at it; nil when it could not be made, and the server uses the person's.
func prepareGradleHome(dataFolder string) []string {
	home := gradleHome(dataFolder)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil
	}
	properties := "# written by code-review: a daemon of its language servers stops when it is not used\n" +
		"org.gradle.daemon.idletimeout=" + strconv.Itoa(gradleIdle) + "\n"
	if os.WriteFile(filepath.Join(home, "gradle.properties"), []byte(properties), 0o644) != nil {
		return nil
	}
	environment := []string{"GRADLE_USER_HOME=" + home}
	persons := personsGradleHome()
	if _, err := os.Stat(filepath.Join(persons, "caches", "modules-2")); err == nil {
		environment = append(environment, "GRADLE_RO_DEP_CACHE="+filepath.Join(persons, "caches"))
	}
	// the Gradle versions the person's wrappers downloaded, shared rather than downloaded again
	wrapper := filepath.Join(persons, "wrapper")
	if _, err := os.Lstat(filepath.Join(home, "wrapper")); os.IsNotExist(err) {
		if _, found := os.Stat(wrapper); found == nil {
			_ = os.Symlink(wrapper, filepath.Join(home, "wrapper"))
		}
	}
	return environment
}

// personsGradleHome is the Gradle home of the person's own builds.
func personsGradleHome() string {
	if home := os.Getenv("GRADLE_USER_HOME"); home != "" {
		return home
	}
	user, _ := os.UserHomeDir()
	return filepath.Join(user, ".gradle")
}

// daemonLog is the log file a Gradle daemon writes, named by its process.
var daemonLog = regexp.MustCompile(`^daemon-(\d+)\.out\.log$`)

// daemonStopWait is how long a daemon told to stop gets before it is killed.
const daemonStopWait = 5 * time.Second

// StopGradleDaemons stops the Gradle daemons of a server's Gradle home still running, waiting for
// them, and tells how many it stopped.
func StopGradleDaemons(dataFolder string) int {
	daemons := runningDaemons(dataFolder)
	for _, process := range daemons {
		_ = syscall.Kill(process, syscall.SIGTERM)
	}
	for deadline := time.Now().Add(daemonStopWait); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if !anyRuns(daemons) {
			return len(daemons)
		}
	}
	for _, process := range daemons {
		_ = syscall.Kill(process, syscall.SIGKILL)
	}
	return len(daemons)
}

// runningDaemons are the Gradle daemons of a server's Gradle home still running. Each logs to a
// file named by its process; a process of that number that is no Gradle daemon is left out.
func runningDaemons(dataFolder string) []int {
	logs, _ := filepath.Glob(filepath.Join(gradleHome(dataFolder), "daemon", "*", "daemon-*.out.log"))
	var daemons []int
	for _, log := range logs {
		found := daemonLog.FindStringSubmatch(filepath.Base(log))
		if found == nil {
			continue
		}
		command, err := exec.Command("ps", "-o", "command=", "-p", found[1]).Output()
		if err == nil && strings.Contains(string(command), "GradleDaemon") {
			process, _ := strconv.Atoi(found[1])
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
