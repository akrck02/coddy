package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
