package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const (
	CodeInvalidJSON       = "invalid_json"
	CodeUnknownCommand    = "unknown_command"
	CodeValidationError   = "validation_error"
	CodeRevisionConflict  = "revision_conflict"
	CodeLuaError          = "lua_error"
	CodeTaskLimitExceeded = "task_limit_exceeded"
	CodeInternalError     = "internal_error"
)

type ProtocolError struct {
	Code string
	Err  error
}

func (e *ProtocolError) Error() string { return e.Err.Error() }
func (e *ProtocolError) Unwrap() error { return e.Err }

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *ProtocolError
	if errors.As(err, &coded) {
		return coded.Code
	}
	var node *nodeError
	if errors.As(err, &node) {
		return CodeValidationError
	}
	var luaError *lua.ApiError
	if errors.As(err, &luaError) {
		return CodeLuaError
	}
	return CodeInternalError
}

func protocolError(code string, err error) error {
	if err == nil {
		return nil
	}
	var coded *ProtocolError
	if errors.As(err, &coded) || ErrorCode(err) != CodeInternalError {
		return err
	}
	return &ProtocolError{Code: code, Err: err}
}

type AgentRequest struct {
	Data  []byte
	Reply chan []byte
}

func ServeAgent(path string, requests chan<- AgentRequest, done <-chan struct{}) (func(), error) {
	if _, e := os.Lstat(path); e == nil {
		return nil, errors.New("socket path already exists; choose unused path")
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		listener.Close()
		return nil, e
	}
	slots := make(chan struct{}, 4)
	var wg sync.WaitGroup
	var once sync.Once
	stopped := make(chan struct{})
	var mu sync.Mutex
	connections := map[*net.UnixConn]struct{}{}
	shutdown := func() {
		once.Do(func() {
			mu.Lock()
			defer mu.Unlock()
			close(stopped)
			listener.Close()
			for conn := range connections {
				conn.Close()
			}
		})
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-done:
			shutdown()
		case <-stopped:
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, e := listener.AcceptUnix()
			if e != nil {
				return
			}
			select {
			case slots <- struct{}{}:
			case <-stopped:
				conn.Close()
				return
			case <-done:
				conn.Close()
				return
			default:
				conn.Close()
				continue
			}
			mu.Lock()
			select {
			case <-stopped:
				mu.Unlock()
				conn.Close()
				<-slots
				return
			default:
			}
			connections[conn] = struct{}{}
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					mu.Lock()
					delete(connections, conn)
					mu.Unlock()
				}()
				defer func() { <-slots }()
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				scanner.Buffer(make([]byte, 4096), maxText+3)
				for {
					conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					if !scanner.Scan() || len(scanner.Bytes()) > maxText {
						return
					}
					request := AgentRequest{append([]byte(nil), scanner.Bytes()...), make(chan []byte, 1)}
					select {
					case requests <- request:
					case <-stopped:
						return
					case <-done:
						return
					}
					select {
					case reply := <-request.Reply:
						conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
						buffers := net.Buffers{reply, []byte{'\n'}}
						if _, err := buffers.WriteTo(conn); err != nil {
							return
						}
					case <-stopped:
						return
					case <-done:
						return
					}
				}
			}()
		}
	}()
	return func() { shutdown(); wg.Wait() }, nil
}

func (r *Runtime) AgentReply(data []byte) []byte {
	var request struct {
		Method  string          `json:"method"`
		Packet  json.RawMessage `json:"packet"`
		Action  string          `json:"action"`
		Payload map[string]any  `json:"payload"`
	}
	if e := strictJSON(data, &request); e != nil {
		return replyJSON(nil, e)
	}
	switch request.Method {
	case "snapshot":
		snapshot, e := r.Snapshot()
		return replyJSON(json.RawMessage(snapshot), e)
	case "apply":
		e := r.ApplyJSON(request.Packet)
		r.ReconcileFocus()
		return replyJSON(map[string]any{"revision": r.Revision}, e)
	case "message":
		if r.L == nil {
			return replyJSON(nil, errors.New("runtime closed"))
		}
		e := r.Action(request.Action, request.Payload)
		return replyJSON(map[string]any{"revision": r.Revision}, protocolError(CodeValidationError, e))
	case "events":
		events := r.Events
		r.Events = nil
		return replyJSON(events, nil)
	default:
		return replyJSON(nil, protocolError(CodeUnknownCommand, errors.New("unknown method")))
	}
}

func replyJSON(result any, e error) []byte {
	m := map[string]any{"ok": e == nil}
	if e != nil {
		m["error"] = e.Error()
		m["code"] = ErrorCode(e)
	} else {
		m["result"] = result
	}
	b, err := json.Marshal(m)
	if err != nil {
		return replyJSON(nil, protocolError(CodeInternalError, err))
	}
	return b
}

func strictJSON(data []byte, target any) error {
	if len(data) > maxText {
		return protocolError(CodeValidationError, errors.New("packet exceeds 1 MiB"))
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if e := d.Decode(new(json.RawMessage)); e != nil {
		return protocolError(CodeInvalidJSON, e)
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return protocolError(CodeInvalidJSON, errors.New("trailing JSON"))
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return protocolError(CodeValidationError, d.Decode(target))
}

func (r *Runtime) ApplyJSON(data []byte) error {
	if r.L == nil {
		return errors.New("runtime closed")
	}
	var packet struct {
		Revision   uint64 `json:"revision"`
		Operations []struct {
			Op    string          `json:"op"`
			ID    string          `json:"id"`
			Props map[string]any  `json:"props"`
			Tree  json.RawMessage `json:"tree"`
		} `json:"operations"`
	}
	if err := strictJSON(data, &packet); err != nil {
		return err
	}
	if packet.Revision != r.Revision {
		return protocolError(CodeRevisionConflict, fmt.Errorf("stale revision: have %d", r.Revision))
	}
	if len(packet.Operations) > 256 {
		return protocolError(CodeValidationError, errors.New("too many operations"))
	}
	ops := make([]Operation, 0, len(packet.Operations))
	for i, p := range packet.Operations {
		switch p.Op {
		case "patch":
			ops = append(ops, Operation{ID: p.ID, Props: p.Props})
		case "replace":
			var tree NodeJSON
			if err := strictJSON(p.Tree, &tree); err != nil {
				return fmt.Errorf("operations[%d].tree: %w", i+1, err)
			}
			ops = append(ops, Operation{ID: p.ID, Replace: true, Tree: tree.node()})
		default:
			return protocolError(CodeValidationError, fmt.Errorf("operations[%d]: unknown operation %q", i+1, p.Op))
		}
	}
	r.Staged = ops
	return protocolError(CodeValidationError, r.Commit())
}

type NodeJSON struct {
	Type     string         `json:"type"`
	ID       string         `json:"id"`
	Props    map[string]any `json:"props"`
	Children []NodeJSON     `json:"children"`
}

func (j NodeJSON) node() *Node {
	n := &Node{Type: j.Type, ID: j.ID, Props: j.Props}
	if n.Props == nil {
		n.Props = map[string]any{}
	}
	for _, c := range j.Children {
		n.Children = append(n.Children, c.node())
	}
	return n
}

func (r *Runtime) Snapshot() ([]byte, error) {
	type item struct {
		ID, Type string
		Bounds   Rect
		Children []string
	}
	items := []item{}
	for n := range ordered(r.Root) {
		i := item{ID: n.ID, Type: n.Type, Bounds: n.Rect}
		for _, c := range n.Children {
			i.Children = append(i.Children, c.ID)
		}
		items = append(items, i)
	}
	return json.Marshal(struct {
		ProtocolVersion int `json:"protocol_version"`
		Revision        uint64
		Focus           string
		Nodes           []item
	}{1, r.Revision, r.Focus, items})
}
