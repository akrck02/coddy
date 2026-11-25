package main

import (
	"log"
	"os"
	"path/filepath"
)

// moveOldDataFolder moves what Coddy kept under its old name into its folder now. An entry the new
// folder already has is left in the old one and told; the old folder goes once it is empty.
// TODO(akrck02): remove a few releases after the rename, once every copy has moved.
func moveOldDataFolder(old, folder string) {
	entries, err := os.ReadDir(old)
	if err != nil {
		return // no old folder: nothing to move
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		log.Printf("the data folder %s could not be made: %v", folder, err)
		return
	}
	for _, entry := range entries {
		target := filepath.Join(folder, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			log.Printf("%s is kept in %s: %s already has one", entry.Name(), old, folder)
			continue
		}
		if err := os.Rename(filepath.Join(old, entry.Name()), target); err != nil {
			log.Printf("%s could not be moved to %s: %v", entry.Name(), folder, err)
		}
	}
	_ = os.Remove(old) // fails only while entries are left, which were told above
}
