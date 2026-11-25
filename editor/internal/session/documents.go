package session

import (
	"os"
	"path/filepath"
	"time"

	"github.com/akrck02/coddy/editor/internal/workspace"
)

// Read is a document read off the interface's goroutine, with what its file was on disk just
// before, to hand to Keep.
type Read struct {
	Reference workspace.FileReference
	Document  *workspace.Document
	stamp     diskStamp
}

// diskStamp is what a file was on disk: cheap to take (one stat, no reading), so a cached document
// can be checked each time it is opened. A file that does not exist has an empty stamp.
type diskStamp struct {
	modified time.Time
	size     int64
}

// stampOf is what a file is on disk now.
func stampOf(projectPath, path string) diskStamp {
	information, err := os.Stat(filepath.Join(projectPath, filepath.FromSlash(path)))
	if err != nil {
		return diskStamp{} // a file that cannot be seen is told by reading it, not here
	}
	return diskStamp{modified: information.ModTime(), size: information.Size()}
}

// ReadDocument reads a file of a project, as its document. It touches nothing of the session, so
// it runs on any goroutine. The file is stamped before it is read, so a change written during the
// reading is seen the next time it is opened.
func ReadDocument(project *workspace.Project, reference workspace.FileReference, workingTree bool) Read {
	stamp := stampOf(project.Path, reference.Path)
	document := workspace.OpenDocument(project, reference.Path, workingTree)
	return Read{Reference: reference, Document: document, stamp: stamp}
}

// Keep remembers a document read, forgetting the least recently opened ones past the budget.
func (session *Session) Keep(read Read) { session.documents.keep(read) }

// Document is the document of a file already read, unless the file changed on disk since.
func (session *Session) Document(reference workspace.FileReference) (*workspace.Document, bool) {
	if reference.Project < 0 || reference.Project >= len(session.Projects) {
		return nil, false
	}
	return session.documents.take(reference, stampOf(session.Projects[reference.Project].Path, reference.Path))
}

// Forget forgets the document of one file, so it is read again.
func (session *Session) Forget(reference workspace.FileReference) {
	session.documents.forget(reference)
}

// ForgetDocuments forgets every document read, so they are read again.
func (session *Session) ForgetDocuments() { session.documents.forgetAll() }
