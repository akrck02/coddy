package programs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAProgramInstalledWithGoIsFoundOffThePath(test *testing.T) {
	bin := test.TempDir()
	program := filepath.Join(bin, "lighthouse-server")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", test.TempDir())
	test.Setenv("GOBIN", bin)
	found, err := Find("lighthouse-server")
	if err != nil || found != program {
		test.Errorf("a program in GOBIN is found at %q, %v", found, err)
	}
}

func TestTheInstallChosenIsTheFirstWhoseInstallerIsHere(test *testing.T) {
	commands := []string{"no-such-installer-here install lamp", "sudo sh -c true", "sh -c true"}
	if got := Choose(commands); got != "sudo sh -c true" {
		test.Errorf("chose %q, want the first whose installer, sh, is here", got)
	}
	if got := Choose([]string{"no-such-installer-here install lamp"}); got != "no-such-installer-here install lamp" {
		test.Errorf("with no installer here, chose %q, want the first, to be shown", got)
	}
	if Choose(nil) != "" {
		test.Error("chose an install where none was given")
	}
}

// A window has no terminal to type a password in, so what asks for one is left to the person.
func TestCoddyRunsOnlyInstallsWhoseInstallerIsHereAndAskNoPassword(test *testing.T) {
	cases := map[string]bool{
		"sh -c true":                        true,
		"sudo sh -c true":                   false,
		"no-such-installer-here install it": false,
		"":                                  false,
	}
	for command, want := range cases {
		if got := Runnable(command); got != want {
			test.Errorf("%q runnable is %v, want %v", command, got, want)
		}
	}
}

func TestAnInstallThatEndsInAnErrorIsInstallFailed(test *testing.T) {
	if err := Install(context.Background(), "sh -c false", nil); !errors.Is(err, InstallFailed) {
		test.Errorf("a failed install gave %v", err)
	}
	if err := Install(context.Background(), "sh -c true", nil); err != nil {
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
	if err := Install(context.Background(), "printenv LIGHTHOUSE_LAMP_LIT", []string{"LIGHTHOUSE_LAMP_LIT=yes"}); err != nil {
		test.Errorf("the install did not see its environment: %v", err)
	}
}

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, notFound := Find("no-such-program-here")
	failed := Install(context.Background(), "", nil)
	if !errors.Is(notFound, NotFound) || !errors.Is(failed, InstallFailed) {
		test.Errorf("the errors are %v and %v", notFound, failed)
	}
}
