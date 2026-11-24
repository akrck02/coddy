package lsp

import (
	"github.com/akrck02/code-review/internal/language"
)

// Tell tells the server a document's text as the reader has it, edits included, at a version
// that grows with each new text (from 1): didOpen the first time, didChange afterwards. A text
// older than the one sent already is dropped, so texts told from several goroutines never leave
// the server on an old one.
func (client *Client) Tell(path, text string, version int) error {
	uri := FileURI(path)
	client.telling.Lock()
	defer client.telling.Unlock()
	client.lock.Lock()
	sent, open := client.versions[uri]
	if version <= sent {
		client.lock.Unlock()
		return nil
	}
	client.versions[uri] = version
	client.lock.Unlock()
	if open {
		return client.conn.Notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": version},
			"contentChanges": []map[string]string{{"text": text}},
		})
	}
	return client.conn.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": language.ServerLanguageOf(path), "version": version, "text": text,
	}})
}

// IsOpen tells whether a document was told to the server.
func (client *Client) IsOpen(path string) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	_, open := client.versions[FileURI(path)]
	return open
}

// CloseOlder closes a document told to the server, unless a newer text of it than version was told
// since: a file left for another closes, but not when it was opened again meanwhile.
func (client *Client) CloseOlder(path string, version int) error {
	client.lock.Lock()
	sent, open := client.versions[FileURI(path)]
	client.lock.Unlock()
	if !open || sent > version {
		return nil
	}
	return client.Close(path)
}

// Close tells the server a document told to it is no longer open: it reads it from disk again.
func (client *Client) Close(path string) error {
	uri := FileURI(path)
	client.telling.Lock()
	defer client.telling.Unlock()
	client.lock.Lock()
	_, open := client.versions[uri]
	delete(client.versions, uri)
	client.lock.Unlock()
	if !open {
		return nil
	}
	return client.conn.Notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}})
}
