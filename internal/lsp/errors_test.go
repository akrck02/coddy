package lsp

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, badHeader := readMessage(bufio.NewReader(strings.NewReader("Content-Length: x\r\n\r\n")))
	_, cut := readMessage(bufio.NewReader(strings.NewReader("Content-Length: 9\r\n\r\n{}")))
	client, _ := pair(nil, echo)
	failed := client.Call(context.Background(), "fail", nil, nil)
	_, notFound := Start(context.Background(), Server{Name: "none", Command: "no-such-language-server-here"}, "/", nil, "")
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a header that is not a number", badHeader, BadMessage},
		{"a message cut short", cut, ServerStopped},
		{"an error reply", failed, RequestFailed},
		{"a server not installed", notFound, ServerNotFound},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}
