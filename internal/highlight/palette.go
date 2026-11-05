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

// exactRoles are the roles of the chroma tokens known by their exact type.
var exactRoles = map[chroma.TokenType]Role{
	chroma.KeywordConstant:       Boolean,
	chroma.KeywordType:           TypeName,
	chroma.KeywordNamespace:      Keyword,
	chroma.NameFunction:          Function,
	chroma.NameFunctionMagic:     Function,
	chroma.NameClass:             TypeName,
	chroma.NameException:         TypeName,
	chroma.NameBuiltin:           TypeName,
	chroma.NameBuiltinPseudo:     TypeName,
	chroma.NameDecorator:         Attribute,
	chroma.NameAttribute:         Attribute,
	chroma.NameTag:               Tag,
	chroma.NameConstant:          Constant,
	chroma.NameEntity:            Constant,
	chroma.NameNamespace:         Namespace,
	chroma.NameProperty:          Property,
	chroma.NameLabel:             Label,
	chroma.NameVariable:          Variable,
	chroma.NameVariableClass:     Variable,
	chroma.NameVariableGlobal:    Variable,
	chroma.NameVariableInstance:  Variable,
	chroma.LiteralStringEscape:   Escape,
	chroma.LiteralStringInterpol: Escape,
	chroma.LiteralStringDoc:      DocComment,
	chroma.CommentSpecial:        DocComment,
	chroma.CommentPreproc:        Preprocess,
	chroma.CommentPreprocFile:    Preprocess,
	chroma.GenericHeading:        Title,
	chroma.GenericSubheading:     Title,
	chroma.Punctuation:           Punctuation,
}

// kindRoles are the roles of the chroma tokens known only by their kind: strings and numbers are
// kinds of literal, keywords, comments and operators are kinds of their own. A token is looked up
// by its subcategory, then by its category; a keyword, a comment or an operator may be found by
// both, with the same role.
var kindRoles = map[chroma.TokenType]Role{
	chroma.LiteralString: String,
	chroma.LiteralNumber: Number,
	chroma.Keyword:       Keyword,
	chroma.Comment:       Comment,
	chroma.Operator:      Operator,
}

// RoleOf finds the role of a chroma token: by its exact type first, then by its kind (any keyword,
// any string…).
func RoleOf(kind chroma.TokenType) Role {
	if role, found := exactRoles[kind]; found {
		return role
	}
	if role, found := kindRoles[kind.SubCategory()]; found {
		return role
	}
	if role, found := kindRoles[kind.Category()]; found {
		return role
	}
	return Plain
}

// RoleColour is the colour of a role, or of plain text when the role has none of its own.
func (palette *Palette) RoleColour(role Role) draw2d.Color {
	if colour, found := palette.colours[role]; found {
		return colour
	}
	return palette.colours[Plain]
}
