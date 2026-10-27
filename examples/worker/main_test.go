package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/VintlGvard/LugoTUI/tui"
)

func TestCloseCancelsWorker(t *testing.T) {
	source, err := os.ReadFile("app.lua")
	if err != nil {
		t.Fatal(err)
	}
	r, err := tui.Load(string(source))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cancelled := make(chan struct{})
	_, err = r.StartTask("finished", func(ctx context.Context) (map[string]any, error) {
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel worker context")
	}
}

func TestExample(t *testing.T) {
	source, err := os.ReadFile("app.lua")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, status, result string }{
		{"hello", "Done", "HELLO"}, {"fail", "Error", "controlled failure"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			r, err := tui.Load(string(source))
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.SetFocus("input"); err != nil {
				t.Fatal(err)
			}
			if err := r.Paste(tc.input); err != nil {
				t.Fatal(err)
			}
			if err := r.Action("run", nil); err != nil {
				t.Fatal(err)
			}
			h := handler()
			for _, event := range r.DrainEvents() {
				if err := h(r, event); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case result := <-r.TaskResults():
				if err := r.HandleTask(result); err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("task timeout")
			}
			if got := r.Node("status").Props["text"]; got != tc.status {
				t.Fatalf("status = %v", got)
			}
			if got := r.Node("result").Props["text"]; got != tc.result {
				t.Fatalf("result = %v", got)
			}
			var reply struct {
				OK     bool `json:"ok"`
				Result struct {
					Version  int `json:"protocol_version"`
					Revision uint64
					Nodes    []struct{ ID, Type string }
				} `json:"result"`
			}
			if err := json.Unmarshal(r.AgentReply([]byte(`{"method":"snapshot"}`)), &reply); err != nil || !reply.OK || reply.Result.Version != 1 || len(reply.Result.Nodes) != 6 {
				t.Fatalf("snapshot: %v, %v", reply, err)
			}
			packet, err := json.Marshal(map[string]any{"method": "apply", "packet": map[string]any{
				"revision":   reply.Result.Revision,
				"operations": []any{map[string]any{"op": "patch", "id": "status", "props": map[string]any{"text": "Inspected"}}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			var applied struct {
				OK   bool   `json:"ok"`
				Code string `json:"code"`
			}
			if err := json.Unmarshal(r.AgentReply(packet), &applied); err != nil || !applied.OK {
				t.Fatalf("apply: %v, %v", applied, err)
			}
			if err := json.Unmarshal(r.AgentReply([]byte(`{"method":"bogus"}`)), &applied); err != nil || applied.OK || applied.Code != tui.CodeUnknownCommand {
				t.Fatalf("error reply: %v, %v", applied, err)
			}
			if err := r.Action("run", nil); err != nil {
				t.Fatal(err)
			}
			for _, event := range r.DrainEvents() {
				if err := h(r, event); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.Action("cancel", nil); err != nil {
				t.Fatal(err)
			}
			for _, event := range r.DrainEvents() {
				if err := h(r, event); err != nil {
					t.Fatal(err)
				}
			}
			if got := r.Node("status").Props["text"]; got != "Cancelled" {
				t.Fatalf("cancel status = %v", got)
			}
		})
	}
}
