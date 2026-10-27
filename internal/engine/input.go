package engine

import (
	"cmp"
	"errors"
	"iter"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func sortedChildren(n *Node) []*Node {
	compare := func(a, b *Node) int {
		return cmp.Compare(num(a, "z_index", 0), num(b, "z_index", 0))
	}
	if slices.IsSortedFunc(n.Children, compare) {
		return n.Children
	}
	children := slices.Clone(n.Children)
	slices.SortStableFunc(children, compare)
	return children
}

func (r *Runtime) focusStep(back bool) {
	nodes := []*Node{}
	r.ReconcileFocus()
	for n := range visibleNodes(r.scope()) {
		if focusable(n) && n.Clip.W > 0 && n.Clip.H > 0 {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return
	}
	index := 0
	for i, n := range nodes {
		if n.ID == r.Focus {
			index = i
			break
		}
	}
	delta := 1
	if back {
		delta = -1
	}
	r.Focus = nodes[(index+delta+len(nodes))%len(nodes)].ID
}

func visibleNodes(n *Node) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		var walk func(*Node) bool
		walk = func(n *Node) bool {
			if flag(n, "hidden") {
				return true
			}
			if !yield(n) {
				return false
			}
			for _, child := range sortedChildren(n) {
				if !walk(child) {
					return false
				}
			}
			return true
		}
		walk(n)
	}
}

func (r *Runtime) scope() *Node {
	root := r.Root
	for n := range visibleNodes(r.Root) {
		if flag(n, "modal") {
			root = n
		}
	}
	return root
}

func (r *Runtime) ReconcileFocus() {
	scope := r.scope()
	if scope.ID != r.modalID {
		if r.modalID == "" && scope != r.Root {
			r.PreviousFocus = r.Focus
		}
		if scope == r.Root && r.PreviousFocus != "" {
			r.Focus = r.PreviousFocus
			r.PreviousFocus = ""
		}
		r.modalID = ""
		if scope != r.Root {
			r.modalID = scope.ID
		}
	}
	for n := range visibleNodes(scope) {
		if n.ID == r.Focus && focusable(n) {
			return
		}
	}
	r.Focus = ""
	for n := range visibleNodes(scope) {
		if focusable(n) {
			r.Focus = n.ID
			return
		}
	}
}

func (r *Runtime) Key(e *tcell.EventKey) error {
	r.ReconcileFocus()
	action := r.keymap[e.Name()]
	if action == "" {
		action = r.keymap[strings.ReplaceAll(e.Name(), "-", "+")]
	}
	if action != "" {
		return r.Action(action, map[string]any{"key": e.Name(), "focus": r.Focus})
	}
	if e.Key() == tcell.KeyEscape && r.scope() != r.Root {
		n := r.scope()
		r.Staged = []Operation{{ID: n.ID, Props: map[string]any{"hidden": true}}}
		return r.Commit()
	}
	if handled, err := r.bindingKey(e); handled || err != nil {
		return err
	}
	if handled, err := r.editorKey(r.Nodes[r.Focus], e); handled || err != nil {
		return err
	}
	if e.Key() == tcell.KeyTab || e.Key() == tcell.KeyBacktab {
		r.focusStep(e.Key() == tcell.KeyBacktab)
		return nil
	}
	n := r.Nodes[r.Focus]
	if n == nil {
		return nil
	}
	s := r.State[n.ID]
	if handled, err := r.keyCollection(n, e); handled || err != nil {
		return err
	}
	if handled, err := r.keyPrimitive(n, e); handled || err != nil {
		return err
	}
	if n.Type == "button" && (e.Key() == tcell.KeyEnter || (e.Key() == tcell.KeyRune && e.Rune() == ' ')) {
		return r.Action(str(n, "on_click", ""), map[string]any{"id": n.ID})
	}
	if n.Type == "list" {
		items := r.items(n)
		s.Selected = max(0, min(s.Selected, len(items)-1))
		switch e.Key() {
		case tcell.KeyHome:
			s.Selected = 0
		case tcell.KeyEnd:
			s.Selected = max(0, len(items)-1)
		case tcell.KeyPgUp:
			s.Selected = max(0, s.Selected-max(1, contentRect(n).H))
		case tcell.KeyPgDn:
			s.Selected = min(max(0, len(items)-1), s.Selected+max(1, contentRect(n).H))
		case tcell.KeyUp:
			s.Selected = max(0, s.Selected-1)
		case tcell.KeyDown:
			s.Selected = min(max(0, len(items)-1), s.Selected+1)
		case tcell.KeyEnter:
			if len(items) == 0 {
				return nil
			}
			var value any
			if len(items) > 0 {
				value = items[s.Selected]
			}
			return r.Action(str(n, "on_select", ""), map[string]any{"id": n.ID, "index": s.Selected + 1, "value": value})
		}
		return nil
	}
	if n.Type == "scroll" {
		switch e.Key() {
		case tcell.KeyUp:
			s.Scroll = max(0, s.Scroll-1)
		case tcell.KeyDown:
			s.Scroll++
		case tcell.KeyPgUp:
			s.Scroll = max(0, s.Scroll-max(1, contentRect(n).H))
		case tcell.KeyPgDn:
			s.Scroll += max(1, contentRect(n).H)
		case tcell.KeyHome:
			s.Scroll = 0
		}
		return nil
	}
	if n.Type != "input" && n.Type != "textarea" {
		return nil
	}
	g := graphemes(s.Value)
	s.Cursor = max(0, min(s.Cursor, len(g)))
	old := s.Cursor
	shift := e.Modifiers()&tcell.ModShift != 0
	switch e.Key() {
	case tcell.KeyLeft:
		s.Cursor = max(0, s.Cursor-1)
	case tcell.KeyRight:
		s.Cursor = min(len(g), s.Cursor+1)
	case tcell.KeyHome:
		s.Cursor = 0
		if n.Type == "textarea" {
			s.Cursor = old
			for s.Cursor > 0 && g[s.Cursor-1] != "\n" {
				s.Cursor--
			}
		}
	case tcell.KeyEnd:
		s.Cursor = len(g)
		if n.Type == "textarea" {
			s.Cursor = old
			for s.Cursor < len(g) && g[s.Cursor] != "\n" {
				s.Cursor++
			}
		}
	case tcell.KeyUp, tcell.KeyDown:
		step := -1
		if e.Key() == tcell.KeyDown {
			step = 1
		}
		_, positions := editorIndex(n, s, editorRect(n, s).W)
		p := positions[old]
		s.Cursor = nearestPosition(positions, p[0], max(0, p[1]+step))
	case tcell.KeyCtrlA:
		s.Anchor = 0
		s.Cursor = len(g)
		return nil
	case tcell.KeyCtrlZ:
		if len(s.Undo) > 0 {
			s.Redo = append(s.Redo, s.Value)
			s.Value = s.Undo[len(s.Undo)-1]
			s.Undo = s.Undo[:len(s.Undo)-1]
			s.Cursor = len(graphemes(s.Value))
			s.Anchor = -1
			return r.changed(n, s)
		}
		return nil
	case tcell.KeyCtrlY:
		if len(s.Redo) > 0 {
			s.Undo = append(s.Undo, s.Value)
			s.Value = s.Redo[len(s.Redo)-1]
			s.Redo = s.Redo[:len(s.Redo)-1]
			s.Cursor = len(graphemes(s.Value))
			s.Anchor = -1
			return r.changed(n, s)
		}
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.Anchor < 0 || s.Anchor == s.Cursor {
			if s.Cursor == 0 {
				return nil
			}
			s.Anchor = s.Cursor - 1
		}
		editSave(s)
		replaceSelection(s, "")
		return r.changed(n, s)
	case tcell.KeyDelete:
		if s.Anchor < 0 || s.Anchor == s.Cursor {
			if s.Cursor == len(g) {
				return nil
			}
			s.Anchor = s.Cursor + 1
		}
		editSave(s)
		replaceSelection(s, "")
		return r.changed(n, s)
	case tcell.KeyEnter:
		if n.Type == "input" || e.Modifiers()&tcell.ModCtrl != 0 {
			return r.Action(str(n, "on_submit", ""), map[string]any{"id": n.ID, "value": s.Value, "error": editorValidation(n, s.Value)})
		}
	}
	if s.Cursor != old {
		if shift && s.Anchor < 0 {
			s.Anchor = old
		} else if !shift {
			s.Anchor = -1
		}
	}
	return nil
}

func (r *Runtime) Mouse(e *tcell.EventMouse) error {
	x, y := e.Position()
	down := e.Buttons()&tcell.Button1 != 0
	pressed := down && !r.mouseDown
	r.mouseDown = down
	var hit *Node
	var scroll *Node
	var walk func(*Node)
	walk = func(n *Node) {
		if flag(n, "hidden") || x < n.Clip.X || x >= n.Clip.X+n.Clip.W || y < n.Clip.Y || y >= n.Clip.Y+n.Clip.H {
			return
		}
		hit = n
		if n.Type == "scroll" {
			scroll = n
		}
		for _, c := range sortedChildren(n) {
			walk(c)
		}
	}
	r.ReconcileFocus()
	if r.dragID != "" && e.Buttons()&tcell.Button1 != 0 {
		if n := r.Nodes[r.dragID]; n != nil {
			r.mouseCursor(n, x, y)
		}
		return nil
	}
	r.releaseMouse(e)
	walk(r.scope())
	if hit == nil {
		return nil
	}
	if e.Buttons()&(tcell.WheelDown|tcell.WheelUp) != 0 && hit.Type != "list" && hit.Type != "table" && hit.Type != "viewport" && hit.Type != "tree" && hit.Type != "filepicker" && scroll != nil {
		hit = scroll
	}
	s := r.State[hit.ID]
	if s == nil {
		return nil
	}
	if pressed && focusable(hit) {
		r.Focus = hit.ID
	}
	if handled, err := r.mouseCollection(hit, e); handled || err != nil {
		return err
	}
	if pressed && focusable(hit) {
		if hit.Type == "input" || hit.Type == "textarea" {
			r.mouseCursor(hit, x, y)
			s.Anchor = s.Cursor
			r.dragID = hit.ID
			return nil
		}
		if hit.Type == "button" {
			return r.Action(str(hit, "on_click", ""), map[string]any{"id": hit.ID})
		}
		if hit.Type == "list" {
			items := r.items(hit)
			index := y - contentRect(hit).Y + s.Scroll
			if index < 0 || index >= len(items) {
				return nil
			}
			s.Selected = index
			return r.Action(str(hit, "on_select", ""), map[string]any{"id": hit.ID, "index": index + 1, "value": items[index]})
		}
	}
	if hit.Type == "scroll" {
		if e.Buttons()&tcell.WheelDown != 0 {
			s.Scroll += 3
		}
		if e.Buttons()&tcell.WheelUp != 0 {
			s.Scroll = max(0, s.Scroll-3)
		}
	}
	if hit.Type == "list" {
		if e.Buttons()&tcell.WheelDown != 0 {
			s.Selected = min(max(0, len(r.items(hit))-1), s.Selected+3)
		}
		if e.Buttons()&tcell.WheelUp != 0 {
			s.Selected = max(0, s.Selected-3)
		}
	}
	return nil
}

func (r *Runtime) changed(n *Node, s *WidgetState) error {
	if b := str(n, "bind", ""); b != "" {
		r.Data[b] = s.Value
	}
	r.Revision++
	return r.Action(str(n, "on_change", ""), map[string]any{"id": n.ID, "value": s.Value, "error": editorValidation(n, s.Value)})
}

func editSave(s *WidgetState) {
	s.Undo = append(s.Undo, s.Value)
	bytes := 0
	for _, v := range s.Undo {
		bytes += len(v)
	}
	for len(s.Undo) > 1 && (len(s.Undo) > 64 || bytes > maxText) {
		bytes -= len(s.Undo[0])
		s.Undo[0] = ""
		s.Undo = s.Undo[1:]
	}
	s.Redo = nil
}

func replaceSelection(s *WidgetState, text string) {
	g := graphemes(s.Value)
	a, b := s.Cursor, s.Cursor
	if s.Anchor >= 0 {
		a, b = min(a, s.Anchor), max(b, s.Anchor)
	}
	a = max(0, min(a, len(g)))
	b = max(a, min(b, len(g)))
	s.Value = strings.Join(g[:a], "") + text + strings.Join(g[b:], "")
	s.Cursor = a + len(graphemes(text))
	s.Anchor = -1
}

var errInputLimit = errors.New("input exceeds 1 MiB")

func editorPositions(g []string, width int, multiline bool) [][2]int {
	p := make([][2]int, len(g)+1)
	x, y := 0, 0
	for i, s := range g {
		if multiline && s != "\n" && x+textWidth(s) > width && x > 0 {
			x = 0
			y++
		}
		p[i] = [2]int{x, y}
		if s == "\n" && multiline {
			x = 0
			y++
		} else {
			x += textWidth(s)
		}
	}
	p[len(g)] = [2]int{x, y}
	return p
}

func nearestPosition(p [][2]int, x, y int) int {
	best, rowDist, columnDist := 0, math.MaxInt, math.MaxInt
	for i, c := range p {
		dy, dx := abs(c[1]-y), abs(c[0]-x)
		if dy < rowDist || dy == rowDist && dx < columnDist {
			best, rowDist, columnDist = i, dy, dx
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (r *Runtime) Paste(text string) error {
	r.ReconcileFocus()
	n := r.Nodes[r.Focus]
	if n == nil || (n.Type != "input" && n.Type != "textarea") {
		return nil
	}
	if n.Type == "input" {
		var clean strings.Builder
		clean.Grow(len(text))
		for _, c := range text {
			if c == '\n' || c == '\r' {
				clean.WriteByte(' ')
			} else if c >= 32 || c == '\t' {
				clean.WriteRune(c)
			}
		}
		text = clean.String()
	}
	s := r.State[n.ID]
	changed, err := editorInsert(n, s, text)
	if err != nil || !changed {
		return err
	}
	return r.changed(n, s)
}

func (r *Runtime) mouseCursor(n *Node, x, y int) {
	s := r.State[n.ID]
	cr := editorRect(n, s)
	x -= cr.X
	y -= cr.Y
	if n.Type == "input" {
		x += s.Scroll
		y = 0
	} else {
		y += s.Scroll
	}
	_, positions := editorIndex(n, s, cr.W)
	s.Cursor = nearestPosition(positions, x, y)
}

func (r *Runtime) releaseMouse(e *tcell.EventMouse) bool {
	if e.Buttons()&tcell.Button1 == 0 {
		r.dragID = ""
		return true
	}
	return false
}

func (s *WidgetState) EditorIndex(width int, multiline bool) ([]string, [][2]int) {
	if !s.indexed || s.indexedValue != s.Value {
		s.clusters = graphemes(s.Value)
		s.positions = nil
		s.indexedValue = s.Value
	}
	if s.positions == nil || s.indexedWidth != width || s.indexedMultiline != multiline {
		s.positions = editorPositions(s.clusters, width, multiline)
		s.indexedWidth = width
		s.indexedMultiline = multiline
	}
	s.indexed = true
	return s.clusters, s.positions
}

func visibleEditorRange(positions [][2]int, scroll, height int) (int, int) {
	start := sort.Search(len(positions)-1, func(i int) bool { return positions[i][1] >= scroll })
	end := sort.Search(len(positions)-1, func(i int) bool { return positions[i][1] >= scroll+height })
	return start, max(start, end)
}
