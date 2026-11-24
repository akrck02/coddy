// Package gradle knows what Coddy does around Gradle builds for the language servers that import
// them (jdtls, kotlin-lsp): a Gradle home of their own whose daemons it stops, the builds that
// cannot be imported, and the models of the projects kept between starts. It speaks no protocol:
// lsp calls it for a server that asks for it.
package gradle

import (
	"os"
	"path/filepath"
	"strconv"
)

// A server that imports Gradle builds leaves Gradle daemons behind, one per Gradle version, each
// waiting three hours for another build. Given a Gradle home of its own, in its data folder, its
// daemons are the reader's: they wait long enough to serve the whole import (jdtls goes through
// the builds one by one, coming back to a Gradle version minutes later), the reader stops them
// once the server has imported, and with the server, never touching those of the person's builds
// or IDE. That home holds only what makes its daemons its own: the rest are links to the person's
// Gradle home, so nothing Gradle downloads or makes, the Gradle versions, their generated API, the
// jars it transforms, is made twice, and it goes on the person's disk.

// idleMilliseconds is how long a daemon of a server's Gradle home waits for another build: longer
// than an import of every project takes, so a daemon is started once per Gradle version. Two
// minutes, as before, restarted daemons in the middle of jdtls's import on the big workspace.
const idleMilliseconds = 30 * 60 * 1000

// sharedFolders are the folders of a server's Gradle home that are the person's: all but daemon/.
// Gradle locks what is in them file by file, across processes, as for builds run at once.
var sharedFolders = []string{"caches", "wrapper", "jdks", "native"}

// homeOf is the Gradle home of a server, in its data folder.
func homeOf(dataFolder string) string { return filepath.Join(dataFolder, "gradle") }

// PrepareHome makes a server's Gradle home and gives the environment that points the server at
// it; nil when it could not be made, and the server uses the person's.
func PrepareHome(dataFolder string) []string {
	home := homeOf(dataFolder)
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil
	}
	properties := "# written by code-review: a daemon of its language servers stops when it is not used\n" +
		"org.gradle.daemon.idletimeout=" + strconv.Itoa(idleMilliseconds) + "\n"
	if os.WriteFile(filepath.Join(home, "gradle.properties"), []byte(properties), 0o644) != nil {
		return nil
	}
	persons := personsHome()
	if _, err := os.Stat(persons); err == nil {
		for _, name := range sharedFolders {
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
		_ = os.Symlink(theirs, own) // without the link, Gradle makes the folder again in the server's home
	}
}

// personsHome is the Gradle home of the person's own builds.
func personsHome() string {
	if home := os.Getenv("GRADLE_USER_HOME"); home != "" {
		return home
	}
	user, _ := os.UserHomeDir() // with no home, ".gradle" from the working folder: Gradle does the same
	return filepath.Join(user, ".gradle")
}
