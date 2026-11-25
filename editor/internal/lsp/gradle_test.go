package lsp

import (
	"testing"
	"time"
)

func TestAServerIsDoneImportingOnceReadyAndQuietAWhile(test *testing.T) {
	now := time.Now()
	client := &Client{ready: make(chan struct{}), working: map[string]*work{}, imported: map[string]time.Time{}}
	if client.settled(now) {
		test.Error("a server not ready yet is done importing")
	}
	close(client.ready)
	client.readyAt = now.Add(-time.Second)
	if client.settled(now) {
		test.Error("a server ready a second ago is done importing: it may report work again")
	}
	client.readyAt = now.Add(-time.Minute)
	client.working["import"] = &work{title: "Importing"}
	if client.settled(now) {
		test.Error("a server still importing is done importing")
	}
	delete(client.working, "import")
	client.idleAt = now.Add(-5 * time.Second)
	if client.settled(now) {
		test.Error("a server whose import ended 5 seconds ago is done importing")
	}
	client.idleAt = now.Add(-time.Minute)
	if !client.settled(now) {
		test.Error("a server ready and quiet for a minute is not done importing")
	}
	client.imported["lamp"] = now.Add(-3 * time.Second)
	if client.settled(now) {
		test.Error("a server that began importing a project 3 seconds ago is done importing")
	}
}

// kotlin-lsp answers with nothing until its first import is done: it is ready once that work ends.
func TestAServerReadyAfterWorkIsReadyOnceItsFirstWorkEnds(test *testing.T) {
	client := &Client{Server: Server{Ready: Readiness{AfterWork: true}}, ready: make(chan struct{}), working: map[string]*work{},
		imported: map[string]time.Time{}, changes: make(chan struct{}, 1)}
	client.noteProgress([]byte(`{"token":"import","value":{"kind":"begin","title":"Importing"}}`))
	if client.isReady() {
		test.Error("a server is ready while its first work runs")
	}
	client.noteProgress([]byte(`{"token":"import","value":{"kind":"end"}}`))
	if !client.isReady() {
		test.Error("a server is not ready once its first work ended")
	}
}
