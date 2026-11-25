// The fake plugin answers as a plugin does, for the tests: each action does one thing a real
// plugin may, by its id. It is a version control too (see versioncontrol.go), a review source
// (review.go) and a mode with a screen of its own (screen.go). With the argument "other-version"
// it answers initialize with another version of the protocol, and nothing more. The action
// "greeting" notes the value of its setting "greeting", as initialize gave it.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/coddy/sdk/jsonrpc"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "other-version" {
		answerAnotherVersion()
		return
	}
	if err := sdk.Serve(sdk.Handlers{Initialize: keep, RunAction: run, VersionControl: fakeControl{}, ReviewSource: fakeReviews{}, DrawMode: newFakeScreen().Answer}); err != nil {
		log.Fatal(err)
	}
}

// settings are the values initialize gave
var settings map[string]any

func keep(given map[string]any) { settings = given }

func run(params sdk.ActionParams) (sdk.ActionResult, error) {
	switch params.Action {
	case "greeting":
		return sdk.ActionResult{Note: fmt.Sprint(settings["greeting"])}, nil
	case "note":
		return sdk.ActionResult{Note: "noted " + params.File}, nil
	case "make":
		made := params.File + ".out"
		if err := os.WriteFile(made, []byte("made"), 0o644); err != nil {
			return sdk.ActionResult{}, err
		}
		return sdk.ActionResult{Created: made}, nil
	case "crash":
		os.Exit(3)
	case "slow":
		time.Sleep(time.Minute)
	}
	return sdk.ActionResult{}, jsonrpc.UnknownMethod
}

// answerAnotherVersion reads the first request, initialize, and answers it with version 2.
func answerAnotherVersion() {
	reader := bufio.NewReader(os.Stdin)
	length := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if value, found := strings.CutPrefix(line, "Content-Length: "); found {
			length, _ = strconv.Atoi(value) // the tests send a valid header
		}
	}
	if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
		return
	}
	body := `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":2}}`
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, _ = io.Copy(io.Discard, reader) // waits for Coddy to close, as a plugin does
}
