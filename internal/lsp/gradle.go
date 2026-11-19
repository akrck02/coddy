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

// A server that imports Gradle builds (jdtls, kotlin-lsp) leaves Gradle daemons behind, one per
// Gradle version, each waiting three hours for another build. Given a Gradle home of its own, in
// its data folder, its daemons are the reader's: they wait long enough to serve the whole import
// (jdtls goes through the builds one by one, coming back to a Gradle version minutes later), the
// reader stops them once the server has imported (releaseGradle), and with the server, never
// touching those of the person's builds or IDE. That home holds only what makes its daemons
// its own: the rest are links to the person's Gradle home, so nothing Gradle downloads or makes,
// the Gradle versions, their generated API, the jars it transforms, is made twice, and it goes on
// the person's disk.

// gradleIdle is how long a daemon of a server's Gradle home waits for another build, in milliseconds:
// longer than an import of every project takes, so a daemon is started once per Gradle version.
// Two minutes, as before, restarted daemons in the middle of jdtls's import on the big workspace.
const gradleIdle = 30 * 60 * 1000

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

// Once a server has imported the projects, its Gradle daemons only wait, half a gigabyte or more
// each, for an import that may never come: the reader stops them, and a later import starts them
// again.
const (
	// gradleCheck is how often a server is looked at for daemons to stop
	gradleCheck = 5 * time.Second
	// gradleSettle is how long a server must have been ready, with no work running, before its
	// daemons are stopped: between two steps of an import it may report none for a moment
	gradleSettle = 20 * time.Second
)

// releaseGradle stops the Gradle daemons of a server each time it is done importing, until it ends.
func (client *Client) releaseGradle() {
	ticker := time.NewTicker(gradleCheck)
	defer ticker.Stop()
	importing := true // a server imports as it starts
	for {
		select {
		case <-client.exited:
			return
		case <-ticker.C:
		}
		if client.Busy() {
			importing = true
			continue
		}
		if importing && client.settled(time.Now()) {
			importing = false
			stopped := StopGradleDaemons(client.data)
			client.lock.Lock()
			client.released, client.releasedAt = stopped, time.Now()
			client.lock.Unlock()
			client.changed()
		}
	}
}

// settled tells whether a server is ready, with no work running, and has been for gradleSettle,
// nor begun importing a project meanwhile: kotlin-lsp is ready as it starts, and imports before
// it reports any work.
func (client *Client) settled(now time.Time) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	if !client.isReady() || len(client.working) > 0 {
		return false
	}
	quiet := later(client.readyAt, client.idleAt)
	for _, began := range client.imported {
		quiet = later(quiet, began)
	}
	return now.Sub(quiet) >= gradleSettle
}

// GradleDaemons tells how many Gradle daemons of a server's own Gradle home run now.
func GradleDaemons(dataFolder string) int { return len(runningDaemons(dataFolder)) }

// Released are the Gradle daemons the reader stopped once the server was done importing last, and
// when; none before.
func (client *Client) Released() (int, time.Time) {
	client.lock.Lock()
	defer client.lock.Unlock()
	return client.released, client.releasedAt
}
