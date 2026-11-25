package icons

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/akrck02/go-renderer/failure"
)

// Theme is one of Zed's icon themes: the picture (an SVG file) each file and folder is drawn
// with. Its format is https://zed.dev/docs/extensions/icon-themes.
type Theme struct {
	Name string
	// Dark tells whether it is drawn for a dark theme.
	Dark bool
	// Folder is where its pictures' paths start: the folder of the extension it comes in.
	Folder string
	// stems and suffixes name the kind of a file by its name or the end of it; pictures give
	// the picture of each kind, its path from Folder
	stems, suffixes, pictures map[string]string
	folders                   folderPictures
	namedFolders              map[string]folderPictures
	chevrons                  folderPictures
}

// folderPictures are the pictures of something open and closed: a folder, or a chevron.
type folderPictures struct {
	Collapsed string `json:"collapsed"`
	Expanded  string `json:"expanded"`
}

// iconThemeFile is a file of icon themes, as Zed writes it.
type iconThemeFile struct {
	Themes []struct {
		Name         string                    `json:"name"`
		Appearance   string                    `json:"appearance"`
		Folders      folderPictures            `json:"directory_icons"`
		NamedFolders map[string]folderPictures `json:"named_directory_icons"`
		Chevrons     folderPictures            `json:"chevron_icons"`
		Stems        map[string]string         `json:"file_stems"`
		Suffixes     map[string]string         `json:"file_suffixes"`
		Pictures     map[string]struct {
			Path string `json:"path"`
		} `json:"file_icons"`
	} `json:"themes"`
}

// ReadThemes reads a file of icon themes, its pictures' paths starting at folder.
func ReadThemes(content []byte, folder string) ([]Theme, error) {
	var file iconThemeFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, failure.Wrap(NotAThemeFile, "", err)
	}
	if len(file.Themes) == 0 {
		return nil, NoThemes
	}
	themes := make([]Theme, 0, len(file.Themes))
	for _, each := range file.Themes {
		pictures := make(map[string]string, len(each.Pictures))
		for kind, picture := range each.Pictures {
			pictures[kind] = fromFolder(folder, picture.Path)
		}
		namedFolders := make(map[string]folderPictures, len(each.NamedFolders))
		for name, named := range each.NamedFolders {
			namedFolders[name] = named.from(folder)
		}
		themes = append(themes, Theme{
			Name: each.Name, Dark: each.Appearance != "light", Folder: folder,
			stems: each.Stems, suffixes: each.Suffixes, pictures: pictures,
			folders: each.Folders.from(folder), namedFolders: namedFolders, chevrons: each.Chevrons.from(folder),
		})
	}
	return themes, nil
}

// from are the pictures with their paths from a folder.
func (pictures folderPictures) from(folder string) folderPictures {
	return folderPictures{fromFolder(folder, pictures.Collapsed), fromFolder(folder, pictures.Expanded)}
}

// fromFolder is a picture's path from the theme's folder, joined once when the theme is read so
// that finding a file's picture each frame allocates nothing; "" stays "".
func fromFolder(folder, path string) string {
	if path == "" {
		return ""
	}
	return filepath.Join(folder, path)
}

// FilePicture is the picture a file is drawn with, found as Zed finds it: by its whole name; then
// by each end of it after a dot, the longest first ("tide.spec.ts": "spec.ts", then "ts"); then
// by what comes before its first dot ("Dockerfile.prod": "Dockerfile"); then the default. Names
// are matched as they are written. It is "" when the theme has no picture for the file.
func (set Theme) FilePicture(name string) string {
	if kind, found := set.kindOf(name); found {
		return set.pictures[kind]
	}
	rest, firstPart, cut := name, "", false
	for {
		before, after, found := strings.Cut(rest, ".")
		if !found {
			break
		}
		if !cut {
			firstPart, cut = before, true
		}
		if kind, found := set.kindOf(after); found {
			return set.pictures[kind]
		}
		rest = after
	}
	if kind, found := set.kindOf(firstPart); cut && found {
		return set.pictures[kind]
	}
	return set.pictures["default"]
}

// kindOf is the kind of file a name, or an end of one, stands for.
func (set Theme) kindOf(part string) (string, bool) {
	if kind, found := set.stems[part]; found {
		return kind, true
	}
	kind, found := set.suffixes[part]
	return kind, found
}

// FolderPicture is the picture a folder is drawn with, open or closed: its own by its name, or
// every folder's; "" when the theme has none.
func (set Theme) FolderPicture(name string, open bool) string {
	if named, found := set.namedFolders[strings.TrimSpace(name)]; found {
		if picture := named.of(open); picture != "" {
			return picture
		}
	}
	return set.folders.of(open)
}

// ChevronPicture is the picture of a folder's chevron, open or closed; "" when the theme has none.
func (set Theme) ChevronPicture(open bool) string {
	return set.chevrons.of(open)
}

// of is the open picture or the closed one.
func (pictures folderPictures) of(open bool) string {
	if open {
		return pictures.Expanded
	}
	return pictures.Collapsed
}
