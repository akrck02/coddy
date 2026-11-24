package window

import (
	"context"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// installWait is how long a server's install may take: go install builds it from its source
const installWait = 10 * time.Minute

// installServer runs a missing server's install command in the background. Once it is installed,
// the next file of its language starts it, as for a server found when the reader started.
func (application *Window) installServer(name string) {
	servers := &application.servers
	server, found := serverNamed(servers.table, name)
	if !found || servers.installing[name] {
		return
	}
	servers.installing[name] = true
	application.work.run("install "+name, func(cancellation) func() {
		ctx, cancel := context.WithTimeout(context.Background(), installWait)
		defer cancel()
		err := lsp.Install(ctx, server)
		return func() { application.installed(name, err) }
	})
}

// installed keeps how a server's install ended: missing no more, or why it did not work.
func (application *Window) installed(name string, err error) {
	servers := &application.servers
	delete(servers.installing, name)
	if err != nil {
		servers.installFailed[name] = err.Error()
		application.toast = name + " could not be installed"
		return
	}
	delete(servers.missing, name)
	delete(servers.installFailed, name)
	application.toast = name + " installed: it starts with a file of its language"
}
