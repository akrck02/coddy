package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAFolderNamedByADotIsOpenedByItsFullPath(test *testing.T) {
	here, err := os.Getwd()
	if err != nil {
		test.Fatal(err)
	}
	folders := absolutes([]string{".", "other"})
	if folders[0] != here || folders[1] != filepath.Join(here, "other") {
		test.Errorf("the folders are %q, want them under %q", folders, here)
	}
}
