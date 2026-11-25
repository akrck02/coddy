package lsp

import (
	"context"
	"errors"
	"testing"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, notFound := Start(context.Background(), Server{Name: "none", Command: "no-such-language-server-here"}, "/", nil, "")
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a server not installed", notFound, ServerNotFound},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}
