// Package sdk is what a plugin of Coddy is written with: the protocol between Coddy and its
// plugins, and what a plugin written in Go needs to answer it.
//
// A plugin is a folder with a plugin.json that names its program and says what it adds to Coddy.
// A plugin that speaks coddy talks to Coddy over its program's standard input and output:
// JSON-RPC 2.0 messages, each framed as language servers frame theirs, a "Content-Length: N"
// header, an empty line and N bytes of JSON (the jsonrpc package, which Coddy uses on its side
// too). Coddy asks, the plugin answers; a plugin in Go only has to give Serve
// what it answers (Handlers): what each of its actions does, its version control, its review
// source, and what it does with its settings when it starts:
//
//	func main() {
//		err := sdk.Serve(sdk.Handlers{RunAction: export})
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Plugins in other languages follow the same messages, described by the types of this package.
package sdk

// TODO(akrck02): initialize could carry Coddy's theme (its name, dark or light, its colours), for
// plugins that make something to be seen; no plugin needs it yet.

// ProtocolVersion is the version of the messages this package describes. Coddy sends its own in
// initialize, and leaves off a plugin that answers another.
const ProtocolVersion = 1

// The methods Coddy calls.
const (
	// InitializeMethod is the first request, once the plugin's program starts:
	// InitializeParams in, InitializeResult out.
	InitializeMethod = "initialize"
	// RunActionMethod runs one of the actions the plugin's manifest declares: ActionParams in,
	// ActionResult out.
	RunActionMethod = "action/run"
)

// InitializeParams is what Coddy says of itself when the plugin starts.
type InitializeParams struct {
	ProtocolVersion int `json:"protocolVersion"`
	// Settings are the values of the settings the plugin's manifest declares, by their ids: a
	// string, or a boolean for a "boolean" setting. Every setting is there, with its default when
	// the person set none. When the person changes one, Coddy stops the program, and the next
	// use starts it with the new values: a plugin reads them only here.
	Settings map[string]any `json:"settings,omitempty"`
}

// InitializeResult is the plugin's answer to initialize.
type InitializeResult struct {
	ProtocolVersion int `json:"protocolVersion"`
}

// ActionParams says which action to run, and on what.
type ActionParams struct {
	// Action is the action's id, as the manifest declares it.
	Action string `json:"action"`
	// File is the absolute path of the file open.
	File string `json:"file"`
}

// TODO(akrck02): an action could also be given the selection and answer with edits (undone as one
// step) or a file to open; no plugin needs them yet.

// ActionResult is what an action did, for Coddy to show. Both fields are optional.
type ActionResult struct {
	// Note is shown to the person for a few seconds: what was done, or why nothing was.
	Note string `json:"note,omitempty"`
	// Created is a file the action made, by its absolute path, which Coddy tells of and offers to
	// open.
	Created string `json:"created,omitempty"`
}

// The methods Coddy calls on a plugin whose manifest contributes a version control, to read a
// repository it marks. Each runs in the repository's folder; paths are relative to it, with
// forward slashes.
const (
	// BranchMethod: FolderParams in, the name of what is checked out out.
	BranchMethod = "versionControl/branch"
	// DefaultBaseMethod: FolderParams in, the revision a development is compared against when the
	// person chose none ("" for none) out.
	DefaultBaseMethod = "versionControl/defaultBase"
	// FilesMethod: FolderParams in, every file it knows and every new one it does not ignore out.
	FilesMethod = "versionControl/files"
	// ChangesMethod: CompareParams in, the files changed since the base, as []Change, out.
	ChangesMethod = "versionControl/changes"
	// ReadMethod: ReadParams in, the text of a file at a revision out.
	ReadMethod = "versionControl/read"
	// DiffMethod: DiffParams in, a changed file's unified diff against the base out.
	DiffMethod = "versionControl/diff"
)

// LastCommit is the revision Coddy asks for to read a file as the last commit has it, whatever
// the version control calls it.
const LastCommit = "HEAD"

// FolderParams names the repository asked about.
type FolderParams struct {
	Folder string `json:"folder"`
}

// CompareParams says what changes are measured between: the base, and either the files on disk
// (WorkingTree) or the last commit.
type CompareParams struct {
	Folder      string `json:"folder"`
	Base        string `json:"base"`
	WorkingTree bool   `json:"workingTree"`
}

// Change is a file changed since the base. Status is "A" added, "M" modified, "D" deleted or
// "R" renamed (OldPath is where it was).
type Change struct {
	Status  string `json:"status"`
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
	// Untracked is a new file the version control does not know about yet.
	Untracked bool `json:"untracked,omitempty"`
}

// ReadParams asks for a file's text at a revision (LastCommit for the last commit).
type ReadParams struct {
	Folder   string `json:"folder"`
	Revision string `json:"revision"`
	Path     string `json:"path"`
}

// DiffParams asks for one changed file's diff, measured as CompareParams says.
type DiffParams struct {
	CompareParams
	Path    string `json:"path"`
	OldPath string `json:"oldPath,omitempty"`
}

// VersionControl is what a plugin that contributes a version control answers.
type VersionControl interface {
	Branch(folder string) (string, error)
	DefaultBase(folder string) (string, error)
	Files(folder string) ([]string, error)
	Changes(asked CompareParams) ([]Change, error)
	Read(asked ReadParams) (string, error)
	Diff(asked DiffParams) (string, error)
}

// The methods Coddy calls on a plugin whose manifest contributes a review source, to read the
// pull requests of a project. Each runs in the project's folder.
const (
	// ReviewListMethod: FolderParams in, the open pull requests, as []Review, out.
	ReviewListMethod = "review/list"
	// ReviewCommentsMethod: ReviewParams in, the comments on the pull request's lines, as
	// []ReviewComment, out.
	ReviewCommentsMethod = "review/comments"
)

// TODO(akrck02): posting a comment and approving a pull request; the prototype only reads.

// Review is an open pull request. BaseRef is the branch it is to be merged into; HeadRef is what
// Coddy fetches from the repository's remote to read its changes (for GitHub, pull/<id>/head).
type Review struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	BaseRef string `json:"baseRef"`
	HeadRef string `json:"headRef"`
}

// ReviewParams names a pull request of the project in Folder.
type ReviewParams struct {
	Folder string `json:"folder"`
	ID     string `json:"id"`
}

// ReviewComment is a comment on a line of a file of the pull request, as the pull request changes
// it: File is relative to the project, Line counts from 1.
type ReviewComment struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

// ReviewSource is what a plugin that contributes a review source answers.
type ReviewSource interface {
	List(folder string) ([]Review, error)
	Comments(asked ReviewParams) ([]ReviewComment, error)
}
