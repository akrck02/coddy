// Code review is a small, fast, read-only code reader for developments that change many
// repositories at once: open a folder of projects and read what each one changed against its base
// branch, file by file, with the changes coloured.
//
//	code-review [-base origin/master] [-committed] [-light] [folder …]
//
// Each folder is a git repository, or a folder of repositories side by side. With none, the
// folders opened last time are opened again.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/akrck02/code-review/internal/files"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/code-review/internal/window"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/opengl/overlay"
	ui "github.com/akrck02/go-ui"
	platform "github.com/akrck02/go-ui/window"
	"golang.org/x/image/font/gofont/gomono"
)

const (
	foldersPreference     = "folders"
	onlyChangedPreference = "only-changed"
	readOnlyPreference    = "read-only"
)

func main() {
	options := platform.Options{Title: "Code review", MinWidth: 900, MinHeight: 520}
	options.RegisterFlags(flag.CommandLine)
	asked := registerFlags()
	flag.Parse()
	options.Fonts = overlay.Fonts{Mono: monospaced(*asked.codeFont)}
	reader, err := window.New(settingsOf(asked, options),
		files.New(),
		// the files marked as viewed are kept in the reader's own folder, never in the repositories
		review.New(review.DefaultReviewedPath()),
	)
	if err != nil {
		log.Fatal(err)
	}
	run(options, reader)
}

// commandLine is what the flags ask for, read once they are parsed.
type commandLine struct {
	base, open, find, codeFont               *string
	committed, light, reviewing, allProjects *bool
	readOnly                                 *bool
}

// registerFlags declares the reader's own flags, beside the platform's.
func registerFlags() commandLine {
	asked := commandLine{
		base:        flag.String("base", "", "compare every project against this branch, tag or commit (default: each one's main or master)"),
		committed:   flag.Bool("committed", false, "compare the last commit only, leaving out changes not committed yet"),
		light:       flag.Bool("light", false, "start in the light theme"),
		open:        flag.String("open", "", "open this file at the start, as project/path/in/it"),
		find:        flag.String("find", "", "open the finder at the start, searching the files' content for this text"),
		reviewing:   flag.Bool("review", false, "start in the review mode: the changed files, as diffs"),
		codeFont:    flag.String("code-font", "", "a TrueType file to read the code in (default: SF Mono, else Go Mono)"),
		allProjects: flag.Bool("all", false, "show every project, not only those with changes (the choice is remembered otherwise)"),
		readOnly:    flag.Bool("read-only", false, "start read-only: files are only read, not edited (the choice is remembered otherwise)"),
	}
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: code-review [flags] [repository or folder of repositories …]")
		flag.PrintDefaults()
	}
	return asked
}

// settingsOf is how the reader starts: the folders named, or else those open last time, and what
// the flags ask for.
func settingsOf(asked commandLine, options platform.Options) window.Settings {
	folders := flag.Args()
	if len(folders) == 0 {
		if remembered, found := platform.Preference(foldersPreference); found && remembered != "" {
			folders = existing(strings.Split(remembered, "\n"))
		}
	}
	return window.Settings{
		Folders:     folders,
		Base:        *asked.base,
		WorkingTree: !*asked.committed,
		Dark:        options.Dark || !*asked.light,
		OnlyChanged: onlyChanged(*asked.allProjects),
		Open:        *asked.open,
		Find:        *asked.find,
		Mode:        startMode(*asked.reviewing),
		Synchronous: options.CapturePath != "",
		ReadOnly:    readOnly(*asked.readOnly),
		Servers:     serversFor(options.CapturePath),
		// what language servers wrote stays theirs after a restart
		ServersWrote: workspace.DefaultWrittenPath(),
	}
}

// serversFor are the language servers to run: none for a capture, which shows one frame and
// should start no program; nil, every server found on the PATH, otherwise.
func serversFor(capturePath string) []lsp.Server {
	if capturePath != "" {
		return []lsp.Server{}
	}
	return nil
}

// run shows the window until it is closed, and keeps the folders open and the projects shown for
// the next time.
func run(options platform.Options, reader *window.Window) {
	options.Driver = reader.Driver()
	options.Hidden = func() {
		reader.Closed()
		_ = platform.KeepPreference(onlyChangedPreference, fmt.Sprint(reader.OnlyChanged()))
		_ = platform.KeepPreference(readOnlyPreference, fmt.Sprint(reader.ReadOnly()))
		_ = platform.KeepPreference(foldersPreference, strings.Join(reader.Folders(), "\n"))
	}
	if err := platform.Run(options, func(context *ui.Context) { reader.Build(context) }); err != nil {
		log.Fatal(err)
	}
}

// onlyChanged tells whether to start showing only the projects with changes: yes, unless -all asks
// for every project or the person chose that last time.
func onlyChanged(allProjects bool) bool {
	if allProjects {
		return false
	}
	remembered, found := platform.Preference(onlyChangedPreference)
	return !found || remembered != "false"
}

// readOnly tells whether to start read-only: when -read-only asks, or the person chose it last
// time. Editing is on otherwise.
func readOnly(asked bool) bool {
	remembered, found := platform.Preference(readOnlyPreference)
	return asked || (found && remembered == "true")
}

// systemMono is the monospaced face macOS reads code in (Xcode's, Terminal's): clean, without the
// serifs of Go Mono, and on every Mac.
const systemMono = "/System/Library/Fonts/SFNSMono.ttf"

// monospaced is the face the code is drawn in: the one asked for, else SF Mono, else Go Mono
// (built in, so the reader works anywhere). go-renderer's own monospaced face is bold.
func monospaced(asked string) []byte {
	for _, path := range []string{asked, systemMono} {
		if path == "" {
			continue
		}
		if content, err := os.ReadFile(path); err == nil {
			return content
		}
		if path == asked {
			log.Printf("cannot read %s: the code is drawn in the default face", asked)
		}
	}
	return gomono.TTF
}

// existing keeps the folders that are still there.
func existing(folders []string) []string {
	var found []string
	for _, folder := range folders {
		if info, err := os.Stat(folder); err == nil && info.IsDir() {
			found = append(found, folder)
		}
	}
	return found
}

// startMode is the mode to start in: the review mode when asked for, else the first.
func startMode(reviewing bool) string {
	if reviewing {
		return "Review"
	}
	return ""
}
