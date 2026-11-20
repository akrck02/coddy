package window

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

// hasButton tells whether a button of this name is in the window.
func hasButton(window *uitest.Window, name string) bool {
	_, err := window.Element(name)
	return err == nil
}

func TestTheBarNamesTheFileOpenAndElseTheWorkspace(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	if !slices.Contains(window.Texts(), application.title()) {
		test.Errorf("with no file open the bar does not name the workspace, %q: %v", application.title(), window.Texts())
	}
	application.openByLabel("lighthouse/src/lamp.go")
	window.Frames(2)
	if !slices.Contains(window.Texts(), "lamp.go") {
		test.Errorf("the bar does not name the file open: %v", window.Texts())
	}
	if slices.Contains(window.Texts(), application.title()) {
		test.Error("the bar names the workspace while a file is open")
	}
}

func TestTheBarShowsTheButtonsOfTheModeOn(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	base := "Compare against another branch, tag or commit (b)"
	lock := "Read-only: on, files are only read (⌘⇧L)"
	if hasButton(window, base) || !hasButton(window, lock) {
		test.Error("the files mode's bar does not offer the lock alone, without the base")
	}
	press(window, input.Key2)
	window.Frames(2)
	if !hasButton(window, base) || hasButton(window, lock) {
		test.Error("the review mode's bar does not offer the base alone, without the lock")
	}
	if !hasButton(window, "Read the projects again (r)") {
		test.Error("the review mode's bar does not offer to read the projects again")
	}
}

// The rail and the sidebar reach the top of the window; the bar sits over the viewer alone, and
// its search keeps to the middle of it.
func TestTheBarSitsOverTheViewerAlone(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	window.Frames(2)
	search, err := window.Element("search files")
	if err != nil {
		test.Fatal(err)
	}
	filter, err := window.Element("Filter projects and files (⌘F)")
	if err != nil {
		test.Fatal(err)
	}
	sidebarRight := float32(modeRailWidth + 1 + application.sidebar.Width)
	if search.Rect.X < sidebarRight {
		test.Errorf("the search begins at %v, over the sidebar ending at %v", search.Rect.X, sidebarRight)
	}
	if search.Rect.Y+search.Rect.Height > titleBarHeight || filter.Rect.Y > titleBarHeight+20 {
		test.Errorf("the search is at %v, the sidebar's filter at %v: the sidebar does not begin at the top", search.Rect, filter.Rect)
	}
	width := float32(1200) // newWindow's
	middle := sidebarRight + (width-sidebarRight)/2
	if centre := search.Rect.X + search.Rect.Width/2; centre < middle-20 || centre > middle+20 {
		test.Errorf("the search's middle is at %v, the viewer's at %v", centre, middle)
	}
}

// A folder's crumb shows the folder in the sidebar, selected, with the keyboard; one in a chain of
// lone folders, as a Java package is (org.lamp), selects the chain's row.
func TestACrumbShowsItsFolderInTheSidebar(test *testing.T) {
	folder := test.TempDir()
	javaRepository(test, filepath.Join(folder, "lighthouse"), map[string]string{
		"app/build.gradle": "plugins {}\n", "app/src/main/java/org/lamp/Lamp.java": "package org.lamp;\n\nclass Lamp {}\n"})
	application := newReader(Settings{Folders: []string{folder}, Synchronous: true, ReadOnly: true,
		Open: "lighthouse/app/src/main/java/org/lamp/Lamp.java"})
	window, err := uitest.New(application.Build, uitest.Driver(application.Driver()), uitest.Size(2000, 800))
	if err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	application.focus = viewerPane
	for crumb, row := range map[string]string{"main": "app/src/main", "org": "app/src/main/java/org/lamp", "app": "app"} {
		if err := window.Click(crumb); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
		selected, ok := application.sidebar.SelectedRow()
		if !ok || selected.Kind != sidebar.FolderRow || selected.Path != row {
			test.Errorf("the crumb %s selects %+v, not the folder %s", crumb, selected, row)
		}
		if application.focus != sidebarPane {
			test.Errorf("the crumb %s leaves the keyboard out of the sidebar", crumb)
		}
		application.focus = viewerPane
	}
}
