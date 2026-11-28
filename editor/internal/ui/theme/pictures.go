package theme

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-renderer/svg"
)

// Pictures are an icon theme's pictures, or a plugin's: each SVG read the first time it is drawn,
// and drawn once for each size, so that later frames only look them up. A PNG has pixels of its
// own: it is read once, and the same picture serves every size.
type Pictures struct {
	drawings map[string]*svg.Drawing // nil for an SVG that cannot be read
	pictures map[pictureSize]*draw2d.Image
	tell     func(error)
}

// pictureSize is a picture of an SVG at a size in pixels.
type pictureSize struct {
	path   string
	pixels int
}

// NewPictures keeps the pictures of an icon theme. tell hears, once each, of the SVGs that
// cannot be read: their files are drawn with Lucide's icons instead.
func NewPictures(tell func(error)) *Pictures {
	return &Pictures{drawings: map[string]*svg.Drawing{}, pictures: map[pictureSize]*draw2d.Image{}, tell: tell}
}

// Picture is an SVG drawn at a size in pixels, or a PNG as it is; nil when it cannot be read.
func (kept *Pictures) Picture(path string, pixels int) *draw2d.Image {
	isPhoto := strings.EqualFold(filepath.Ext(path), ".png")
	if isPhoto {
		pixels = 0 // one picture for every size
	}
	key := pictureSize{path, pixels}
	if picture, found := kept.pictures[key]; found {
		return picture
	}
	if isPhoto {
		picture := kept.photo(path)
		kept.pictures[key] = picture
		return picture
	}
	return kept.drawn(key, func() ([]byte, error) { return os.ReadFile(path) })
}

// SourcePicture is an SVG compiled into Coddy drawn at a size in pixels, kept under a name of its
// own as a file's picture is under its path; nil when it cannot be read.
func (kept *Pictures) SourcePicture(name string, source []byte, pixels int) *draw2d.Image {
	key := pictureSize{name, pixels}
	if picture, found := kept.pictures[key]; found {
		return picture
	}
	return kept.drawn(key, func() ([]byte, error) { return source, nil })
}

// drawn draws an SVG at a size and keeps the picture; read gives its content the first time.
func (kept *Pictures) drawn(key pictureSize, read func() ([]byte, error)) *draw2d.Image {
	drawing := kept.drawing(key.path, read)
	if drawing == nil {
		return nil
	}
	picture := drawing.Picture(key.pixels, key.pixels)
	kept.pictures[key] = picture
	return picture
}

// drawing is an SVG read, once; nil when it cannot be, which is told the first time.
func (kept *Pictures) drawing(path string, read func() ([]byte, error)) *svg.Drawing {
	if drawing, found := kept.drawings[path]; found {
		return drawing
	}
	content, err := read()
	var drawing *svg.Drawing
	if err == nil {
		drawing, err = svg.Read(content)
	}
	if drawing == nil && err != nil {
		kept.tell(failure.Wrap(NotDrawn, path, err))
	}
	kept.drawings[path] = drawing
	return drawing
}

// photo is a PNG read; nil when it cannot be, which is told.
func (kept *Pictures) photo(path string) *draw2d.Image {
	file, err := os.Open(path)
	if err != nil {
		kept.tell(failure.Wrap(NotDrawn, path, err))
		return nil
	}
	defer file.Close() // only read: closing it cannot lose anything
	picture, err := draw2d.DecodeImage(file)
	if err != nil {
		kept.tell(failure.Wrap(NotDrawn, path, err))
	}
	return picture
}
