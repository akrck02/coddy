package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"testing"
	"time"
)

// pair is two connections facing each other over pipes, each answering with its own handler.
func pair(first, second Handler) (*Conn, *Conn) {
	firstReads, secondWrites := io.Pipe()
	secondReads, firstWrites := io.Pipe()
	return NewConn(firstReads, firstWrites, first), NewConn(secondReads, secondWrites, second)
}

// within is a context that gives up after a second, so a broken test fails instead of hanging.
func within(test *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	test.Cleanup(cancel)
	return ctx
}

func echo(method string, params json.RawMessage) (any, error) {
	if method == "fail" {
		return nil, errors.New("no such method") // the peer's own error, as a server's would be
	}
	return params, nil
}

func TestARequestGetsItsReply(test *testing.T) {
	client, _ := pair(nil, echo)
	var got map[string]int
	if err := client.Call(within(test), "echo", map[string]int{"line": 4}, &got); err != nil {
		test.Fatal(err)
	}
	if got["line"] != 4 {
		test.Errorf("the reply was %v", got)
	}
}

func TestTheServersOwnRequestsAreAnswered(test *testing.T) {
	asked := make(chan string, 1)
	client, server := pair(func(method string, _ json.RawMessage) (any, error) {
		asked <- method
		return []string{"setting"}, nil
	}, nil)
	_ = client
	var got []string
	if err := server.Call(within(test), "workspace/configuration", nil, &got); err != nil {
		test.Fatal(err)
	}
	if <-asked != "workspace/configuration" || len(got) != 1 {
		test.Errorf("the client answered %v", got)
	}
}

func TestANotificationReachesTheHandler(test *testing.T) {
	heard := make(chan string, 1)
	client, _ := pair(nil, func(method string, _ json.RawMessage) (any, error) {
		heard <- method
		return nil, nil
	})
	if err := client.Notify("initialized", struct{}{}); err != nil {
		test.Fatal(err)
	}
	select {
	case method := <-heard:
		if method != "initialized" {
			test.Errorf("heard %q", method)
		}
	case <-time.After(time.Second):
		test.Fatal("the notification never arrived")
	}
}

func TestNotificationsAreHandledInTheOrderSent(test *testing.T) {
	heard := make(chan string, 200)
	client, _ := pair(nil, func(_ string, params json.RawMessage) (any, error) {
		heard <- string(params)
		return nil, nil
	})
	for number := range 200 {
		if err := client.Notify("textDocument/didChange", number); err != nil {
			test.Fatal(err)
		}
	}
	for number := range 200 {
		select {
		case got := <-heard:
			if got != strconv.Itoa(number) {
				test.Fatalf("notification %d was heard as %s", number, got)
			}
		case <-time.After(time.Second):
			test.Fatalf("notification %d never arrived", number)
		}
	}
}

func TestAnErrorReplyIsARequestFailed(test *testing.T) {
	client, _ := pair(nil, echo)
	if err := client.Call(within(test), "fail", nil, nil); !errors.Is(err, RequestFailed) {
		test.Errorf("an error reply gave %v", err)
	}
}

func TestABrokenMessageEndsTheConnection(test *testing.T) {
	reader, writer := io.Pipe()
	client := NewConn(reader, io.Discard, nil)
	go io.WriteString(writer, "Content-Length: 5\r\n\r\n{oops")
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		test.Fatal("the connection kept going after a broken message")
	}
	if !errors.Is(client.Err(), BadMessage) {
		test.Errorf("it ended with %v", client.Err())
	}
	if err := client.Call(within(test), "echo", nil, nil); !errors.Is(err, ServerStopped) {
		test.Errorf("a request after the end gave %v", err)
	}
}

func TestColumnsCountUTF16UnitsBothWays(test *testing.T) {
	line := "a😀b"
	if units := UTF16Column(line, 2); units != 3 {
		test.Errorf("two characters are %d units, want 3", units)
	}
	if characters := CharacterColumn(line, 3); characters != 2 {
		test.Errorf("three units are %d characters, want 2", characters)
	}
}

func TestAFileURIRoundTrips(test *testing.T) {
	path := "/work/my repo/Main.java"
	if back := PathOf(FileURI(path)); back != path {
		test.Errorf("%q came back as %q", path, back)
	}
	if PathOf("jdt://contents/rt.jar/java.lang/String.class") != "" {
		test.Error("a jar's URI was taken for a file")
	}
}

func TestAJarLocationNamesItsClass(test *testing.T) {
	uri := "jdt://contents/lamp-1.0-SNAPSHOT.jar/org.lighthouse.lamp/Lamp.java?=keeper/%5C/home%5C/.m2%5C/lamp.jar=/%3Corg.lighthouse.lamp%28Lamp.class"
	if pkg, file, found := ClassInJar(uri); !found || pkg != "org.lighthouse.lamp" || file != "Lamp.java" {
		test.Errorf("the jar's class is %q %q (%t)", pkg, file, found)
	}
	if _, _, found := ClassInJar("file:///lamp/Lamp.java"); found {
		test.Error("a file is taken for a class in a jar")
	}
}
