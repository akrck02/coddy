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
	"path/filepath"
	"slices"
	"strings"

	"github.com/akrck02/code-review/internal/files"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/uses"
	"github.com/akrck02/code-review/internal/window"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/opengl/overlay"
	ui "github.com/akrck02/go-ui"
	platform "github.com/akrck02/go-ui/window"
	"golang.org/x/image/font/gofont/gomono"
)

const (
	foldersPreference     = "folders"
	iconThemePreference   = "icon-theme"
	onlyChangedPreference = "only-changed"
	readOnlyPreference    = "read-only"
	serverDataPreference  = "server-data"
	sidebarPreference     = "sidebar-folded"
	statusPreference      = "hides-status"
	themePreference       = "theme"
)

func main() {
	options := platform.Options{Title: "Code review", MinWidth: 900, MinHeight: 520}
	options.RegisterFlags(flag.CommandLine)
	asked := registerFlags()
	flag.Parse()
	options.Fonts = systemFonts(*asked.codeFont)
	reader, err := window.New(settingsOf(asked, options),
		files.New(),
		// the files marked as viewed are kept in the reader's own folder, never in the repositories
		review.New(review.DefaultReviewedPath()),
		// every use of a name, across the projects, and their graphs (⇧F12)
		uses.New(),
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
	serverData, serverMemory                 *string
	keepIndexes                              *int
}

// registerFlags declares the reader's own flags, beside the platform's.
func registerFlags() commandLine {
	asked := commandLine{
		base:        flag.String("base", "", "compare every project against this branch, tag or commit (default: each one's main or master)"),
		committed:   flag.Bool("committed", false, "compare the last commit only, leaving out changes not committed yet"),
		light:       flag.Bool("light", false, "start in the light theme, One Light (the theme is chosen in the settings otherwise, and remembered)"),
		open:        flag.String("open", "", "open this file at the start, as project/path/in/it"),
		find:        flag.String("find", "", "open the finder at the start, searching the files' content for this text"),
		reviewing:   flag.Bool("review", false, "start in the review mode: the changed files, as diffs"),
		codeFont:    flag.String("code-font", "", "a TrueType file to read the code in (default: the system's, else Go Mono)"),
		allProjects: flag.Bool("all", false, "show every project, even if only the changes were shown last time"),
		readOnly:    flag.Bool("read-only", false, "start read-only: files are only read, not edited (the choice is remembered otherwise)"),
		serverData: flag.String("server-data", "", "keep the language servers' indexes in this folder, another disk say "+
			"(default: the reader's own folder; the choice is remembered)"),
		serverMemory: flag.String("server-memory", "2g", "the most memory each Java language server may take (jdtls, kotlin-lsp)"),
		keepIndexes:  flag.Int("keep-indexes", 1, "how many of kotlin-lsp's indexes to keep, one per set of projects, about 5 GB each"),
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
	folders := absolutes(flag.Args())
	if len(folders) == 0 {
		if remembered, found := platform.Preference(foldersPreference); found && remembered != "" {
			folders = existing(strings.Split(remembered, "\n"))
		}
	}
	return window.Settings{
		Folders:          folders,
		Base:             *asked.base,
		WorkingTree:      !*asked.committed,
		Theme:            themeName(*asked.light),
		Dark:             options.Dark || !*asked.light,
		Themes:           themes(),
		ThemesFolder:     theme.Folder(),
		IconTheme:        rememberedText(iconThemePreference),
		IconThemes:       iconThemes(),
		IconThemesFolder: theme.IconThemesFolder(),
		// an icon that cannot be drawn is told once, and drawn as Lucide's
		Tell:        func(problem error) { log.Printf("icon theme %v", problem) },
		OnlyChanged: onlyChanged(*asked.allProjects),
		Open:        *asked.open,
		Find:        *asked.find,
		Mode:        startMode(*asked.reviewing),
		Synchronous: options.CapturePath != "",
		ReadOnly:    readOnly(*asked.readOnly),
		// a capture shows the sidebar, whatever was left last time
		SidebarFolded: options.CapturePath == "" && remembered(sidebarPreference),
		HidesStatus:   remembered(statusPreference),
		Servers:       serversFor(options.CapturePath, *asked.serverMemory, *asked.keepIndexes),
		// what language servers wrote stays theirs after a restart
		ServersWrote: workspace.DefaultWrittenPath(),
		ServerData:   serverData(*asked.serverData),
	}
}

// serversFor are the language servers to run: none for a capture, which shows one frame and
// should start no program; every server of the table otherwise, found on the PATH, with the
// memory and the indexes the flags ask for.
func serversFor(capturePath, memory string, keep int) []lsp.Server {
	if capturePath != "" {
		return []lsp.Server{}
	}
	servers := slices.Clone(lsp.Servers)
	for index := range servers {
		if servers[index].Memory.Most != "" && memory != "" {
			servers[index].Memory.Most = memory
		}
		if servers[index].Index.Folder != "" && keep > 0 {
			servers[index].Index.Keep = keep
		}
	}
	return servers
}

// run shows the window until it is closed, and keeps the folders open and the projects shown for
// the next time.
func run(options platform.Options, reader *window.Window) {
	options.Driver = reader.Driver()
	options.Hidden = func() {
		reader.Closed()
		_ = platform.KeepPreference(onlyChangedPreference, fmt.Sprint(reader.OnlyChanged()))
		_ = platform.KeepPreference(readOnlyPreference, fmt.Sprint(reader.ReadOnly()))
		_ = platform.KeepPreference(sidebarPreference, fmt.Sprint(reader.SidebarFolded()))
		_ = platform.KeepPreference(statusPreference, fmt.Sprint(reader.HidesStatus()))
		_ = platform.KeepPreference(themePreference, reader.ThemeName())
		_ = platform.KeepPreference(iconThemePreference, reader.IconThemeName())
		_ = platform.KeepPreference(foldersPreference, strings.Join(reader.Folders(), "\n"))
	}
	if err := platform.Run(options, func(context *ui.Context) { reader.Build(context) }); err != nil {
		log.Fatal(err)
	}
}

// onlyChanged tells whether to start showing only the projects with changes: only if the person
// chose that last time, and -all does not ask for every project.
func onlyChanged(allProjects bool) bool {
	if allProjects {
		return false
	}
	remembered, _ := platform.Preference(onlyChangedPreference)
	return rememberedOnlyChanged(remembered)
}

// rememberedOnlyChanged reads the kept choice: every project unless "true" was kept, so a first
// start shows the whole workspace.
func rememberedOnlyChanged(remembered string) bool { return remembered == "true" }

// themeName is the theme to start in: One Light when -light asks, else the one chosen last time;
// "" leaves it to the reader (IntelliJ Dark in the dark).
func themeName(light bool) string {
	if light {
		return theme.DefaultLight
	}
	remembered, _ := platform.Preference(themePreference)
	return remembered
}

// themes are the reader's own themes and the person's, from the themes folder; a file that cannot
// be read is told, and left out.
func themes() []theme.Theme {
	found, problems := theme.Load(theme.Folder())
	for _, problem := range problems {
		log.Printf("theme %v", problem)
	}
	return found
}

// iconThemes are the person's icon themes: those in the reader's icon themes folder and those
// installed in Zed; a file that cannot be read is told, and left out.
func iconThemes() []theme.IconTheme {
	found, problems := theme.LoadIconThemes(theme.IconThemesFolder(), theme.ZedExtensionsFolder())
	for _, problem := range problems {
		log.Printf("icon theme %v", problem)
	}
	return found
}

// rememberedText is a preference kept as text, "" when none was kept.
func rememberedText(name string) string {
	remembered, _ := platform.Preference(name)
	return remembered
}

// serverData is the folder the servers keep their indexes in: the one -server-data names, kept
// for the next time, else the one named last time; "" is the reader's own folder.
func serverData(asked string) string {
	if asked != "" {
		absolute, err := filepath.Abs(asked)
		if err == nil {
			_ = platform.KeepPreference(serverDataPreference, absolute)
			return absolute
		}
	}
	remembered, _ := platform.Preference(serverDataPreference)
	return remembered
}

// remembered tells whether a yes-or-no preference was left on last time.
func remembered(preference string) bool {
	value, found := platform.Preference(preference)
	return found && value == "true"
}

// readOnly tells whether to start read-only: when -read-only asks, or the person chose it last
// time. Editing is on otherwise.
func readOnly(asked bool) bool {
	remembered, found := platform.Preference(readOnlyPreference)
	return asked || (found && remembered == "true")
}

// systemFonts are the fonts the system reads its interface and its code in (Adwaita Sans and
// Adwaita Mono on GNOME, SF and SF Mono on a Mac), with the code in the face asked for instead,
// if any.
func systemFonts(codeFont string) overlay.Fonts {
	// without a system font the Go fonts stay, so the reader works anywhere
	fonts, _ := overlay.SystemFonts()
	fonts.Mono = monospaced(codeFont, fonts.Mono)
	return fonts
}

// monospaced is the face the code is drawn in: the one asked for, else the system's, else Go
// Mono, built in. go-renderer's own monospaced face is bold.
func monospaced(asked string, system []byte) []byte {
	if asked != "" {
		content, err := os.ReadFile(asked)
		if err == nil {
			return content
		}
		log.Printf("cannot read %s: the code is drawn in the system's face", asked)
	}
	if system != nil {
		return system
	}
	return gomono.TTF
}

// absolutes are folders named on the command line as full paths, so "." is called by its own name
// and is found again when the reader starts elsewhere.
func absolutes(folders []string) []string {
	full := make([]string, 0, len(folders))
	for _, folder := range folders {
		if absolute, err := filepath.Abs(folder); err == nil {
			folder = absolute
		}
		full = append(full, folder)
	}
	return full
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
