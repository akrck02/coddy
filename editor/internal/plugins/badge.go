package plugins

import (
	"hash/fnv"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/akrck02/go-renderer/draw2d"
)

// Badge is the picture that stands for a plugin in the settings. It is resolved once, when the
// plugin is read, so drawing it each frame only looks at these fields.
type Badge struct {
	// Icon is one of Lucide's names, when the manifest gives one.
	Icon string
	// File is the absolute path of the icon file the manifest gives, inside the plugin's folder.
	File string
	// Initial is the first letter of the plugin's name, in upper case: the automatic badge, drawn
	// when the plugin gives neither Icon nor File.
	Initial string
	// Source is the SVG of a built-in plugin's logo, compiled into Coddy: it has no folder.
	Source []byte
	// Colour is the plugin's own colour, chosen from its name: the fill of the automatic badge, and
	// the ink of a Lucide icon.
	Colour draw2d.Color
}

// badgeColours are the colours a badge is chosen from: mid tones that hold white letters and read
// on light and dark backgrounds alike.
var badgeColours = []draw2d.Color{
	draw2d.MustHex("#5b6cd9"), // indigo
	draw2d.MustHex("#1f9e8f"), // teal
	draw2d.MustHex("#d97a2b"), // orange
	draw2d.MustHex("#d64f72"), // rose
	draw2d.MustHex("#8b5cf6"), // violet
	draw2d.MustHex("#2f8fd6"), // sky
	draw2d.MustHex("#3f9f4f"), // green
	draw2d.MustHex("#b0623a"), // clay
}

// badgeOf resolves the badge of a plugin from its name, the icon its manifest gives and its folder.
// An icon file outside the folder is refused, and the automatic badge drawn instead.
func badgeOf(name, icon, folder string) Badge {
	badge := Badge{Initial: initialOf(name), Colour: badgeColour(name)}
	switch {
	case !isIconFile(icon):
		badge.Icon = icon
	case folder != "":
		badge.File, _ = iconFile(folder, icon) // a refused file leaves the automatic badge
	}
	return badge
}

// badgeColour is the colour of a name: the same name always gets the same one, so a plugin keeps
// its colour across runs and computers.
func badgeColour(name string) draw2d.Color {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name)) // a hash's Write never fails
	return badgeColours[hash.Sum32()%uint32(len(badgeColours))]
}

// initialOf is a name's first letter in upper case, "?" for an empty name.
func initialOf(name string) string {
	first, _ := utf8.DecodeRuneInString(name)
	if first == utf8.RuneError {
		return "?"
	}
	return string(unicode.ToUpper(first))
}

// iconExtensions are the files an icon may be: Coddy draws SVG and PNG pictures.
var iconExtensions = []string{".svg", ".png"}

// isIconFile tells whether a manifest's icon names a file rather than one of Lucide's icons.
func isIconFile(icon string) bool {
	extension := strings.ToLower(filepath.Ext(icon))
	for _, each := range iconExtensions {
		if extension == each {
			return true
		}
	}
	return false
}

// iconFile is the path of an icon file in a plugin's folder; inside is false for a path that
// leaves the folder (absolute, or climbing out with ".."), which a plugin may not point at.
func iconFile(folder, icon string) (path string, inside bool) {
	if !filepath.IsLocal(icon) {
		return "", false
	}
	return filepath.Join(folder, icon), true
}

// NeedsRestart tells whether turning the plugin on or off waits for Coddy's next start: language
// servers join, folders are read by a version control and modes reach the rail only then.
func (plugin *Plugin) NeedsRestart() bool {
	added := plugin.Contributes
	return added.LanguageServer != nil || added.VersionControl != nil || added.Mode != nil
}
