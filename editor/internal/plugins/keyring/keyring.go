// Package keyring keeps secrets (tokens, passwords) in the system's keyring, through the
// keyring's own program: secret-tool on Linux (the Secret Service of GNOME Keyring or KWallet),
// security on macOS. It has no dependencies; a system without such a program has no keyring here.
package keyring

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/akrck02/coddy/editor/internal/execution/programs"
	"github.com/akrck02/go-renderer/failure"
)

// answerSeconds is how long the keyring's program may take: long enough for the person to type
// the password of a locked keyring when it asks, short enough that a keyring that hangs is given up.
const answerSeconds = 60

// macNotFoundStatus is the exit status of security for a password it does not keep.
const macNotFoundStatus = 44

// The things the keyring is asked to do.
const (
	reading = iota
	writing
	deleting
)

// TODO(akrck02): Windows' Credential Manager has no program that reads a secret back; it needs
// its API (CredReadW, through syscall), and until then Windows has no keyring here.

// invocation is how the keyring's program is asked something: the program, its arguments, and
// what it reads on its standard input.
type invocation struct {
	program   string
	arguments []string
	input     string
}

// Read is the secret kept for an account of a service: NotFound when there is none.
func Read(service, account string) (string, error) {
	output, err := run(runtime.GOOS, reading, service, account, "")
	if err != nil {
		return "", err
	}
	// both programs end the secret with a line break when they print it to a terminal, and
	// security does always
	return strings.TrimSuffix(output, "\n"), nil
}

// Write keeps a secret for an account of a service, in place of the one kept before.
func Write(service, account, secret string) error {
	_, err := run(runtime.GOOS, writing, service, account, secret)
	return err
}

// Delete forgets the secret kept for an account of a service. Forgetting one never kept is not
// an error: either way, none is kept.
func Delete(service, account string) error {
	_, err := run(runtime.GOOS, deleting, service, account, "")
	if errors.Is(err, NotFound) {
		return nil
	}
	return err
}

// run asks the keyring's program of a system to do something, and gives what it printed.
func run(system string, asked int, service, account, secret string) (string, error) {
	invoked, known := invocationOf(system, asked, service, account, secret)
	if !known {
		return "", NotAvailable
	}
	program, err := programs.Find(invoked.program)
	if err != nil {
		return "", failure.Wrap(NotAvailable, invoked.program, err)
	}
	waiting, cancel := context.WithTimeout(context.Background(), answerSeconds*time.Second)
	defer cancel()
	command := exec.CommandContext(waiting, program, invoked.arguments...)
	command.Stdin = strings.NewReader(invoked.input)
	var output, errorOutput bytes.Buffer
	command.Stdout, command.Stderr = &output, &errorOutput
	if err := command.Run(); err != nil {
		return "", problemOf(system, errorOutput.String(), err)
	}
	return output.String(), nil
}

// problemOf is the error of a keyring's program that ended with one: NotFound for a secret it
// does not keep, Failed with what it wrote for anything else.
func problemOf(system, errorOutput string, err error) error {
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		// secret-tool ends with 1 and says nothing when it keeps no such secret
		silent := system == "linux" && strings.TrimSpace(errorOutput) == ""
		if silent || (system == "darwin" && exited.ExitCode() == macNotFoundStatus) {
			return NotFound
		}
	}
	return failure.Wrap(Failed, strings.TrimSpace(errorOutput), err)
}

// invocationOf is how a system's keyring program is asked to do something; known is false on a
// system without a keyring Coddy knows.
func invocationOf(system string, asked int, service, account, secret string) (invoked invocation, known bool) {
	switch system {
	case "linux":
		return secretTool(asked, service, account, secret), true
	case "darwin":
		return security(asked, service, account, secret), true
	}
	return invocation{}, false
}

// secretTool asks libsecret's secret-tool. The secret goes on its standard input, never in its
// arguments, which other programs of the computer can read.
func secretTool(asked int, service, account, secret string) invocation {
	attributes := []string{"service", service, "account", account}
	switch asked {
	case writing:
		label := "--label=" + service + " " + account
		return invocation{program: "secret-tool", arguments: append([]string{"store", label}, attributes...), input: secret}
	case deleting:
		return invocation{program: "secret-tool", arguments: append([]string{"clear"}, attributes...)}
	}
	return invocation{program: "secret-tool", arguments: append([]string{"lookup"}, attributes...)}
}

// security asks macOS's security, for a generic password of the person's login keychain.
func security(asked int, service, account, secret string) invocation {
	const program = "/usr/bin/security"
	attributes := []string{"-s", service, "-a", account}
	switch asked {
	case writing:
		// the secret is on the command line for an instant, where other programs of the computer
		// could read it: security takes it no other way without asking for it interactively
		arguments := append(append([]string{"add-generic-password", "-U"}, attributes...), "-w", secret)
		return invocation{program: program, arguments: arguments}
	case deleting:
		return invocation{program: program, arguments: append([]string{"delete-generic-password"}, attributes...)}
	}
	return invocation{program: program, arguments: append(append([]string{"find-generic-password"}, attributes...), "-w")}
}
