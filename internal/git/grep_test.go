package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// gitRepository makes a repository holding one committed file.
func gitRepository(test *testing.T) string {
	test.Helper()
	folder := test.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "lamp.txt"), []byte("the lamp is on\n"), 0o644); err != nil {
		test.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "."},
		{"-c", "user.name=keeper", "-c", "user.email=keeper@example.com", "commit", "-q", "-m", "lamp"}} {
		command := exec.Command("git", arguments...)
		command.Dir = folder
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v %s", arguments, err, output)
		}
	}
	return folder
}

// Finding nothing is not a failure, but a folder git cannot search is: the two must not be confused.
func TestASearchThatFindsNothingIsEmptyAndOneThatFailsSaysSo(test *testing.T) {
	repository := gitRepository(test)
	hits, err := Grep(context.Background(), repository, "beacon", GrepOptions{})
	if err != nil || len(hits) != 0 {
		test.Errorf("searching for what is not there gave %v, %v", hits, err)
	}
	if _, err := Grep(context.Background(), test.TempDir(), "lamp", GrepOptions{}); !errors.Is(err, CommandFailed) {
		test.Errorf("searching a folder that is no repository gave %v", err)
	}
	if hits, err := Grep(context.Background(), repository, "lamp", GrepOptions{}); err != nil || len(hits) != 1 {
		test.Errorf("searching for the lamp gave %v, %v", hits, err)
	}
}

// A search no longer wanted stops its git process instead of running to its end.
func TestACancelledSearchStops(test *testing.T) {
	stop, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Grep(stop, gitRepository(test), "lamp", GrepOptions{}); !errors.Is(err, context.Canceled) {
		test.Errorf("a cancelled search gave %v", err)
	}
}

// Reading a big project takes these commands longest; a reading replaced by a newer one stops them.
func TestTheLongestCommandsStopWhenTheirWorkIsCancelled(test *testing.T) {
	repository := gitRepository(test)
	stop, cancel := context.WithCancel(context.Background())
	cancel()
	comparison := Comparison{MergeBase: "HEAD", WorkingTree: true}
	_, listing := TrackedAndNewFiles(stop, repository)
	_, changes := Changes(stop, repository, comparison)
	_, counting := CountLines(stop, repository, comparison)
	_, blobs := BlobsAt(stop, repository, "HEAD")
	for name, err := range map[string]error{"ls-files": listing, "diff": changes, "numstat": counting, "ls-tree": blobs} {
		if !errors.Is(err, context.Canceled) {
			test.Errorf("a cancelled %s gave %v", name, err)
		}
	}
}

// How long the fake git sleeps, and how soon a cancelled one must have stopped: far less.
const (
	fakeGitSeconds = 30
	stoppedWithin  = 5 * time.Second
	cancelAfter    = 100 * time.Millisecond
)

// sleepingGit puts first on the PATH a git that only sleeps, as a search of a huge repository does.
func sleepingGit(test *testing.T) {
	folder := test.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nexec sleep %d\n", fakeGitSeconds)
	if err := os.WriteFile(filepath.Join(folder, "git"), []byte(script), 0o755); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A search cancelled while git runs kills git, and gives back at once, not when git would end.
func TestASearchCancelledWhileItRunsStopsItsGit(test *testing.T) {
	if runtime.GOOS == "windows" {
		test.Skip("the fake git is a shell script")
	}
	sleepingGit(test)
	stop, cancel := context.WithCancel(context.Background())
	time.AfterFunc(cancelAfter, cancel)
	started := time.Now()
	_, err := Grep(stop, test.TempDir(), "lamp", GrepOptions{})
	if !errors.Is(err, context.Canceled) {
		test.Errorf("a search cancelled while it ran gave %v", err)
	}
	if took := time.Since(started); took > stoppedWithin {
		test.Errorf("a search cancelled after %v gave back after %v; its git kept running", cancelAfter, took)
	}
}
