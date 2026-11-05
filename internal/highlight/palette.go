package highlight

import (
	"sync"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/alecthomas/chroma/v2"
)

// Role is what a piece of code is, in the words an editor's theme colours it by (Zed's).
type Role string

const (
	Plain       Role = "text"
	Keyword     Role = "keyword"
	String      Role = "string"
	Escape      Role = "string.escape"
	Function    Role = "function"
	TypeName    Role = "type"
	Comment     Role = "comment"
	DocComment  Role = "comment.doc"
	Number      Role = "number"
	Constant    Role = "constant"
	Boolean     Role = "boolean"
	Attribute   Role = "attribute"
	Property    Role = "property"
	Variable    Role = "variable"
	Punctuation Role = "punctuation"
	Operator    Role = "operator"
	Tag         Role = "tag"
	Title       Role = "title"
	Namespace   Role = "namespace"
	Preprocess  Role = "preproc"
	Label       Role = "label"
)

// Palette gives each kind of token its colour, by its role.
type Palette struct {
	colours map[Role]draw2d.Color
	mutex   sync.Mutex
	byKind  map[chroma.TokenType]draw2d.Color
}

// NewPalette makes a palette from a colour per role; Plain colours whatever has no role of its own.
func NewPalette(colours map[Role]draw2d.Color) *Palette {
	return &Palette{colours: colours, byKind: map[chroma.TokenType]draw2d.Color{}}
}

// Colour is the colour of a kind of token.
func (palette *Palette) Colour(kind chroma.TokenType) draw2d.Color {
	palette.mutex.Lock()
	defer palette.mutex.Unlock()
	if colour, known := palette.byKind[kind]; known {
		return colour
	}
	colour, found := palette.colours[RoleOf(kind)]
	if !found {
		colour = palette.colours[Plain]
	}
	palette.byKind[kind] = colour
	return colour
}

// RoleOf finds the role of a chroma token: by its exact type first, then by its kind (any keyword,
// any string…).
func RoleOf(kind chroma.TokenType) Role {
	switch kind {
	case chroma.KeywordConstant:
		return Boolean
	case chroma.KeywordType:
		return TypeName
	case chroma.KeywordNamespace:
		return Keyword
	case chroma.NameFunction, chroma.NameFunctionMagic:
		return Function
	case chroma.NameClass, chroma.NameException, chroma.NameBuiltin, chroma.NameBuiltinPseudo:
		return TypeName
	case chroma.NameDecorator, chroma.NameAttribute:
		return Attribute
	case chroma.NameTag:
		return Tag
	case chroma.NameConstant, chroma.NameEntity:
		return Constant
	case chroma.NameNamespace:
		return Namespace
	case chroma.NameProperty:
		return Property
	case chroma.NameLabel:
		return Label
	case chroma.NameVariable, chroma.NameVariableClass, chroma.NameVariableGlobal, chroma.NameVariableInstance:
		return Variable
	case chroma.LiteralStringEscape, chroma.LiteralStringInterpol:
		return Escape
	case chroma.LiteralStringDoc, chroma.CommentSpecial:
		return DocComment
	case chroma.CommentPreproc, chroma.CommentPreprocFile:
		return Preprocess
	case chroma.GenericHeading, chroma.GenericSubheading:
		return Title
	case chroma.Punctuation:
		return Punctuation
	}
	switch kind.SubCategory() { // strings and numbers are kinds of literal
	case chroma.LiteralString:
		return String
	case chroma.LiteralNumber:
		return Number
	}
	switch kind.Category() {
	case chroma.Keyword:
		return Keyword
	case chroma.Comment:
		return Comment
	case chroma.Operator:
		return Operator
	}
	return Plain
}
