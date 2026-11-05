package window

import (
	"context"
	"sync/atomic"

	platform "github.com/akrck02/go-ui/window"
)

// readerLimit is how many repositories are read at once, so a workspace of many projects on a
// slow disk does not start dozens of git processes together.
const readerLimit = 6

// work runs jobs off the goroutine that builds the interface and brings each result back to it,
// waking the window. A job given a key replaces the job under that key still running: the older
// one's context is cancelled and its result dropped. Only the interface's goroutine calls run and
// take.
type work struct {
	// synchronous runs every job at once, on the caller's goroutine: for a capture or a test,
	// where there is one frame to show the result in
	synchronous bool
	results     chan func()
	running     map[string]*job
	readers     chan struct{}
}

// cancellation tells a job that a newer job took its key: it should stop, and return nil.
type cancellation = context.Context

// job is one job running under a key.
type job struct {
	cancel   context.CancelFunc
	replaced bool // a newer job took its key; read and written on the interface's goroutine only
}

func newWork(synchronous bool) *work {
	return &work{
		synchronous: synchronous,
		results:     make(chan func(), 256),
		running:     map[string]*job{},
		readers:     make(chan struct{}, readerLimit),
	}
}

// run starts a job. The job returns what to do with its result on the interface's goroutine; it
// should stop early, returning nil, once its context is cancelled. An empty key replaces nothing.
func (work *work) run(key string, do func(stop cancellation) func()) {
	work.runSettling(key, do, nil)
}

// runSettling is run, with settle called on the interface's goroutine once the job is over,
// whether its result was applied, dropped for a newer job, or there was none: for counting the
// jobs still running.
func (work *work) runSettling(key string, do func(stop cancellation) func(), settle func()) {
	stop, cancel := context.WithCancel(context.Background())
	current := &job{cancel: cancel}
	if key != "" {
		work.cancel(key)
		work.running[key] = current
	}
	finish := func(apply func()) func() {
		return func() {
			if key != "" && work.running[key] == current {
				delete(work.running, key)
			}
			if settle != nil {
				settle()
			}
			if current.replaced || apply == nil {
				return // a newer job of the same key took over, or this one found nothing to do
			}
			apply()
		}
	}
	if work.synchronous {
		finish(do(stop))()
		cancel()
		return
	}
	go func() {
		defer cancel()
		work.results <- finish(do(stop))
		wake()
	}()
}

// cancel stops the job running under a key, if any, and drops its result.
func (work *work) cancel(key string) {
	if older, found := work.running[key]; found {
		older.replaced = true
		older.cancel()
		delete(work.running, key)
	}
}

// take applies the results that came back since the last frame.
func (work *work) take() {
	for {
		select {
		case apply := <-work.results:
			apply()
		default:
			return
		}
	}
}

// read runs a reading of a repository once one of the few readers is free; false when the job
// was cancelled while waiting, and nothing was read.
func (work *work) read(stop cancellation, reading func()) bool {
	select {
	case work.readers <- struct{}{}:
	case <-stop.Done():
		return false
	}
	defer func() { <-work.readers }()
	if stop.Err() != nil {
		return false
	}
	reading()
	return true
}

// windowOpen is set once the window has drawn its first frame and cleared when it closes: the
// window can only be woken in between. Work that finishes before the first frame needs no waking,
// as that frame takes its result.
var windowOpen atomic.Bool

// wake asks the window for a frame, when there is a window to ask.
func wake() {
	if !windowOpen.Load() {
		return
	}
	defer func() {
		_ = recover() // the window closed between the check and the call: there is nothing to wake
	}()
	platform.Wake()
}
