package highlight

import (
	"sync"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// Palette gives each kind of token its colour, from a chroma style.
type Palette struct {
	style      *chroma.Style
	Background draw2d.Color
	Text       draw2d.Color
	mutex      sync.Mutex
	colours    map[chroma.TokenType]draw2d.Color
}

// NewPalette reads a chroma style ("github-dark", "github"…), or chroma's fallback if unknown.
func NewPalette(styleName string) *Palette {
	style := styles.Get(styleName)
	background := style.Get(chroma.Background)
	palette := &Palette{style: style, colours: map[chroma.TokenType]draw2d.Color{}}
	palette.Background = colourOf(background.Background, draw2d.Color{1, 1, 1, 1})
	palette.Text = colourOf(background.Colour, draw2d.Color{0, 0, 0, 1})
	return palette
}

// Colour is the colour of a kind of token.
func (palette *Palette) Colour(kind chroma.TokenType) draw2d.Color {
	palette.mutex.Lock()
	defer palette.mutex.Unlock()
	if colour, known := palette.colours[kind]; known {
		return colour
	}
	colour := colourOf(palette.style.Get(kind).Colour, palette.Text)
	palette.colours[kind] = colour
	return colour
}

func colourOf(colour chroma.Colour, otherwise draw2d.Color) draw2d.Color {
	if !colour.IsSet() {
		return otherwise
	}
	return draw2d.Color{float32(colour.Red()) / 255, float32(colour.Green()) / 255, float32(colour.Blue()) / 255, 1}
}
