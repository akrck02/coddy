package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/akrck02/go-renderer/failure"
)

// Handler answers what a server sends on its own: a request, which wants a result back, or a
// notification (reply false), whose result is dropped. It runs on a goroutine of its own.
type Handler func(method string, params json.RawMessage) (result any, err error)

// Conn is a JSON-RPC 2.0 connection to a server: requests that wait for their reply,
// notifications, and the server's own requests handed to a Handler.
type Conn struct {
	writer    io.Writer
	writeLock sync.Mutex

	lock    sync.Mutex
	nextID  int64
	pending map[int64]chan message
	closed  bool
	err     error // why the connection ended

	handler Handler
	done    chan struct{}
}

// message is any JSON-RPC message: a request has a method and an id, a notification a method and
// no id, a reply an id and a result or an error.
type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *replyError      `json:"error,omitempty"`
}

// replyError is the error of a reply.
type replyError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewConn starts a connection over a server's output (reader) and input (writer), reading its
// messages until it ends.
func NewConn(reader io.Reader, writer io.Writer, handler Handler) *Conn {
	conn := &Conn{writer: writer, pending: map[int64]chan message{}, handler: handler, done: make(chan struct{})}
	go conn.read(bufio.NewReader(reader))
	return conn
}

// Done is closed once the connection has ended; Err then says why.
func (conn *Conn) Done() <-chan struct{} { return conn.done }

// Err is why the connection ended: nil while it runs.
func (conn *Conn) Err() error {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	return conn.err
}

// Call sends a request and waits for its reply, decoding its result into result (nil drops it).
// It stops early when ctx is done or the connection ends.
func (conn *Conn) Call(ctx context.Context, method string, params, result any) error {
	reply := make(chan message, 1)
	conn.lock.Lock()
	if conn.closed {
		conn.lock.Unlock()
		return failure.Wrap(ServerStopped, method, conn.err)
	}
	conn.nextID++
	id := conn.nextID
	conn.pending[id] = reply
	conn.lock.Unlock()
	defer conn.forget(id)
	raw := json.RawMessage(strconv.FormatInt(id, 10))
	if err := conn.send(message{ID: &raw, Method: method}, params); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		_ = conn.Notify("$/cancelRequest", map[string]int64{"id": id})
		return ctx.Err()
	case <-conn.done:
		return failure.Wrap(ServerStopped, method, conn.Err())
	case answer := <-reply:
		return decodeReply(method, answer, result)
	}
}

// decodeReply gives a reply's error, or decodes its result.
func decodeReply(method string, answer message, result any) error {
	if answer.Error != nil {
		return failure.Wrap(RequestFailed, method+": "+answer.Error.Message, nil)
	}
	if result == nil || len(answer.Result) == 0 || string(answer.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(answer.Result, result); err != nil {
		return failure.Wrap(BadMessage, method, err)
	}
	return nil
}

// forget drops a request that was answered or given up.
func (conn *Conn) forget(id int64) {
	conn.lock.Lock()
	delete(conn.pending, id)
	conn.lock.Unlock()
}

// Notify sends a notification: a message that wants no reply.
func (conn *Conn) Notify(method string, params any) error {
	return conn.send(message{Method: method}, params)
}

// send writes a message, with its params turned to JSON, as the protocol frames it: a header
// giving its length, then the JSON.
func (conn *Conn) send(outgoing message, params any) error {
	outgoing.JSONRPC = "2.0"
	if params != nil {
		encoded, err := json.Marshal(params)
		if err != nil {
			return failure.Wrap(BadMessage, outgoing.Method, err)
		}
		outgoing.Params = encoded
	}
	return conn.write(outgoing)
}

// write frames and writes a message.
func (conn *Conn) write(outgoing message) error {
	body, err := json.Marshal(outgoing)
	if err != nil {
		return failure.Wrap(BadMessage, outgoing.Method, err)
	}
	conn.writeLock.Lock()
	defer conn.writeLock.Unlock()
	if _, err := io.WriteString(conn.writer, "Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"); err != nil {
		return failure.Wrap(ServerStopped, outgoing.Method, err)
	}
	if _, err := conn.writer.Write(body); err != nil {
		return failure.Wrap(ServerStopped, outgoing.Method, err)
	}
	return nil
}

// read reads the server's messages until it ends or sends one that cannot be read.
func (conn *Conn) read(reader *bufio.Reader) {
	for {
		incoming, err := readMessage(reader)
		if err != nil {
			conn.end(err)
			return
		}
		conn.dispatch(incoming)
	}
}

// dispatch hands a reply to the request waiting for it, and the server's own messages to the handler.
func (conn *Conn) dispatch(incoming message) {
	if incoming.Method == "" {
		if incoming.ID == nil {
			return
		}
		id, err := strconv.ParseInt(string(*incoming.ID), 10, 64)
		conn.lock.Lock()
		reply, waiting := conn.pending[id]
		conn.lock.Unlock()
		if err == nil && waiting {
			reply <- incoming
		}
		return
	}
	go conn.answer(incoming)
}

// answer runs the handler on a message of the server, and replies when it was a request.
func (conn *Conn) answer(incoming message) {
	var result any
	var err error
	if conn.handler != nil {
		result, err = conn.handler(incoming.Method, incoming.Params)
	}
	if incoming.ID == nil {
		return // a notification wants no reply
	}
	reply := message{JSONRPC: "2.0", ID: incoming.ID}
	if err != nil {
		reply.Error = &replyError{Code: methodNotFound, Message: err.Error()}
	} else {
		encoded, _ := json.Marshal(result)
		reply.Result = encoded
	}
	_ = conn.write(reply)
}

// methodNotFound is JSON-RPC's code for a request the other side does not answer.
const methodNotFound = -32601

// end closes the connection for a reason, waking every request still waiting.
func (conn *Conn) end(reason error) {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	if conn.closed {
		return
	}
	conn.closed, conn.err = true, reason
	close(conn.done)
}

// readMessage reads one framed message: headers up to an empty line, then as many bytes of JSON
// as Content-Length says.
func readMessage(reader *bufio.Reader) (message, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return message{}, failure.Wrap(ServerStopped, "reading a header", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return message{}, failure.Wrap(BadMessage, line, err)
			}
		}
	}
	return readBody(reader, length)
}

// readBody reads a message's JSON.
func readBody(reader *bufio.Reader, length int) (message, error) {
	if length < 0 {
		return message{}, failure.Wrap(BadMessage, "no Content-Length", nil)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return message{}, failure.Wrap(ServerStopped, "reading a message", err)
	}
	var incoming message
	if err := json.Unmarshal(body, &incoming); err != nil {
		return message{}, failure.Wrap(BadMessage, string(body[:min(len(body), 80)]), err)
	}
	return incoming, nil
}
