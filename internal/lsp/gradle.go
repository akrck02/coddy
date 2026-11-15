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
// never touching those of the person's builds or IDE. That home holds only what makes its daemons
// its own: the rest are links to the person's Gradle home, so nothing Gradle downloads or makes,
// the Gradle versions, their generated API, the jars it transforms, is made twice, and it goes on
// the person's disk.

// gradleIdle is how long a daemon of a server's Gradle home waits for another build, in milliseconds.
const gradleIdle = 120000

// sharedGradle are the folders of a server's Gradle home that are the person's: all but daemon/.
// Gradle locks what is in them file by file, across processes, as for builds run at once.
var sharedGradle = []string{"caches", "wrapper", "jdks", "native"}

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
	persons := personsGradleHome()
	if _, err := os.Stat(persons); err == nil {
		for _, name := range sharedGradle {
			linkToPersons(home, persons, name)
		}
	}
	return []string{"GRADLE_USER_HOME=" + home}
}

// linkToPersons makes a folder of a server's Gradle home a link to the person's, made there when
// missing; a folder of its own there before, from a home of the old kind, is removed: all of it
// can be made again.
func linkToPersons(home, persons, name string) {
	own, theirs := filepath.Join(home, name), filepath.Join(persons, name)
	if target, err := os.Readlink(own); err == nil && target == theirs {
		return
	}
	if os.MkdirAll(theirs, 0o755) != nil {
		return
	}
	if os.RemoveAll(own) == nil {
		_ = os.Symlink(theirs, own)
	}
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
