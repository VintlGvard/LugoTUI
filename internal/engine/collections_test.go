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

func TestViewportLineNumbersSoftWrapAndHorizontalClip(t *testing.T) {
	r, err := Load(`local ui = require('ui')
return {
  view = ui.column {
    id = 'root',
    ui.viewport { id = 'vp_wrap', width = 10, height = 3, soft_wrap = true, show_line_numbers = true, text = 'abcdefgh\nijklmnop\nend' },
    ui.viewport { id = 'vp_horiz', width = 10, height = 2, soft_wrap = false, show_line_numbers = true, text = '1234567890abcdef' }
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(10, 5)
	r.Draw(screen, "")

	r0c1, _, _, _ := screen.GetContent(1, 0)
	r1c1, _, _, _ := screen.GetContent(1, 1)
	r2c1, _, _, _ := screen.GetContent(1, 2)
	if r0c1 != '1' || r1c1 != ' ' || r2c1 != '2' {
		t.Fatalf("expected logical line numbers [1, ' ', 2] on wrapped lines, got [%q, %q, %q]", r0c1, r1c1, r2c1)
	}

	if err := r.SetFocus("vp_wrap"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnd, 0, 0)); err != nil {
		t.Fatal(err)
	}
	r.Draw(screen, "")
	if r.State["vp_wrap"].Scroll != 2 {
		t.Fatalf("expected vp_wrap KeyEnd to reach scroll=2 (gutter-aware), got %d", r.State["vp_wrap"].Scroll)
	}

	if err := r.SetFocus("vp_horiz"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0)); err != nil {
			t.Fatal(err)
		}
	}
	r.Draw(screen, "")
	numCell, _, _, _ := screen.GetContent(1, 3)
	textCell, _, _, _ := screen.GetContent(3, 3)
	if numCell != '1' || textCell != '5' {
		t.Fatalf("expected line number '1' preserved and scrolled text '5' at x=3, got (%q, %q)", numCell, textCell)
	}
}

func TestFilepickerParentEntryBackspaceAndMouseDragGuard(t *testing.T) {
	r, err := Load(`local ui = require('ui')
return {
  view = ui.column {
    id = 'root',
    ui.filepicker {
      id = 'fp',
      height = 5,
      current_dir = '/a/b',
      items = {
        { text = '../', path = '..', is_dir = true },
        { text = '.env', path = '/a/b/.env', is_dir = false },
        { text = 'main.go', path = '/a/b/main.go', is_dir = false }
      },
      on_navigate = 'nav',
      on_select = 'sel'
    },
    ui.tree {
      id = 'tr',
      height = 4,
      items = { { id = 'n1', text = 'root', children = { { id = 'n2', text = 'leaf' } } } },
      on_select = 'sel'
    }
  },
  actions = {
    nav = function(e) host.dispatch('nav', e) end,
    sel = function(e) host.dispatch('sel', e) end
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, fpCol := r.collectionState(r.Nodes["fp"])
	fpItems := r.collectionItems(r.Nodes["fp"], fpCol)
	if len(fpItems) != 2 || fpItems[0].path != ".." {
		t.Fatalf("expected '..' visible and '.env' hidden, got %+v", fpItems)
	}

	if err := r.SetFocus("fp"); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyBackspace2, 0, 0)); err != nil {
		t.Fatal(err)
	}
	ev := r.DrainEvents()
	if len(ev) != 1 || ev[0].Action != "nav" || ev[0].Payload.(map[string]any)["path"] != "/a" {
		t.Fatalf("expected backspace navigation to /a, got %+v", ev)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 12)
	r.Draw(screen, "")

	trY := r.Nodes["tr"].Rect.Y
	if err := r.Mouse(tcell.NewEventMouse(0, trY, tcell.Button1, 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Mouse(tcell.NewEventMouse(10, trY, tcell.Button1, 0)); err != nil {
		t.Fatal(err)
	}
	if err := r.Mouse(tcell.NewEventMouse(10, trY, tcell.ButtonNone, 0)); err != nil {
		t.Fatal(err)
	}
	if selEv := r.DrainEvents(); len(selEv) != 0 {
		t.Fatalf("expected tree toggle without drag activation, got %+v", selEv)
	}
}

func TestCollectionValidationAndTableTitleWidth(t *testing.T) {
	tbl := Table([]TableColumn{{Title: "Привет"}}, [][]string{{"ok"}})
	cols := tbl.Props["columns"].([]any)
	if w := cols[0].(map[string]any)["width"].(int); w != 8 {
		t.Fatalf("expected display width 8 for 'Привет', got %d", w)
	}

	badProps := &Node{Type: "text", ID: "t", Props: map[string]any{"columns": []any{}}}
	if validateCollection(badProps) == nil {
		t.Fatal("expected collection property on text node to fail")
	}

	badCell := &Node{Type: "table", ID: "tbl", Props: map[string]any{
		"columns": []any{map[string]any{"title": "A", "width": float64(5)}},
		"rows":    []any{[]any{map[string]any{"text": "x", "bold": "nope"}}},
	}}
	if validateCollection(badCell) == nil {
		t.Fatal("expected non-boolean table cell bold to fail")
	}
}
