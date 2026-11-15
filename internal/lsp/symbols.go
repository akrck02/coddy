package lsp

import (
	"context"
	"encoding/json"
)

// Symbol is a declaration of a document: its name, what it spans, where its name is, and the
// declarations inside it.
type Symbol struct {
	Name      string   `json:"name"`
	Range     Range    `json:"range"`
	Selection Range    `json:"selectionRange"`
	Children  []Symbol `json:"children"`
}

// Symbols are the declarations of a document, as the server sees them.
func (client *Client) Symbols(ctx context.Context, path string) ([]Symbol, error) {
	params := map[string]any{"textDocument": TextDocumentIdentifier{URI: FileURI(path)}}
	raw, err := client.ask(ctx, "textDocument/documentSymbol", params, func(raw json.RawMessage) bool {
		return len(raw) == 0 || string(raw) == "null" || string(raw) == "[]"
	})
	if err != nil {
		return nil, err
	}
	var symbols []Symbol
	if json.Unmarshal(raw, &symbols) != nil {
		return nil, nil
	}
	// a server that answers with SymbolInformation gives a location instead of a range
	var flat []struct {
		Location Location `json:"location"`
	}
	if json.Unmarshal(raw, &flat) == nil {
		for index := range symbols {
			if index < len(flat) && symbols[index].Range == (Range{}) {
				symbols[index].Range, symbols[index].Selection = flat[index].Location.Range, flat[index].Location.Range
			}
		}
	}
	return symbols, nil
}

// Enclosing is the innermost declaration a place is in, not counting one whose name is there.
func Enclosing(symbols []Symbol, at Position) (Symbol, bool) {
	for _, symbol := range symbols {
		if !contains(symbol.Range, at) {
			continue
		}
		if inner, found := Enclosing(symbol.Children, at); found {
			return inner, true
		}
		if !contains(symbol.Selection, at) {
			return symbol, true
		}
	}
	return Symbol{}, false
}

// contains tells whether a place is in a range, its end left out.
func contains(span Range, at Position) bool {
	after := at.Line > span.Start.Line || at.Line == span.Start.Line && at.Character >= span.Start.Character
	before := at.Line < span.End.Line || at.Line == span.End.Line && at.Character < span.End.Character
	return after && before
}
