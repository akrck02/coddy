package app

import (
	"testing"
	"time"
)

// waitTo gives a job time to run on its goroutine; the jobs here finish at once, or never.
const waitTo = 2 * time.Second

func TestANewerSearchCancelsTheOlderOne(test *testing.T) {
	jobs := newWork(false)
	var shown []string
	olderStopped := make(chan bool, 1)
	jobs.run(contentSearch, func(stop cancellation) func() {
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
	jobs.run(contentSearch, func(stop cancellation) func() {
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
	jobs.take()
	if len(shown) != 1 || shown[0] != "newer" {
		test.Errorf("the results shown are %v, want only the newer search's", shown)
	}
}

func TestJobsWithoutAKeyAllFinish(test *testing.T) {
	jobs := newWork(false)
	var finished int
	for range 3 {
		jobs.run("", func(cancellation) func() { return func() { finished++ } })
	}
	deadline := time.Now().Add(waitTo)
	for len(jobs.results) < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	jobs.take()
	if finished != 3 {
		test.Errorf("%d of 3 jobs without a key showed their result", finished)
	}
}
