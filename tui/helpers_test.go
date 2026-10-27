package tui_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/tui"
	"github.com/gdamore/tcell/v2"
)

func node(kind, id string, props map[string]any, children ...*tui.Node) *tui.Node {
	return &tui.Node{Type: kind, ID: id, Props: props, Children: children}
}

func mouseFixture(t *testing.T, source string) *tui.Runtime {
	t.Helper()
	r, e := tui.Load(source)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(r.Close)
	s := tcell.NewSimulationScreen("UTF-8")
	if e = s.Init(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Fini)
	s.SetSize(60, 20)
	r.Draw(s, "")
	return r
}

func screenContent(s tcell.SimulationScreen) string {
	contents, w, h := s.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := contents[y*w+x]
			if len(cell.Runes) > 0 {
				b.WriteRune(cell.Runes[0])
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func str(n *tui.Node, key, fallback string) string {
	if n != nil {
		if v, ok := n.Props[key].(string); ok {
			return v
		}
	}
	return fallback
}

func setFocus(t testing.TB, r *tui.Runtime, id string) {
	t.Helper()
	if err := r.SetFocus(id); err != nil {
		t.Fatal(err)
	}
}

func snapshotNodes(t testing.TB, r *tui.Runtime) map[string]*tui.Node {
	t.Helper()
	data, err := r.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var s struct{ Nodes []struct{ ID string } }
	if err = json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	out := map[string]*tui.Node{}
	for _, n := range s.Nodes {
		if node := r.Node(n.ID); node != nil {
			out[n.ID] = node
		}
	}
	return out
}

func rootNode(t testing.TB, r *tui.Runtime) *tui.Node {
	t.Helper()
	data, err := r.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	var s struct{ Nodes []struct{ ID string } }
	if err = json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Nodes) == 0 {
		return nil
	}
	if n := r.Node(s.Nodes[0].ID); n != nil {
		return n
	}
	root := &tui.Node{}
	for _, v := range s.Nodes {
		if n := r.Node(v.ID); n != nil {
			root.Children = append(root.Children, n)
		}
	}
	return root
}

func pasteInto(t testing.TB, r *tui.Runtime, id, text string) {
	t.Helper()
	if err := r.SetFocus(id); err != nil {
		t.Fatal(err)
	}
	if err := r.Paste(text); err != nil {
		t.Fatal(err)
	}
}
