package keyring

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// fakeSecretTool puts on the PATH a secret-tool that writes down its arguments and what it read,
// and keeps one secret, for the account "keeper"; it fails, saying why, for the account "locked".
// It gives the file of what it wrote down.
func fakeSecretTool(test *testing.T) (written string) {
	test.Helper()
	if runtime.GOOS != "linux" {
		test.Skip("secret-tool is the keyring of Linux")
	}
	folder := test.TempDir()
	written = filepath.Join(folder, "written")
	script := `#!/bin/sh
echo "$@" > "` + written + `"
cat >> "` + written + `"
case "$*" in
*"account locked"*) echo "the collection is locked" >&2; exit 1 ;;
"lookup service lighthouse account keeper") printf 'lamp oil' ;;
lookup*) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(folder, "secret-tool"), []byte(script), 0o755); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", folder+string(os.PathListSeparator)+os.Getenv("PATH"))
	return written
}

// readWritten is what the fake secret-tool wrote down: its arguments on the first line, then what
// it read on its standard input.
func readWritten(test *testing.T, written string) string {
	test.Helper()
	content, err := os.ReadFile(written)
	if err != nil {
		test.Fatal(err)
	}
	return string(content)
}

// A secret on the command line can be read by any program of the computer, so secret-tool is
// given it on its standard input.
func TestOnLinuxASecretIsGivenToSecretToolOnItsInputNeverInItsArguments(test *testing.T) {
	written := fakeSecretTool(test)
	if err := Write("lighthouse", "keeper", "lamp oil"); err != nil {
		test.Fatal(err)
	}
	want := "store --label=lighthouse keeper service lighthouse account keeper\nlamp oil"
	if got := readWritten(test, written); got != want {
		test.Errorf("secret-tool was given %q, want %q", got, want)
	}
}

func TestOnLinuxASecretKeptIsReadBackAndOneNotKeptIsNotFound(test *testing.T) {
	fakeSecretTool(test)
	secret, err := Read("lighthouse", "keeper")
	if err != nil || secret != "lamp oil" {
		test.Errorf("the secret kept is %q (%v)", secret, err)
	}
	if _, err := Read("lighthouse", "harbour"); !errors.Is(err, NotFound) {
		test.Errorf("a secret not kept gave %v, want keyring.not-found", err)
	}
	if _, err := Read("lighthouse", "locked"); !errors.Is(err, Failed) || !strings.Contains(err.Error(), "the collection is locked") {
		test.Errorf("a keyring that fails gave %v, want keyring.failed with what it said", err)
	}
}

func TestOnLinuxASecretIsForgottenWithSecretToolsClear(test *testing.T) {
	written := fakeSecretTool(test)
	if err := Delete("lighthouse", "keeper"); err != nil {
		test.Fatal(err)
	}
	if got := readWritten(test, written); got != "clear service lighthouse account keeper\n" {
		test.Errorf("secret-tool was asked %q", got)
	}
}

// security takes the secret only on its command line, or else asks for it in a terminal.
func TestOnMacOSTheLoginKeychainIsAskedWithSecurity(test *testing.T) {
	cases := []struct {
		asked int
		want  []string
	}{
		{writing, []string{"add-generic-password", "-U", "-s", "lighthouse", "-a", "keeper", "-w", "lamp oil"}},
		{reading, []string{"find-generic-password", "-s", "lighthouse", "-a", "keeper", "-w"}},
		{deleting, []string{"delete-generic-password", "-s", "lighthouse", "-a", "keeper"}},
	}
	for _, each := range cases {
		invoked, known := invocationOf("darwin", each.asked, "lighthouse", "keeper", "lamp oil")
		if !known || invoked.program != "/usr/bin/security" || !slices.Equal(invoked.arguments, each.want) {
			test.Errorf("security is asked %+v, want %v", invoked, each.want)
		}
	}
}

func TestASecretNotKeptOnMacOSIsNotFound(test *testing.T) {
	// security ends with 44 for a password it does not keep; any program that does stands for it
	if problem := problemOf("darwin", "", exitWith(test, macNotFoundStatus)); !errors.Is(problem, NotFound) {
		test.Errorf("security ending with 44 gave %v, want keyring.not-found", problem)
	}
	if problem := problemOf("darwin", "denied", exitWith(test, 1)); !errors.Is(problem, Failed) {
		test.Errorf("security ending with 1 gave %v, want keyring.failed", problem)
	}
}

func TestASystemWithoutAKeyringProgramHasNoKeyring(test *testing.T) {
	if _, err := run("windows", reading, "lighthouse", "keeper", ""); !errors.Is(err, NotAvailable) {
		test.Errorf("Windows gave %v, want keyring.not-available", err)
	}
	test.Setenv("PATH", test.TempDir())
	test.Setenv("GOBIN", "")
	test.Setenv("GOPATH", test.TempDir())
	if _, err := run("linux", writing, "lighthouse", "keeper", "lamp oil"); !errors.Is(err, NotAvailable) {
		test.Errorf("Linux without secret-tool gave %v, want keyring.not-available", err)
	}
}

// exitWith is the error of a program that ended with a status.
func exitWith(test *testing.T, status int) error {
	test.Helper()
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(status)).Run()
	if err == nil {
		test.Fatal("sh did not end with an error")
	}
	return err
}
