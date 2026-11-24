package work

import (
	"context"
	"testing"
	"time"
)

// waitTo gives a job time to run on its goroutine; the jobs here finish at once, or never.
const waitTo = 2 * time.Second

// search is the key both searches run under.
const search = "search"

func TestANewerSearchCancelsTheOlderOne(test *testing.T) {
	jobs := New(false, func() {})
	var shown []string
	olderStopped := make(chan bool, 1)
	jobs.Run(search, func(stop context.Context) func() {
		select {
		case <-stop.Done():
			olderStopped <- true
			return nil
		case <-time.After(waitTo):
			olderStopped <- false
			return func() { shown = append(shown, "older") }
		}
	})
	newerDone := make(chan struct{})
	jobs.Run(search, func(stop context.Context) func() {
		defer close(newerDone)
		return func() { shown = append(shown, "newer") }
	})
	if stopped := <-olderStopped; !stopped {
		test.Error("the older search was not told to stop")
	}
	<-newerDone
	deadline := time.Now().Add(waitTo)
	for len(jobs.results) < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	jobs.Take()
	if len(shown) != 1 || shown[0] != "newer" {
		test.Errorf("the results shown are %v, want only the newer search's", shown)
	}
}

func TestJobsWithoutAKeyAllFinish(test *testing.T) {
	jobs := New(false, func() {})
	var finished int
	for range 3 {
		jobs.Run("", func(context.Context) func() { return func() { finished++ } })
	}
	deadline := time.Now().Add(waitTo)
	for len(jobs.results) < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	jobs.Take()
	if finished != 3 {
		test.Errorf("%d of 3 jobs without a key showed their result", finished)
	}
}

func TestAJobReplacedByANewerOneStillSettles(test *testing.T) {
	jobs := New(false, func() {})
	running := 0
	for range 2 {
		running++
		jobs.RunSettling(search, func(stop context.Context) func() {
			<-stop.Done() // the first waits to be replaced; the second is cancelled below
			return nil
		}, func() { running-- })
	}
	jobs.Cancel(search)
	deadline := time.Now().Add(waitTo)
	for running > 0 && time.Now().Before(deadline) {
		jobs.Take()
		time.Sleep(time.Millisecond)
	}
	if running != 0 {
		test.Errorf("%d jobs never settled", running)
	}
}
