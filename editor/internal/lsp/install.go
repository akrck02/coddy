package lsp

import (
	"github.com/akrck02/coddy/editor/internal/execution/programs"
	"github.com/akrck02/go-renderer/failure"
)

// Find is where a server's command is: on the PATH, or where installers put programs (see
// programs.Find).
func Find(server Server) (string, error) {
	path, err := programs.Find(server.Command)
	if err != nil {
		return "", failure.Wrap(ServerNotFound, server.Name, err)
	}
	return path, nil
}
