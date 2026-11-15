package lsp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/go-renderer/failure"
)

// Progress is what a server says it is doing, to show the person: its phase, on what, how far,
// since when, and its last messages.
type Progress struct {
	Phase   string    // "starting", "importing", "indexing", or the server's own word; "ready" when done
	Detail  string    // what it works on, as it says: a project, a file
	Percent int       // how far, from 0 to 100; -1 when it does not say
	Done    int       // projects imported, of Total; 0 of 0 when it does not say
	Total   int       //
	Since   time.Time // when the phase began
	Started time.Time // when the server was started
	Log     []Message // its last messages, oldest first
	// Projects are the workspace folders it reads, with what it says of each
	Projects []ProjectProgress
}

// ProjectProgress is how far a server is with one project: "waiting", "importing", "imported",
// "reading" or "read", and how long it took, or takes so far.
type ProjectProgress struct {
	Path  string
	State string
	Took  time.Duration
}

// Message is a line a server logged or showed.
type Message struct {
	At    time.Time
	Error bool
	Text  string
}

// logLength is how many of a server's messages are kept.
const logLength = 50

// work is one piece of work a server reports by $/progress.
type work struct {
	title, message string
	percent        int
	since          time.Time
}

// progressNote is the $/progress a server sends: a begin, a report or an end of a piece of work.
type progressNote struct {
	Token json.RawMessage `json:"token"`
	Value struct {
		Kind       string `json:"kind"`
		Title      string `json:"title"`
		Message    string `json:"message"`
		Percentage *int   `json:"percentage"`
	} `json:"value"`
}

// noteProgress follows the work a server reports ($/progress): a begin starts it, a report moves
// it on, an end ends it. While some runs (Gradle's import, indexing) the server may answer with nothing.
func (client *Client) noteProgress(params json.RawMessage) {
	var progress progressNote
	if json.Unmarshal(params, &progress) != nil {
		return
	}
	token, value := string(progress.Token), progress.Value
	client.lock.Lock()
	defer client.lock.Unlock()
	defer client.changed()
	switch value.Kind {
	case "begin":
		client.working[token] = &work{title: value.Title, message: value.Message, percent: -1, since: time.Now()}
	case "end":
		delete(client.working, token)
		if len(client.working) == 0 {
			client.idleAt = time.Now()
		}
		return
	}
	running := client.working[token]
	if running == nil {
		return
	}
	if value.Message != "" {
		running.message = value.Message
	}
	if value.Percentage != nil {
		running.percent = *value.Percentage
	} else if percent, found := percentIn(value.Message); found {
		running.percent = percent
	}
}

// percentIn reads a percentage a server gives as words: "45%", "45 % done".
func percentIn(words string) (int, bool) {
	number, rest, found := strings.Cut(strings.TrimSpace(words), "%")
	if !found || strings.TrimSpace(rest) != "" && !strings.HasPrefix(rest, " ") {
		return 0, false
	}
	percent, err := strconv.Atoi(strings.TrimSpace(number))
	return percent, err == nil && percent >= 0 && percent <= 100
}

// noteStatus keeps what jdtls says of itself (language/status): "Starting", "Importing Gradle
// project(s)", "45% Starting Java Language Server", until it says it is ready.
func (client *Client) noteStatus(params json.RawMessage) {
	var status struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(params, &status) != nil {
		return
	}
	if status.Type == client.Server.ReadyStatus && status.Type != "" {
		client.becomeReady()
	}
	client.lock.Lock()
	defer client.lock.Unlock()
	if status.Message != "" && status.Message != client.status {
		client.status, client.statusSince = status.Message, time.Now()
	}
	client.changed()
}

// noteImport keeps the projects kotlin-lsp imports (intellij/importLog: "Importing with Gradle:
// <folder>"), to say which and how many.
func (client *Client) noteImport(params json.RawMessage) {
	var logged struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(params, &logged) != nil {
		return
	}
	client.noteLog(params)
	_, folder, found := strings.Cut(logged.Message, "Importing with Gradle: ")
	if !found {
		return
	}
	client.lock.Lock()
	defer client.lock.Unlock()
	client.imported[strings.TrimSpace(folder)] = time.Now()
	client.importing = filepath.Base(strings.TrimSpace(folder))
}

// noteLog keeps a message a server logged or showed (window/logMessage, window/showMessage).
func (client *Client) noteLog(params json.RawMessage) {
	var logged struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if json.Unmarshal(params, &logged) != nil || strings.TrimSpace(logged.Message) == "" {
		return
	}
	client.lock.Lock()
	defer client.lock.Unlock()
	first, _, _ := strings.Cut(strings.TrimSpace(logged.Message), "\n")
	client.log = append(client.log, Message{At: time.Now(), Error: logged.Type == 1, Text: first})
	if len(client.log) > logLength {
		client.log = client.log[len(client.log)-logLength:]
	}
	client.changed()
}

// changed tells the window, without waiting, that what the server says of itself changed.
func (client *Client) changed() {
	select {
	case client.changes <- struct{}{}:
	default:
	}
}

// Changes tells when what the server says of itself changed, to show it.
func (client *Client) Changes() <-chan struct{} { return client.changes }

// Progress is what the server is doing now.
func (client *Client) Progress() Progress {
	client.lock.Lock()
	defer client.lock.Unlock()
	progress := Progress{Phase: "ready", Percent: -1, Started: client.started, Since: client.started,
		Log: append([]Message(nil), client.log...)}
	progress.Done, progress.Total = len(client.imported), len(client.folders)
	progress.Projects = client.projectStates(time.Now())
	if latest := client.latestWork(); latest != nil {
		progress.Phase, progress.Detail = phaseOf(latest.title), latest.message
		progress.Percent, progress.Since = latest.percent, latest.since
		if progress.Phase == "importing" && client.importing != "" {
			progress.Detail = client.importing
		}
		return progress
	}
	if !client.isReady() {
		progress.Phase, progress.Detail, progress.Since = "starting", client.status, client.statusSince
		if percent, found := percentIn(strings.Fields(client.status + " x")[0]); found {
			progress.Percent = percent
		}
	}
	return progress
}

// projectStates says how far the server is with each of its folders. A server that tells which
// project it imports (kotlin-lsp) is followed project by project; of another, all are read
// together.
func (client *Client) projectStates(now time.Time) []ProjectProgress {
	busy, ready := len(client.working) > 0, client.isReady()
	done := now
	if ready && !busy {
		done = later(client.readyAt, client.idleAt)
	}
	var states []ProjectProgress
	for folder := range client.folders {
		state := ProjectProgress{Path: folder, State: "reading", Took: done.Sub(client.started)}
		if ready && !busy {
			state.State = "read"
		}
		if began, imported := client.imported[folder]; imported {
			state.State, state.Took = "imported", client.nextImport(began, done).Sub(began)
			if busy && filepath.Base(folder) == client.importing && client.nextImport(began, done) == done {
				state.State = "importing"
			}
		} else if len(client.imported) > 0 && busy {
			state.State, state.Took = "waiting", 0
		}
		states = append(states, state)
	}
	sort.Slice(states, func(first, second int) bool { return states[first].Path < states[second].Path })
	return states
}

// nextImport is when the import begun after one began, or done when it was the last.
func (client *Client) nextImport(began, done time.Time) time.Time {
	next := done
	for _, other := range client.imported {
		if other.After(began) && other.Before(next) {
			next = other
		}
	}
	return next
}

// later is the later of two times.
func later(first, second time.Time) time.Time {
	if second.After(first) {
		return second
	}
	return first
}

// latestWork is the work begun last, of what the server reports running; nil for none.
func (client *Client) latestWork() *work {
	var latest *work
	for _, running := range client.working {
		if latest == nil || running.since.After(latest.since) {
			latest = running
		}
	}
	return latest
}

// phaseOf names a piece of work by its title: importing, indexing, or the title itself.
func phaseOf(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "import"):
		return "importing"
	case strings.Contains(lower, "index"):
		return "indexing"
	case lower == "":
		return "working"
	}
	return lower
}

// Busy tells whether the server reports work still running.
func (client *Client) Busy() bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	return len(client.working) > 0
}

// idleCheck is how often a request waiting for a busy server looks again.
const idleCheck = 200 * time.Millisecond

// waitIdle waits for the server to end the work it reports, the request's context, or the server.
func (client *Client) waitIdle(ctx context.Context) error {
	ticker := time.NewTicker(idleCheck)
	defer ticker.Stop()
	for client.Busy() {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return failure.Wrap(RequestFailed, "waiting for "+client.Server.Name+" to finish reading", ctx.Err())
		case <-client.conn.Done():
			return failure.Wrap(ServerStopped, client.Server.Name, client.conn.Err())
		}
	}
	return nil
}

// askTries is how many times a request answered with nothing by a busy server is asked again.
const askTries = 3

// ask sends a request at once: a server still reading answers what it has read already (jdtls on
// the big workspace answers minutes before it says it is ready). An answer with nothing before the server is
// ready is asked again once it is; one from a server that was busy, once it is idle: kotlin-lsp,
// still importing, answers every request empty.
func (client *Client) ask(ctx context.Context, method string, params any, empty func(json.RawMessage) bool) (json.RawMessage, error) {
	client.noteAsked()
	defer client.noteAsked()
	if !client.isReady() {
		var raw json.RawMessage
		if err := client.conn.Call(ctx, method, params, &raw); err == nil && !empty(raw) {
			return raw, nil
		}
		if err := client.waitReady(ctx); err != nil {
			return nil, err
		}
	}
	for try := 1; ; try++ {
		busy := client.Busy()
		var raw json.RawMessage
		if err := client.conn.Call(ctx, method, params, &raw); err != nil {
			return nil, err
		}
		if !empty(raw) || !(busy || client.Busy()) || try == askTries {
			return raw, nil
		}
		if err := client.waitIdle(ctx); err != nil {
			return nil, err
		}
	}
}

// noteAsked notes that the server was asked something, or answered.
func (client *Client) noteAsked() {
	client.lock.Lock()
	client.askedAt = time.Now()
	client.lock.Unlock()
}

// QuietSince is when the server was last asked something, answered, or ended the work it
// reported; when it started, if none of that happened since.
func (client *Client) QuietSince() time.Time {
	client.lock.Lock()
	defer client.lock.Unlock()
	quiet := client.started
	for _, moment := range []time.Time{client.readyAt, client.idleAt, client.askedAt} {
		if moment.After(quiet) {
			quiet = moment
		}
	}
	return quiet
}
