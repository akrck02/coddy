package servers

import (
	"context"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// installWait is how long a server's install may take: go install builds it from its source.
const installWait = 10 * time.Minute

// Install runs a missing server's install command in the background. Once it is installed, the
// next file of its language starts it, as for a server found when the reader started.
func (fleet *Fleet) Install(name string) {
	server, found := fleet.Named(name)
	if !found || fleet.installing[name] {
		return
	}
	fleet.installing[name] = true
	fleet.work.Run("install "+name, func(context.Context) func() {
		ctx, cancel := context.WithTimeout(context.Background(), installWait)
		defer cancel()
		err := lsp.Install(ctx, server)
		return func() { fleet.installed(name, err) }
	})
}

// InstallCommand is the command that installs a server of the table, "" for one it does not know.
func (fleet *Fleet) InstallCommand(name string) string {
	server, _ := fleet.Named(name)
	return server.Install
}

// installed keeps how a server's install ended: missing no more, or why it did not work.
func (fleet *Fleet) installed(name string, err error) {
	delete(fleet.installing, name)
	if err != nil {
		fleet.installFailed[name] = err.Error()
		fleet.host.Toast(name + " could not be installed")
		return
	}
	delete(fleet.missing, name)
	delete(fleet.installFailed, name)
	fleet.host.Toast(name + " installed: it starts with a file of its language")
}
