package window

import (
	"context"
	"os"
	"path/filepath"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/mode"
)

// UsersOf finds the declaration a use is in, a method or a class, and who uses that declaration,
// asking the file's own server: the graph of the name opened a level further. done gets the
// declaration's name and its uses, once known; none when the use is in no declaration.
func (application *Window) UsersOf(use mode.Use, done func(declaration string, users []mode.Use)) bool {
	project := application.projects[use.File.Project]
	path := filepath.Join(project.Path, filepath.FromSlash(use.File.Path))
	text := lineOf(path, use.Line-1, application)
	place := candidate{path: path, project: project.Path, at: lsp.Position{Line: use.Line - 1, Character: lsp.UTF16Column(text, use.Start)}}
	client, _, state := application.clientOf(place)
	switch {
	case state == "none":
		application.toast = "No language server reads " + filepath.Base(path)
		return false
	case client == nil:
		application.toast = "The server of " + filepath.Base(path) + " is starting: try again in a moment"
		return false
	}
	application.work.Run("users of "+path, func(stop context.Context) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		defer openFor(client, path)()
		symbols, _ := client.Symbols(ctx, path)
		around, found := lsp.Enclosing(symbols, place.at)
		var locations []lsp.Location
		if found {
			locations, _ = client.References(ctx, path, around.Selection.Start)
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
		if user, known := application.modeUseOf(location); known {
			users = append(users, user)
		}
	}
	return users
}

// openFor tells a server of a file it does not have open, for a question about it, and gives what
// closes it again after.
func openFor(client *lsp.Client, path string) func() {
	if client.IsOpen(path) {
		return func() {}
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return func() {}
	}
	_ = client.Tell(path, string(text), 1)
	return func() { _ = client.Close(path) }
}
