package lsp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/akrck02/go-renderer/failure"
)

// noteProgress follows the work a server reports ($/progress): a begin starts it, an end ends it.
// While some runs (Gradle's import, indexing) the server may answer with nothing.
func (client *Client) noteProgress(params json.RawMessage) {
	var progress struct {
		Token json.RawMessage `json:"token"`
		Value struct {
			Kind string `json:"kind"`
		} `json:"value"`
	}
	if json.Unmarshal(params, &progress) != nil {
		return
	}
	client.lock.Lock()
	defer client.lock.Unlock()
	switch progress.Value.Kind {
	case "begin":
		client.working[string(progress.Token)] = true
	case "end":
		delete(client.working, string(progress.Token))
	}
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

// ask sends a request once the server is ready. An answer with nothing from a server that was busy
// is asked again once it is idle: kotlin-lsp, still importing, answers every request empty.
func (client *Client) ask(ctx context.Context, method string, params any, empty func(json.RawMessage) bool) (json.RawMessage, error) {
	if err := client.waitReady(ctx); err != nil {
		return nil, err
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
