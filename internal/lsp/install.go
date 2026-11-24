package lsp

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

// Find is where a server's command is: on the PATH, or in a folder where installers put commands
// that a window opened from the desktop may not have on its PATH (Go's, Homebrew's).
func Find(server Server) (string, error) {
	path, err := exec.LookPath(server.Command)
	if err == nil {
		return path, nil
	}
	for _, folder := range installFolders() {
		candidate := filepath.Join(folder, server.Command)
		if found, lookErr := exec.LookPath(candidate); lookErr == nil {
			return found, nil
		}
	}
	return "", failure.Wrap(ServerNotFound, server.Name, err)
}

// installFolders are the folders installers put commands in: go install's, and Homebrew's.
func installFolders() []string {
	var folders []string
	if bin := os.Getenv("GOBIN"); bin != "" {
		folders = append(folders, bin)
	}
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, _ := os.UserHomeDir() // without a home, go install has no folder to put commands in
		goPath = filepath.Join(home, "go")
	}
	for _, each := range filepath.SplitList(goPath) {
		folders = append(folders, filepath.Join(each, "bin"))
	}
	return append(folders, "/opt/homebrew/bin", "/usr/local/bin", "/home/linuxbrew/.linuxbrew/bin")
}

// Installable tells whether a server can be installed from here: it says how, and the program
// its install command runs (go, brew) is on this computer.
func Installable(server Server) bool {
	words := strings.Fields(server.Install)
	if len(words) == 0 {
		return false
	}
	_, err := Find(Server{Name: words[0], Command: words[0]})
	return err == nil
}

// Install runs a server's install command, without a shell, and gives InstallFailed with the end
// of what it wrote when it cannot run or ends with an error.
func Install(ctx context.Context, server Server) error {
	words := strings.Fields(server.Install)
	if len(words) == 0 {
		return failure.Wrap(InstallFailed, server.Name, nil)
	}
	program, err := Find(Server{Name: words[0], Command: words[0]})
	if err != nil {
		return failure.Wrap(InstallFailed, server.Name, err)
	}
	command := exec.CommandContext(ctx, program, words[1:]...)
	command.Env = append(os.Environ(), server.InstallEnvironment...)
	output, err := command.CombinedOutput()
	if err != nil {
		return failure.Wrap(InstallFailed, server.Name+": "+lastLines(string(output), installOutputLines), err)
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
