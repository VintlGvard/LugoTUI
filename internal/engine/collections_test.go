package engine

import (
	"github.com/gdamore/tcell/v2"
	"strings"
	"testing"
)

func TestCollectionValidation(t *testing.T) {
	cases := []struct {
		name string
		n    *Node
		bad  bool
	}{
		{"table", &Node{Type: "table", Props: map[string]any{"columns": []any{map[string]any{"title": "Name", "width": float64(8)}}, "rows": []any{[]any{"ok"}}}}, false},
		{"ragged", &Node{Type: "table", Props: map[string]any{"columns": []any{map[string]any{"title": "Name", "width": float64(8)}}, "rows": []any{[]any{"a", "b"}}}}, true},
		{"bad width", &Node{Type: "table", Props: map[string]any{"columns": []any{map[string]any{"title": "Name", "width": 1.5}}}}, true},
		{"unknown field", &Node{Type: "tree", Props: map[string]any{"items": []any{map[string]any{"id": "a", "text": "A", "oops": true}}}}, true},
		{"duplicate id", &Node{Type: "tree", Props: map[string]any{"items": []any{map[string]any{"id": "a", "text": "A", "children": []any{map[string]any{"id": "a", "text": "B"}}}}}}, true},
		{"highlight", &Node{Type: "viewport", Props: map[string]any{"highlights": []any{map[string]any{"start": float64(3), "end": float64(2)}}}}, true},
		{"limit", &Node{Type: "viewport", Props: map[string]any{"text": strings.Repeat("x", maxText+1)}}, true},
		{"file path", &Node{Type: "filepicker", Props: map[string]any{"items": []any{map[string]any{"text": "file"}}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateCollection(tc.n); (err != nil) != tc.bad {
				t.Fatalf("error=%v bad=%v", err, tc.bad)
			}
		})
	}
}
func TestCollectionTreeFilterClone(t *testing.T) {
	r := &Runtime{State: map[string]*WidgetState{}, Data: map[string]any{}}
	tree := &Node{ID: "tree", Type: "tree", Props: map[string]any{"items": []any{map[string]any{"id": "a", "text": "A", "children": []any{map[string]any{"id": "b", "text": "B"}}}}}, Rect: Rect{W: 20, H: 8}}
	s, c := r.collectionState(tree)
	if len(r.collectionItems(tree, c)) != 1 {
		t.Fatal("collapsed tree")
	}
	if _, err := r.keyCollection(tree, tcell.NewEventKey(tcell.KeyRight, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if len(r.collectionItems(tree, c)) != 2 {
		t.Fatal("expand")
	}
	r.keyCollection(tree, tcell.NewEventKey(tcell.KeyDown, 0, 0))
	r.keyCollection(tree, tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if s.Selected != 0 {
		t.Fatal("parent navigation")
	}
	r.keyCollection(tree, tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if len(r.collectionItems(tree, c)) != 1 {
		t.Fatal("collapse")
	}
	clone := cloneCollectionState(c)
	clone.Expanded["a"] = true
	if c.Expanded["a"] {
		t.Fatal("clone aliases expansion")
	}
	list := &Node{ID: "list", Type: "list", Props: map[string]any{"filterable": true, "items": []any{"alphabet", map[string]any{"text": "ab", "description": "best"}, "zzz"}}, Rect: Rect{W: 20, H: 10}}
	_, lc := r.collectionState(list)
	r.keyCollection(list, tcell.NewEventKey(tcell.KeyRune, '/', 0))
	r.keyCollection(list, tcell.NewEventKey(tcell.KeyRune, 'a', 0))
	r.keyCollection(list, tcell.NewEventKey(tcell.KeyRune, 'b', 0))
	r.keyCollection(list, tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	items := r.collectionItems(list, lc)
	if len(items) != 2 || items[0].text != "ab" || items[0].index != 2 {
		t.Fatalf("fuzzy ranking: %+v", items)
	}
}
func TestCollectionPaintScrollAndMouse(t *testing.T) {
	r, err := Load(`local ui=require('ui'); return {view=ui.viewport{id='v',text='界a\nsecond\nthird',width=5,height=2,highlights={{start=1,['end']=1}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(5, 2)
	r.Draw(screen, "")
	ch, _, style, w := screen.GetContent(0, 0)
	_, _, attrs := style.Decompose()
	if ch != '界' || w != 2 || attrs&tcell.AttrReverse == 0 {
		t.Fatalf("grapheme/highlight %q %d %v", ch, w, attrs)
	}
	r.Key(tcell.NewEventKey(tcell.KeyEnd, 0, 0))
	r.Draw(screen, "")
	ch, _, _, _ = screen.GetContent(0, 0)
	if ch != 's' {
		t.Fatalf("end scroll %q", ch)
	}
	r.mouseCollection(r.Nodes["v"], tcell.NewEventMouse(1, 1, tcell.WheelUp, 0))
	if r.State["v"].Scroll != 0 {
		t.Fatal("wheel")
	}
	n := &Node{ID: "table", Type: "table", Props: map[string]any{"columns": []any{map[string]any{"title": "C", "width": float64(3)}}, "rows": []any{[]any{"界x"}, []any{"two"}}}, Rect: Rect{W: 5, H: 2}, Clip: Rect{W: 5, H: 2}}
	r.paintCollection(screen, n, tcell.StyleDefault, n.Rect, n.Clip)
	ch, _, _, _ = screen.GetContent(0, 0)
	if ch != 'C' {
		t.Fatal("table header")
	}
	r.keyCollection(n, tcell.NewEventKey(tcell.KeyDown, 0, 0))
	r.paintCollection(screen, n, tcell.StyleDefault, n.Rect, n.Clip)
	ch, _, _, _ = screen.GetContent(0, 1)
	if ch != 't' {
		t.Fatal("table selection scroll")
	}
}
func TestCollectionFileRestrictions(t *testing.T) {
	n := &Node{Type: "filepicker", Props: map[string]any{"allowed_extensions": []any{".go"}}}
	if !collectionFileAllowed(n, collectionItem{path: "a.go"}) || collectionFileAllowed(n, collectionItem{path: "a.txt"}) || collectionFileAllowed(n, collectionItem{path: "dir", dir: true}) {
		t.Fatal("file restrictions")
	}
	n.Props["dir_allowed"] = true
	if !collectionFileAllowed(n, collectionItem{dir: true}) {
		t.Fatal("dir allowed")
	}
}
