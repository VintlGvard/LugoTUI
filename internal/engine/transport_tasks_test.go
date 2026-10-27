package engine

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".sock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "agent.sock")
}

func TestAgentPacketTextLimit(t *testing.T) {
	requests := make(chan AgentRequest, 1)
	path := shortSocketPath(t)
	stop, err := ServeAgent(path, requests, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	for _, tc := range []struct {
		name, ending string
		size         int
	}{
		{"LF", "\n", maxText},
		{"CRLF", "\r\n", maxText},
		{"oversize", "\n", maxText + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.Dial("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			packet := append(bytes.Repeat([]byte(" "), tc.size-2), '{', '}')
			if _, err := conn.Write(append(packet, tc.ending...)); err != nil {
				t.Fatal(err)
			}
			if tc.size > maxText {
				if _, err := bufio.NewReader(conn).ReadString('\n'); err == nil {
					t.Fatal("oversized packet accepted")
				} else if err, ok := err.(net.Error); ok && err.Timeout() {
					t.Fatal("oversized packet did not close connection")
				}
				select {
				case <-requests:
					t.Fatal("oversized packet forwarded")
				default:
				}
				return
			}
			select {
			case request := <-requests:
				if !bytes.Equal(request.Data, packet) {
					t.Fatalf("packet length = %d, want %d", len(request.Data), len(packet))
				}
				request.Reply <- []byte("{}")
			case <-time.After(time.Second):
				t.Fatal("maximum-size packet rejected")
			}
			if _, err := bufio.NewReader(conn).ReadString('\n'); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskCompletionReleasesContext(t *testing.T) {
	r := protocolRuntime(t)
	contexts := make(chan context.Context, 1)
	_, err := r.StartTask("done", func(ctx context.Context) (map[string]any, error) {
		contexts <- ctx
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var result TaskResult
	select {
	case result = <-r.TaskResults():
	case <-time.After(time.Second):
		t.Fatal("task result missing")
	}
	ctx := <-contexts
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("completed task retained live context until HandleTask")
	}
	if err := r.HandleTask(result); err != nil {
		t.Fatal(err)
	}
	if str(r.Nodes["text"], "text", "") != "after" || len(r.tasks.active) != 0 {
		t.Fatal("context cleanup discarded completed result or retained task slot")
	}
}

func TestAgentReplyDoesNotMutateBuffer(t *testing.T) {
	requests := make(chan AgentRequest, 1)
	path := shortSocketPath(t)
	stop, err := ServeAgent(path, requests, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	var request AgentRequest
	select {
	case request = <-requests:
	case <-time.After(time.Second):
		t.Fatal("request missing")
	}
	buffer := []byte("{}!")
	request.Reply <- buffer[:2]
	if reply, err := bufio.NewReader(conn).ReadString('\n'); err != nil || reply != "{}\n" {
		t.Fatalf("reply = %q, error = %v", reply, err)
	}
	stop()
	if string(buffer) != "{}!" {
		t.Fatalf("reply buffer mutated: %q", buffer)
	}
}
