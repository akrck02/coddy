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

	"github.com/akrck02/code-review/internal/app"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/opengl/overlay"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/window"
	"golang.org/x/image/font/gofont/gomono"
)

const (
	foldersPreference     = "folders"
	onlyChangedPreference = "only-changed"
)

func main() {
	options := window.Options{Title: "Code review", MinWidth: 900, MinHeight: 520}
	options.RegisterFlags(flag.CommandLine)
	base := flag.String("base", "", "compare every project against this branch, tag or commit (default: each one's main or master)")
	committed := flag.Bool("committed", false, "compare the last commit only, leaving out changes not committed yet")
	light := flag.Bool("light", false, "start in the light theme")
	open := flag.String("open", "", "open this file at the start, as project/path/in/it")
	reviewing := flag.Bool("review", false, "start in the review mode: the changed files, as diffs")
	codeFont := flag.String("code-font", "", "a TrueType file to read the code in (default: SF Mono, else Go Mono)")
	allProjects := flag.Bool("all", false, "show every project, not only those with changes (the choice is remembered otherwise)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: code-review [flags] [repository or folder of repositories …]")
		flag.PrintDefaults()
	}
	flag.Parse()

	folders := flag.Args()
	if len(folders) == 0 {
		if remembered, found := window.Preference(foldersPreference); found && remembered != "" {
			folders = existing(strings.Split(remembered, "\n"))
		}
	}
	options.Fonts = overlay.Fonts{Mono: monospaced(*codeFont)}

	reader := app.New(app.Settings{
		Folders:     folders,
		Base:        *base,
		WorkingTree: !*committed,
		Dark:        options.Dark || !*light,
		OnlyChanged: onlyChanged(*allProjects),
		Open:        *open,
		Review:      *reviewing,
		// the files marked as viewed are kept in the reader's own folder, never in the repositories
		ReviewedPath: review.DefaultReviewedPath(),
		Synchronous:  options.CapturePath != "",
	})
	options.Driver = reader.Driver()
	options.Hidden = func() {
		reader.Closed()
		_ = window.KeepPreference(onlyChangedPreference, fmt.Sprint(reader.OnlyChanged()))
		_ = window.KeepPreference(foldersPreference, strings.Join(reader.Folders(), "\n"))
	}
	if err := window.Run(options, func(context *ui.Context) { reader.Build(context) }); err != nil {
		log.Fatal(err)
	}
}

// onlyChanged tells whether to start showing only the projects with changes: yes, unless -all asks
// for every project or the person chose that last time.
func onlyChanged(allProjects bool) bool {
	if allProjects {
		return false
	}
	remembered, found := window.Preference(onlyChangedPreference)
	return !found || remembered != "false"
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
