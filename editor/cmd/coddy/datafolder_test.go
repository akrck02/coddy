package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheOldDataFolderEntriesMoveToTheNewOneOnceAndNothingIsOverwritten(test *testing.T) {
	config := test.TempDir()
	old, folder := filepath.Join(config, "code-review"), filepath.Join(config, "coddy")
	write := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	write(filepath.Join(old, "reviewed.json"), "old reviewed")
	write(filepath.Join(old, "themes", "sea.json"), "old theme")
	write(filepath.Join(old, "preferences.json"), "old preferences")
	write(filepath.Join(folder, "preferences.json"), "new preferences")

	moveOldDataFolder(old, folder)
	moveOldDataFolder(old, folder)

	read := func(path string) string {
		content, _ := os.ReadFile(path) // a missing file reads as empty, which the checks catch
		return string(content)
	}
	if read(filepath.Join(folder, "reviewed.json")) != "old reviewed" || read(filepath.Join(folder, "themes", "sea.json")) != "old theme" {
		test.Error("the old folder's entries did not move to the new one")
	}
	if read(filepath.Join(folder, "preferences.json")) != "new preferences" {
		test.Error("an entry the new folder already had was overwritten")
	}
	if read(filepath.Join(old, "preferences.json")) != "old preferences" {
		test.Error("an entry that could not move was not left in the old folder")
	}
}
