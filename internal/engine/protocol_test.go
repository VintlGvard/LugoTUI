package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func protocolRuntime(t *testing.T) *Runtime {
	t.Helper()
	r, err := Load(`local ui=require('ui'); return {view=ui.text{id='text',text='before'},actions={done=function() ui.patch('text',{text='after'}) end, fail=function() error('boom') end, invalid=function() ui.patch('text',{width=-1}) end}}`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

func TestProtocolCodes(t *testing.T) {
	r := protocolRuntime(t)
	cases := []struct{ request, code string }{
		{`{`, CodeInvalidJSON},
		{`{"extra":1,`, CodeInvalidJSON},
		{`{} {}`, CodeInvalidJSON},
		{`{"method":"snapshot","extra":1}`, CodeValidationError},
		{`{"method":1}`, CodeValidationError},
		{`{"method":"missing"}`, CodeUnknownCommand},
		{`{"method":"message","action":"missing"}`, CodeValidationError},
		{`{"method":"message","action":"fail"}`, CodeLuaError},
		{`{"method":"apply","packet":{"revision":999}}`, CodeRevisionConflict},
		{`{"method":"apply","packet":{"operations":[{"op":"missing"}]}}`, CodeValidationError},
		{`{"method":"apply","packet":{"operations":[{"op":"patch","id":"missing"}]}}`, CodeValidationError},
		{`{"method":"apply","packet":{"operations":[{"op":"replace","id":"text","tree":{"extra":1}}]}}`, CodeValidationError},
	}
	for _, tc := range cases {
		t.Run(tc.request, func(t *testing.T) {
			var reply map[string]any
			if err := json.Unmarshal(r.AgentReply([]byte(tc.request)), &reply); err != nil {
				t.Fatal(err)
			}
			if reply["ok"] != false || reply["code"] != tc.code || reply["error"] == "" {
				t.Fatalf("reply: %v; want %s", reply, tc.code)
			}
		})
	}
	err := r.ApplyJSON([]byte(`{"revision":999}`))
	var detail *ProtocolError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &detail) || ErrorCode(err) != CodeRevisionConflict || detail.Unwrap() == nil || err.Error() != "stale revision: have 0" {
		t.Fatalf("revision error: %v", err)
	}
	r.Close()
	var reply map[string]any
	if err := json.Unmarshal(r.AgentReply([]byte(`{"method":"message","action":"done"}`)), &reply); err != nil {
		t.Fatal(err)
	}
	if reply["code"] != CodeInternalError {
		t.Fatalf("closed reply: %v", reply)
	}
}

func TestProtocolWrappedNodeError(t *testing.T) {
	err := fmt.Errorf("action: %w", &nodeError{Err: errors.New("bad property")})
	if ErrorCode(err) != CodeValidationError {
		t.Fatalf("node error code: %s", ErrorCode(err))
	}
}

func TestProtocolSnapshotAndMarshalFailure(t *testing.T) {
	r := protocolRuntime(t)
	var reply struct {
		OK     bool                       `json:"ok"`
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(r.AgentReply([]byte(`{"method":"snapshot"}`)), &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.OK || string(reply.Result["protocol_version"]) != "1" {
		t.Fatalf("snapshot: %+v", reply)
	}
	for _, key := range []string{"Revision", "Focus", "Nodes"} {
		if _, ok := reply.Result[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	var failure map[string]any
	if err := json.Unmarshal(replyJSON(make(chan int), nil), &failure); err != nil {
		t.Fatal(err)
	}
	if failure["code"] != CodeInternalError {
		t.Fatal(failure)
	}
}

func TestTaskPanicAndLimitCodes(t *testing.T) {
	r := protocolRuntime(t)
	id, err := r.StartTask("done", func(context.Context) (map[string]any, error) { panic("boom") })
	if err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-r.TaskResults():
		if result.ID != id || result.Err == nil || !strings.Contains(result.Err.Error(), "task panic: boom") {
			t.Fatalf("result: %+v", result)
		}
		if err := r.HandleTask(result); err != result.Err || ErrorCode(err) != CodeInternalError {
			t.Fatalf("callback error: %v", err)
		}
		if len(r.tasks.active) != 0 || str(r.Nodes["text"], "text", "") != "before" {
			t.Fatal("failed task invoked action or leaked slot")
		}
	case <-time.After(time.Second):
		t.Fatal("panic result missing")
	}
	for i := 0; i < 32; i++ {
		_, err := r.StartTask("done", func(ctx context.Context) (map[string]any, error) { <-ctx.Done(); return nil, ctx.Err() })
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = r.StartTask("done", func(context.Context) (map[string]any, error) { return nil, nil })
	if ErrorCode(fmt.Errorf("wrapped: %w", err)) != CodeTaskLimitExceeded || err.Error() != "32 task limit reached" {
		t.Fatalf("limit: %v", err)
	}
	results := r.TaskResults()
	r.Close()
	select {
	case _, ok := <-results:
		if !ok {
			t.Fatal("results channel closed")
		}
	default:
	}
}
