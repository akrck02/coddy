// Package programs finds the programs Coddy and its plugins run, and installs a missing one with
// the command its plugin gives.
package programs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/akrck02/go-renderer/failure"
)

// installOutputLines is how many of the last lines of a failed install are kept to say why
const installOutputLines = 3

// administrator is the command that runs another as the administrator, asking for a password
const administrator = "sudo"

// Find is where a program is: on the PATH, or in a folder where installers put programs that a
// window opened from the desktop may not have on its PATH (Go's, Homebrew's).
func Find(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	for _, folder := range installFolders() {
		if found, lookErr := exec.LookPath(filepath.Join(folder, name)); lookErr == nil {
			return found, nil
		}
	}
	return "", failure.Wrap(NotFound, name, err)
}

// installFolders are the folders installers put programs in: go install's, and Homebrew's.
func installFolders() []string {
	var folders []string
	if bin := os.Getenv("GOBIN"); bin != "" {
		folders = append(folders, bin)
	}
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, _ := os.UserHomeDir() // without a home, go install has no folder to put programs in
		goPath = filepath.Join(home, "go")
	}
	for _, each := range filepath.SplitList(goPath) {
		folders = append(folders, filepath.Join(each, "bin"))
	}
	return append(folders, "/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin")
}

// AsksForPassword tells whether an install command runs as the administrator, and so asks for a
// password: "sudo dnf install pandoc".
func AsksForPassword(command string) bool { return strings.HasPrefix(command, administrator+" ") }

// Installer is the program an install command runs to install: its first word, or the one after
// sudo ("dnf" in "sudo dnf install pandoc").
func Installer(command string) string {
	words := strings.Fields(command)
	if len(words) > 1 && words[0] == administrator {
		return words[1]
	}
	if len(words) == 0 {
		return ""
	}
	return words[0]
}

// Choose is the install command for this computer among those given: the first whose installer
// is here (dnf on one Linux, apt on another), else the first, to be shown; "" when none is given.
func Choose(commands []string) string {
	for _, command := range commands {
		if _, err := Find(Installer(command)); err == nil {
			return command
		}
	}
	if len(commands) == 0 {
		return ""
	}
	return commands[0]
}

// Runnable tells whether Coddy can run an install command itself: its installer is here, and it
// asks for no password, which a window has no terminal to type in. One that does is shown to be
// run by hand.
func Runnable(command string) bool {
	if AsksForPassword(command) || Installer(command) == "" {
		return false
	}
	_, err := Find(Installer(command))
	return err == nil
}

// Install runs an install command, without a shell, with environment added to Coddy's own
// ("GOTOOLCHAIN=auto"). It gives InstallFailed with the end of what it wrote when it cannot run
// or ends with an error.
func Install(stop context.Context, command string, environment []string) error {
	words := strings.Fields(command)
	if len(words) == 0 {
		return failure.Wrap(InstallFailed, "no install command", nil)
	}
	program, err := Find(words[0])
	if err != nil {
		return failure.Wrap(InstallFailed, command, err)
	}
	running := exec.CommandContext(stop, program, words[1:]...)
	running.Env = append(os.Environ(), environment...)
	output, err := running.CombinedOutput()
	if err != nil {
		return failure.Wrap(InstallFailed, command+": "+lastLines(string(output), installOutputLines), err)
	}
	return nil
}

// lastLines are the last lines of a text that are not empty, on one line.
func lastLines(text string, count int) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return strings.Join(lines[max(len(lines)-count, 0):], " · ")
}
