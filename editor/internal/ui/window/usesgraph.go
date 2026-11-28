package window

import (
	"context"
	"path/filepath"

	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/ui/mode"
	"github.com/akrck02/coddy/editor/internal/ui/navigation"
)

// UsersOf finds the declaration a use is in, a method or a class, and who uses that declaration,
// asking the file's own server: the graph of the name opened a level further. done gets the
// declaration's name and its uses, once known; none when the use is in no declaration.
func (application *Window) UsersOf(use mode.Use, done func(declaration string, users []mode.Use)) bool {
	project := application.workspace.Projects[use.File.Project]
	path := filepath.Join(project.Path, filepath.FromSlash(use.File.Path))
	text := application.LineOf(path, use.Line-1)
	at := lsp.Position{Line: use.Line - 1, Character: lsp.UTF16Column(text, use.Start)}
	client, _, reads := application.servers.ClientFor(project.Path, path)
	switch {
	case !reads:
		application.toast = "No language server reads " + filepath.Base(path)
		return false
	case client == nil:
		application.toast = "The server of " + filepath.Base(path) + " is starting: try again in a moment"
		return false
	}
	application.work.Run("users of "+path, func(stop context.Context) func() {
		requestContext, cancel := context.WithTimeout(stop, navigation.RequestWait)
		defer cancel()
		defer navigation.OpenFor(client, path)()
		symbols, _ := client.Symbols(requestContext, path) // no symbols: the use is in no declaration, as told
		around, found := lsp.Enclosing(symbols, at)
		var locations []lsp.Location
		if found {
			locations, _ = client.References(requestContext, path, around.Selection.Start) // no answer: no users
		}
		return func() {
			if !found {
				application.toast = "This use is in no declaration a server knows: an import, say"
			}
			done(around.Name, application.usersFound(locations, path, around))
		}
	})
	return true
}

// usersFound are the uses of a declaration, but its own name where it is declared.
func (application *Window) usersFound(locations []lsp.Location, path string, declared lsp.Symbol) []mode.Use {
	var users []mode.Use
	for _, location := range locations {
		if lsp.PathOf(location.URI) == path && location.Range.Start.Line == declared.Selection.Start.Line {
			continue
		}
		if user, known := navigation.ModeUseOf(application, location); known {
			users = append(users, user)
		}
	}
	return users
}
