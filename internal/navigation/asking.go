package navigation

import (
	"os"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

// RequestWait is how long a request waits for its answer: a server still reading the projects
// (jdtls on the big workspace takes minutes) answers once it is done.
const RequestWait = 3 * time.Minute

// Asking is what a request about the file open needs: its server, its path, and its text as it is
// now with the version it was told at.
type Asking struct {
	Client  *lsp.Client
	Path    string
	Text    string
	Version int
}

// TellFirst sends the text of the file as it was when the request was made, on the request's own
// goroutine, so the request never reaches the server before it; a text sent already is not sent again.
func (asked Asking) TellFirst() {
	_ = asked.Client.Tell(asked.Path, asked.Text, asked.Version) // a server that stopped answers the request with its error
}

// OpenFor tells a server of a file it does not have open, for a question about it, and gives what
// closes it again after.
func OpenFor(client *lsp.Client, path string) func() {
	if client.IsOpen(path) {
		return func() {}
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return func() {} // a file gone is asked about unopened: the server answers nothing
	}
	_ = client.Tell(path, string(text), 1)   // as above, the question tells whether it worked
	return func() { _ = client.Close(path) } // closing a file of a server that stopped changes nothing
}
