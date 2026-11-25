package sdk

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/coddy/sdk/jsonrpc"
	"github.com/akrck02/go-renderer/failure"
)

// coddy is Coddy's side of a connection to a plugin serving handlers, and the channel that gives
// what the plugin's Serve returned once Coddy closes the connection with the function given back.
func coddy(test *testing.T, handlers Handlers) (*jsonrpc.Conn, func() error) {
	pluginReads, coddyWrites := io.Pipe()
	coddyReads, pluginWrites := io.Pipe()
	served := make(chan error, 1)
	go func() { served <- ServeOver(pluginReads, pluginWrites, handlers) }()
	close := func() error {
		coddyWrites.Close()
		select {
		case err := <-served:
			return err
		case <-time.After(time.Second):
			test.Fatal("the plugin kept serving after Coddy closed")
			return nil
		}
	}
	return jsonrpc.NewConn(coddyReads, coddyWrites, nil), close
}

// noFile is the error of a plugin's own, as a plugin declares them.
var noFile = failure.Define("lamp.no-file", "the file is not there")

// within is a context that gives up after a second, so a broken test fails instead of hanging.
func within(test *testing.T) context.Context {
	asking, cancel := context.WithTimeout(context.Background(), time.Second)
	test.Cleanup(cancel)
	return asking
}

func TestAPluginAnswersInitializeAndItsActionsUntilCoddyCloses(test *testing.T) {
	run := func(params ActionParams) (ActionResult, error) {
		return ActionResult{Note: "ran " + params.Action + " on " + params.File}, nil
	}
	conn, close := coddy(test, Handlers{RunAction: run})
	var started InitializeResult
	err := conn.Call(within(test), InitializeMethod, InitializeParams{ProtocolVersion: ProtocolVersion}, &started)
	if err != nil || started.ProtocolVersion != ProtocolVersion {
		test.Errorf("initialize answered %+v (%v)", started, err)
	}
	var done ActionResult
	err = conn.Call(within(test), RunActionMethod, ActionParams{Action: "count", File: "/lamp.go"}, &done)
	if err != nil || done.Note != "ran count on /lamp.go" {
		test.Errorf("the action answered %+v (%v)", done, err)
	}
	if err := close(); err != nil {
		test.Errorf("serving ended with %v, want nil once Coddy closes", err)
	}
}

func TestARequestThePluginDoesNotAnswerIsAnErrorReplyNotAnEnd(test *testing.T) {
	failing := func(ActionParams) (ActionResult, error) { return ActionResult{}, failure.Wrap(noFile, "lamp.go", nil) }
	conn, close := coddy(test, Handlers{RunAction: failing})
	if err := conn.Call(within(test), "tree/list", nil, nil); !errors.Is(err, jsonrpc.RequestFailed) {
		test.Errorf("an unknown request gave %v", err)
	}
	err := conn.Call(within(test), RunActionMethod, ActionParams{Action: "count"}, nil)
	// the plugin's error crosses the pipe as words only, so its code is looked for by its name
	if !errors.Is(err, jsonrpc.RequestFailed) || !strings.Contains(err.Error(), noFile.Name()) {
		test.Errorf("a failed action gave %v, want its error", err)
	}
	if err := close(); err != nil {
		test.Errorf("serving ended with %v", err)
	}
}

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, unknown := resultOf(Handlers{}, "tree/list", nil)
	run := func(ActionParams) (ActionResult, error) { return ActionResult{}, nil }
	_, badParams := resultOf(Handlers{RunAction: run}, RunActionMethod, []byte(`"not an object"`))
	if !failure.Is(unknown, jsonrpc.UnknownMethod) {
		test.Errorf("a request not answered gave %v", unknown)
	}
	if !failure.Is(badParams, jsonrpc.BadMessage) {
		test.Errorf("parameters that cannot be read gave %v", badParams)
	}
}

// A plugin is configured by its settings, which only initialize carries, so Initialize must be
// given them before any other request.
func TestAPluginIsGivenItsSettingsAsItStarts(test *testing.T) {
	var given map[string]any
	conn, close := coddy(test, Handlers{Initialize: func(settings map[string]any) { given = settings }})
	asked := InitializeParams{ProtocolVersion: ProtocolVersion, Settings: map[string]any{"host": "lamps.example.com", "dim": true}}
	var started InitializeResult
	if err := conn.Call(within(test), InitializeMethod, asked, &started); err != nil || started.ProtocolVersion != ProtocolVersion {
		test.Fatalf("initialize answered %+v (%v)", started, err)
	}
	if given["host"] != "lamps.example.com" || given["dim"] != true {
		test.Errorf("the plugin was given %v", given)
	}
	if err := close(); err != nil {
		test.Errorf("serving ended with %v", err)
	}
}
