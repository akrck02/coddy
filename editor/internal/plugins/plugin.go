package plugins

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/coddy/sdk/jsonrpc"
	"github.com/akrck02/go-renderer/failure"
)

const (
	// startSeconds is how long a plugin's program has to answer initialize
	startSeconds = 10
	// actionSeconds is how long an action may take: printing a long file with a browser takes a
	// while
	actionSeconds = 120
)

// The states of a plugin, as the settings show them.
const (
	stateOff = "Off"
	stateOn  = "On"
)

// TODO(akrck02): a plugin that crashes starts again each time it is used; it could stop trying
// after a few crashes in a row.

// Plugin is one plugin installed: its manifest, whether it is on, and its program once started.
type Plugin struct {
	Manifest
	Folder string
	// Badge is the picture that stands for it in the settings, resolved as it is read.
	Badge Badge
	// BuiltIn is compiled into Coddy (BuiltIn): it has no folder and no program.
	BuiltIn bool
	// On is whether the person turned it on; only its frame goroutine changes it, with Turn
	On bool
	// State is what the settings show of it: off, on, or failed with its error's code. It is
	// made when it changes, not at each frame.
	State string
	// Verified is one of Coddy's own plugins, marked with a check in the settings.
	Verified bool
	// Missing are the programs it requires that are not on this computer, found by
	// CheckRequirements, which also makes the two fields after it.
	Missing []Missing
	// Installable tells whether Coddy can install one of the programs missing itself.
	Installable bool
	// Warning is what the settings warn of the plugin: its last error, the programs it misses;
	// "" when all is well.
	Warning string
	// Installing is true while Coddy installs the programs it misses.
	Installing bool
	// Values are the value of each of its settings, by id: what the person set, or else the
	// default. They are made when they are read and when one is chosen, never in a frame, and
	// replaced whole, under lock, as the program may be starting with them off the frame.
	Values map[string]any
	// failed is the code of its last error, nil while it works
	failed *failure.Code
	// settingsFile keeps the values the person chose, chosen, by setting id
	settingsFile string
	chosen       map[string]any
	// secrets keeps its secret settings: the system's keyring when nil
	secrets Keyring

	lock    sync.Mutex
	process *exec.Cmd
	conn    *jsonrpc.Conn
}

// Turn turns the plugin on or off; off stops its program.
func (plugin *Plugin) Turn(on bool) {
	plugin.On = on
	if !on {
		plugin.Stop()
	}
	plugin.State = plugin.stateWords()
	plugin.Warning = plugin.warningWords()
}

// Tell keeps the outcome of the plugin's last use for its state: an error's code, or none once it
// works again. It runs on the frame goroutine, with what Run gave back.
func (plugin *Plugin) Tell(err error) {
	if errors.Is(err, jsonrpc.RequestFailed) {
		err = nil // the plugin answered that it could not: it works
	}
	plugin.failed = failure.CodeOf(err)
	plugin.State = plugin.stateWords()
	plugin.Warning = plugin.warningWords()
}

// stateWords is the plugin's state as the settings show it.
func (plugin *Plugin) stateWords() string {
	switch {
	case !plugin.On:
		return stateOff
	case plugin.failed != nil:
		return "Failed: " + plugin.failed.Name()
	}
	return stateOn
}

// Failed tells whether the plugin's last use failed, and it is still on: its State then says the
// error's code.
func (plugin *Plugin) Failed() bool { return plugin.On && plugin.failed != nil }

// Run asks the plugin to run one of its actions, starting its program first if it is not running.
// It waits for the answer, so it runs off the frame; stop gives up on it.
func (plugin *Plugin) Run(stop context.Context, params sdk.ActionParams) (sdk.ActionResult, error) {
	var result sdk.ActionResult
	conn, err := plugin.running(stop)
	if err != nil {
		return result, err
	}
	asking, cancel := context.WithTimeout(stop, actionSeconds*time.Second)
	defer cancel()
	err = conn.Call(asking, sdk.RunActionMethod, params, &result)
	return result, plugin.problem(params.Action, err)
}

// ask calls a method of the plugin and waits readSeconds at most for its answer, starting its
// program if it is not running.
func (plugin *Plugin) ask(stop context.Context, method string, params, result any) error {
	conn, err := plugin.running(stop)
	if err != nil {
		return err
	}
	asking, cancel := context.WithTimeout(stop, readSeconds*time.Second)
	defer cancel()
	return plugin.problem(method, conn.Call(asking, method, params, result))
}

// problem is the error of a call to the plugin, with the code of what went wrong. A plugin that
// stopped answering, or did not answer in time, is stopped, to start afresh the next time.
func (plugin *Plugin) problem(method string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		plugin.Stop()
		return failure.Wrap(Timeout, plugin.Name+": "+method, err)
	case errors.Is(err, jsonrpc.Stopped), errors.Is(err, jsonrpc.BadMessage):
		plugin.Stop()
		return failure.Wrap(Crashed, plugin.Name+": "+method, err)
	}
	return err // the plugin's own answer: what it could not do, which it says itself
}

// running is the connection to the plugin's program, started and initialized when it is not
// running yet.
func (plugin *Plugin) running(stop context.Context) (*jsonrpc.Conn, error) {
	plugin.lock.Lock()
	defer plugin.lock.Unlock()
	if plugin.conn != nil {
		select {
		case <-plugin.conn.Done():
		default:
			return plugin.conn, nil
		}
	}
	settings, err := plugin.startingSettings()
	if err != nil {
		return nil, err
	}
	conn, err := plugin.start()
	if err != nil {
		return nil, err
	}
	starting, cancel := context.WithTimeout(stop, startSeconds*time.Second)
	defer cancel()
	var answer sdk.InitializeResult
	asked := sdk.InitializeParams{ProtocolVersion: sdk.ProtocolVersion, Settings: settings}
	if err := conn.Call(starting, sdk.InitializeMethod, asked, &answer); err != nil {
		plugin.stopLocked()
		return nil, failure.Wrap(NotStarted, plugin.Name, err)
	}
	if answer.ProtocolVersion != sdk.ProtocolVersion {
		plugin.stopLocked()
		return nil, failure.Wrap(ProtocolVersion, plugin.Name, nil)
	}
	return conn, nil
}

// start runs the plugin's program in its folder, its standard input and output the connection.
func (plugin *Plugin) start() (*jsonrpc.Conn, error) {
	if len(plugin.Command) == 0 {
		return nil, failure.Wrap(NotStarted, plugin.Name+" has no command", nil)
	}
	program := plugin.Command[0]
	if inFolder := filepath.Join(plugin.Folder, program); fileExists(inFolder) {
		program = inFolder
	}
	process := exec.Command(program, plugin.Command[1:]...)
	process.Dir = plugin.Folder
	process.Stderr = os.Stderr // the plugin's logs, beside Coddy's
	input, err := process.StdinPipe()
	if err != nil {
		return nil, failure.Wrap(NotStarted, plugin.Name, err)
	}
	output, err := process.StdoutPipe()
	if err != nil {
		return nil, failure.Wrap(NotStarted, plugin.Name, err)
	}
	if err := process.Start(); err != nil {
		return nil, failure.Wrap(NotStarted, plugin.Name, err)
	}
	conn := jsonrpc.NewConn(output, input, nil)
	// the process is waited for once its output ends, so every read is done before Wait
	go func() {
		<-conn.Done()
		_ = process.Wait() // how it ended is already told by the call that saw it end
	}()
	plugin.process, plugin.conn = process, conn
	return conn, nil
}

// Stop ends the plugin's program, if it runs.
func (plugin *Plugin) Stop() {
	plugin.lock.Lock()
	defer plugin.lock.Unlock()
	plugin.stopLocked()
}

// stopLocked is Stop with the lock held.
func (plugin *Plugin) stopLocked() {
	if plugin.process == nil {
		return
	}
	_ = plugin.process.Process.Kill() // fails only if it has ended already, which is what is wanted
	plugin.process, plugin.conn = nil, nil
}

// fileExists says whether a file is there.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
