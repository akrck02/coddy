// Package work runs jobs off the goroutine that builds the interface and brings each result back
// to it: the one way the reader does anything slow (reading repositories, asking servers,
// searching) without stopping a frame.
package work

import "context"

// readerLimit is how many repositories are read at once, so a workspace of many projects on a
// slow disk does not start dozens of git processes together.
const readerLimit = 6

// Queue runs jobs off the goroutine that builds the interface and brings each result back to it,
// waking the window. A job given a key replaces the job under that key still running: the older
// one's context is cancelled and its result dropped. Only the interface's goroutine calls Run and
// Take.
type Queue struct {
	// synchronous runs every job at once, on the caller's goroutine: for a capture or a test,
	// where there is one frame to show the result in
	synchronous bool
	results     chan func()
	running     map[string]*job
	readers     chan struct{}
	// wake asks the window for a frame, to take a result that came back
	wake func()
}

// job is one job running under a key.
type job struct {
	cancel   context.CancelFunc
	replaced bool // a newer job took its key; read and written on the interface's goroutine only
}

// New makes a queue that wakes the window with wake when a result comes back. A synchronous queue
// runs every job at once, on the caller's goroutine: for a capture or a test, where there is one
// frame to show the result in.
func New(synchronous bool, wake func()) *Queue {
	return &Queue{
		synchronous: synchronous,
		results:     make(chan func(), 256),
		running:     map[string]*job{},
		readers:     make(chan struct{}, readerLimit),
		wake:        wake,
	}
}

// Synchronous tells whether the queue runs each job at once, for a capture or a test.
func (work *Queue) Synchronous() bool { return work.synchronous }

// Run starts a job. The job returns what to do with its result on the interface's goroutine; it
// should stop early, returning nil, once its context is cancelled: a newer job took its key. An
// empty key replaces nothing.
func (work *Queue) Run(key string, do func(stop context.Context) func()) {
	work.RunSettling(key, do, nil)
}

// RunSettling is Run, with settle called on the interface's goroutine once the job is over,
// whether its result was applied, dropped for a newer job, or there was none: for counting the
// jobs still running.
func (work *Queue) RunSettling(key string, do func(stop context.Context) func(), settle func()) {
	stop, cancel := context.WithCancel(context.Background())
	current := &job{cancel: cancel}
	if key != "" {
		work.Cancel(key)
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
		work.wake()
	}()
}

// WhenDone runs then on the interface's goroutine once done is closed, waking the window: always
// in the background, even for a capture or a test, as done may close long after.
func (work *Queue) WhenDone(done <-chan struct{}, then func()) {
	go func() {
		<-done
		work.results <- then
		work.wake()
	}()
}

// Cancel stops the job running under a key, if any, and drops its result.
func (work *Queue) Cancel(key string) {
	if older, found := work.running[key]; found {
		older.replaced = true
		older.cancel()
		delete(work.running, key)
	}
}

// Take applies the results that came back since the last frame.
func (work *Queue) Take() {
	for {
		select {
		case apply := <-work.results:
			apply()
		default:
			return
		}
	}
}

// Read runs a reading of a repository once one of the few readers is free; false when the job
// was cancelled while waiting, and nothing was read.
func (work *Queue) Read(stop context.Context, reading func()) bool {
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
