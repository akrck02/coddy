package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gomono"
)

func TestTheCodeIsReadInTheFaceAskedForThenTheSystemsThenGoMono(test *testing.T) {
	asked := filepath.Join(test.TempDir(), "Asked.ttf")
	if err := os.WriteFile(asked, []byte("asked"), 0o644); err != nil {
		test.Fatal(err)
	}
	system := []byte("system")
	if face := monospaced(asked, system); string(face) != "asked" {
		test.Errorf("the code is read in %q, not in the face asked for", face)
	}
	if face := monospaced("", system); string(face) != "system" {
		test.Errorf("the code is read in %q, not in the system's face", face)
	}
	if face := monospaced("", nil); &face[0] != &gomono.TTF[0] {
		test.Error("without a system face the code is not read in Go Mono")
	}
}
