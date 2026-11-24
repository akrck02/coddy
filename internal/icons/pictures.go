package icons

import (
	"os"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-renderer/svg"
)

// Pictures are an icon theme's pictures: each SVG read the first time it is drawn, and drawn
// once for each size, so that later frames only look them up.
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

// Picture is an SVG drawn at a size in pixels, or nil when it cannot be read.
func (kept *Pictures) Picture(path string, pixels int) *draw2d.Image {
	key := pictureSize{path, pixels}
	if picture, found := kept.pictures[key]; found {
		return picture
	}
	drawing := kept.drawing(path)
	if drawing == nil {
		return nil
	}
	picture := drawing.Picture(pixels, pixels)
	kept.pictures[key] = picture
	return picture
}

// drawing is an SVG read, once; nil when it cannot be, which is told the first time.
func (kept *Pictures) drawing(path string) *svg.Drawing {
	if drawing, found := kept.drawings[path]; found {
		return drawing
	}
	content, err := os.ReadFile(path)
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
