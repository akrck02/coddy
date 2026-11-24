package lsp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTheFreeSpaceOfAFolderNotMadeYetIsThatOfItsDisk(test *testing.T) {
	folder := test.TempDir()
	free, err := FreeSpace(filepath.Join(folder, "kotlin-lsp", "workspaces"))
	if err != nil || free == 0 {
		test.Errorf("free space %d (%v)", free, err)
	}
}

func TestTheIndexesOfAServerAreMeasured(test *testing.T) {
	data := test.TempDir()
	index := filepath.Join(data, "workspaces", "0123456789abcdef0123456789abcdef")
	_ = os.MkdirAll(index, 0o755)
	_ = os.WriteFile(filepath.Join(index, "lamp"), make([]byte, 1000), 0o644)
	_ = os.WriteFile(filepath.Join(data, "other"), make([]byte, 50), 0o644)
	if size := IndexesSize(data, Server{Index: Index{Folder: "workspaces"}}); size != 1000 {
		test.Errorf("the indexes take %d bytes, want 1000", size)
	}
}

func TestTheMemoryOfAProcessCountsThoseItStarted(test *testing.T) {
	command := exec.Command("/bin/bash", "-c", "/bin/sleep 30 & wait")
	if err := command.Start(); err != nil {
		test.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	time.Sleep(200 * time.Millisecond)
	own, _ := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(command.Process.Pid)).Output()
	kilobytes, _ := strconv.ParseInt(strings.TrimSpace(string(own)), 10, 64)
	if memory := MemoryOf(command.Process.Pid); memory <= kilobytes<<10 {
		test.Errorf("the process and its child take %d bytes, no more than the process alone (%d KB)", memory, kilobytes)
	}
}
