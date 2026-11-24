package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/akrck02/go-renderer/failure"
)

// Definition is where the name at a place of a document is declared.
func (client *Client) Definition(ctx context.Context, path string, at Position) ([]Location, error) {
	return client.locations(ctx, "textDocument/definition", path, at, nil)
}

// References are every place the name at a place of a document is used, its declaration included.
func (client *Client) References(ctx context.Context, path string, at Position) ([]Location, error) {
	return client.locations(ctx, "textDocument/references", path, at, map[string]bool{"includeDeclaration": true})
}

// locations asks a request whose answer is a location, a list of them, or nothing.
func (client *Client) locations(ctx context.Context, method, path string, at Position, context any) ([]Location, error) {
	params := map[string]any{"textDocument": TextDocumentIdentifier{URI: FileURI(path)}, "position": at}
	if context != nil {
		params["context"] = context
	}
	raw, err := client.ask(ctx, method, params, func(raw json.RawMessage) bool {
		found, _ := decodeLocations(raw)
		return len(found) == 0
	})
	if err != nil {
		return nil, err
	}
	return decodeLocations(raw)
}

// decodeLocations reads a Location, a list of them, or a list of LocationLinks.
func decodeLocations(raw json.RawMessage) ([]Location, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	if strings.HasPrefix(text, "{") {
		raw = json.RawMessage("[" + text + "]")
	}
	var items []struct {
		Location
		TargetURI   string `json:"targetUri"`
		TargetRange Range  `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, failure.Wrap(BadMessage, "locations", err)
	}
	locations := make([]Location, len(items))
	for index, item := range items {
		locations[index] = item.Location
		if item.TargetURI != "" {
			locations[index] = Location{URI: item.TargetURI, Range: item.TargetRange}
		}
	}
	return locations, nil
}

// Hover is the text the server shows for the name at a place: its type and documentation, as
// Markdown; "" when it has none.
func (client *Client) Hover(ctx context.Context, path string, at Position) (string, error) {
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: FileURI(path)}, Position: at}
	raw, err := client.ask(ctx, "textDocument/hover", params, func(raw json.RawMessage) bool { return hoverOf(raw) == "" })
	if err != nil {
		return "", err
	}
	return hoverOf(raw), nil
}

// hoverOf is the text of a hover's answer; "" for none.
func hoverOf(raw json.RawMessage) string {
	var answer struct {
		Contents json.RawMessage `json:"contents"`
	}
	if json.Unmarshal(raw, &answer) != nil {
		return ""
	}
	return hoverText(answer.Contents)
}

// hoverText reads a hover's contents: MarkupContent, a MarkedString, or a list of them.
func hoverText(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return plain
	}
	var markup struct {
		Language string `json:"language"`
		Value    string `json:"value"`
	}
	if json.Unmarshal(raw, &markup) == nil && markup.Value != "" {
		if markup.Language != "" {
			return "```" + markup.Language + "\n" + markup.Value + "\n```"
		}
		return markup.Value
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return ""
	}
	parts := make([]string, 0, len(list))
	for _, item := range list {
		if text := hoverText(item); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}
