package lsp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAServerInstalledWithGoIsFoundOffThePath(test *testing.T) {
	bin := test.TempDir()
	command := filepath.Join(bin, "lighthouse-server")
	if err := os.WriteFile(command, []byte("#!/bin/sh\n"), 0o755); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", test.TempDir())
	test.Setenv("GOBIN", bin)
	found, err := Find(Server{Name: "lighthouse", Command: "lighthouse-server"})
	if err != nil || found != command {
		test.Errorf("a server in GOBIN is found at %q, %v", found, err)
	}
}

func TestAServerIsInstallableOnlyWhereItsInstallerIs(test *testing.T) {
	if !Installable(Server{Install: "sh -c true"}) {
		test.Error("a server installed with sh is not installable, though sh is here")
	}
	if Installable(Server{Install: "no-such-installer-here install it"}) || Installable(Server{}) {
		test.Error("a server is installable without its installer, or without an install command")
	}
}

func TestAnInstallThatEndsInAnErrorIsInstallFailed(test *testing.T) {
	if err := Install(context.Background(), Server{Name: "lighthouse", Install: "sh -c false"}); !errors.Is(err, InstallFailed) {
		test.Errorf("a failed install gave %v", err)
	}
	if err := Install(context.Background(), Server{Name: "lighthouse", Install: "sh -c true"}); err != nil {
		test.Errorf("an install that works gave %v", err)
	}
}

func TestAFailedInstallIsToldByTheLastLinesItWrote(test *testing.T) {
	output := "downloading lamp\n\nbuilding lamp\nno wick found\nexit status 1\n"
	if got := lastLines(output, 2); got != "no wick found · exit status 1" {
		test.Errorf("the last lines are %q", got)
	}
}

func TestAnInstallRunsWithItsOwnEnvironment(test *testing.T) {
	// printenv fails when the variable is not in its environment
	server := Server{Name: "lighthouse", Install: "printenv LIGHTHOUSE_LAMP_LIT", InstallEnvironment: []string{"LIGHTHOUSE_LAMP_LIT=yes"}}
	if err := Install(context.Background(), server); err != nil {
		test.Errorf("the install did not see its environment: %v", err)
	}
}
