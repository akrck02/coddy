package lsp

import (
	"path/filepath"
	"sort"
)

// AddFolder adds a workspace folder to a server that runs for the whole window.
func (client *Client) AddFolder(folder string) error { return client.changeFolder(folder, true) }

// RemoveFolder takes a workspace folder away from a server that runs for the whole window.
func (client *Client) RemoveFolder(folder string) error { return client.changeFolder(folder, false) }

// changeFolder adds or removes a workspace folder, when it is not so already.
func (client *Client) changeFolder(folder string, add bool) error {
	client.lock.Lock()
	known := client.folders[folder]
	if add {
		client.folders[folder] = true
	} else {
		delete(client.folders, folder)
	}
	client.lock.Unlock()
	if known == add {
		return nil
	}
	client.changed()
	changed := []map[string]string{{"uri": FileURI(client.serverFolder(folder)), "name": filepath.Base(folder)}}
	event := map[string]any{"added": changed, "removed": []map[string]string{}}
	if !add {
		event = map[string]any{"added": []map[string]string{}, "removed": changed}
	}
	return client.conn.Notify("workspace/didChangeWorkspaceFolders", map[string]any{"event": event})
}

// FollowsFolders tells whether the server follows workspace folders added and removed while it
// runs; one that does not must be started again to read others.
func (client *Client) FollowsFolders() bool { return client.followsFolders }

// Folders are the workspace folders the server reads, sorted.
func (client *Client) Folders() []string {
	client.lock.Lock()
	defer client.lock.Unlock()
	folders := make([]string, 0, len(client.folders))
	for folder := range client.folders {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	return folders
}

// HasFolder tells whether a server already reads a workspace folder.
func (client *Client) HasFolder(folder string) bool {
	client.lock.Lock()
	defer client.lock.Unlock()
	return client.folders[folder]
}
