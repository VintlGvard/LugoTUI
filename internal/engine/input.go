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

func (r *Runtime) nextFocusable(back bool) string {
	nodes := []*Node{}
	r.ReconcileFocus()
	for n := range visibleNodes(r.scope()) {
		if focusable(n) && n.Clip.W > 0 && n.Clip.H > 0 {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return ""
	}
	current := r.Focus
	if r.stagedFocus != nil {
		current = *r.stagedFocus
	}
	index := 0
	for i, n := range nodes {
		if n.ID == current {
			index = i
			break
		}
	}
	delta := 1
	if back {
		delta = -1
	}
	return nodes[(index+delta+len(nodes))%len(nodes)].ID
}

func (r *Runtime) focusStep(back bool) {
	if next := r.nextFocusable(back); next != "" {
		r.Focus = next
	}
}

func explicitNavTarget(n *Node, dir string) (string, bool) {
	if s := str(n, "nav_"+dir, ""); s != "" {
		if s == "none" || s == "false" {
			return "", true
		}
		return s, false
	}
	if navMap, ok := n.Props["nav"].(map[string]any); ok {
		if s, ok := navMap[dir].(string); ok && s != "" {
			if s == "none" || s == "false" {
				return "", true
			}
			return s, false
		}
	}
	return "", false
}

func (r *Runtime) findDirectionNode(curr *Node, dir string) *Node {
	if curr == nil {
		return nil
	}
	targetID, disabled := explicitNavTarget(curr, dir)
	if disabled {
		return nil
	}
	if targetID != "" {
		for n := range visibleNodes(r.scope()) {
			if n.ID == targetID && focusable(n) && n.Clip.W > 0 && n.Clip.H > 0 {
				return n
			}
		}
		return nil
	}

	c := curr.Clip
	cx1, cy1, cx2, cy2 := float64(c.X), float64(c.Y), float64(c.X+c.W), float64(c.Y+c.H)
	cxMid, cyMid := (cx1+cx2)*0.5, (cy1+cy2)*0.5
	var best *Node
	bestScore := math.MaxFloat64
	for n := range visibleNodes(r.scope()) {
		if n.ID == curr.ID || !focusable(n) || n.Clip.W <= 0 || n.Clip.H <= 0 {
			continue
		}
		nc := n.Clip
		nx1, ny1, nx2, ny2 := float64(nc.X), float64(nc.Y), float64(nc.X+nc.W), float64(nc.Y+nc.H)
		nxMid, nyMid := (nx1+nx2)*0.5, (ny1+ny2)*0.5
		var primary, secondary, overlap float64
		valid := false
		switch dir {
		case "up":
			if ny2 <= cy1 || (nyMid < cyMid && ny1 < cy1) {
				valid, primary = true, cy1-ny2
				if primary < 0 {
					primary = cyMid - nyMid
				}
				overlap = min(cx2, nx2) - max(cx1, nx1)
				if overlap <= 0 {
					secondary = max(0, max(cx1-nx2, nx1-cx2))
				}
				secondary += math.Abs(cxMid-nxMid) * 0.2
			}
		case "down":
			if ny1 >= cy2 || (nyMid > cyMid && ny2 > cy2) {
				valid, primary = true, ny1-cy2
				if primary < 0 {
					primary = nyMid - cyMid
				}
				overlap = min(cx2, nx2) - max(cx1, nx1)
				if overlap <= 0 {
					secondary = max(0, max(cx1-nx2, nx1-cx2))
				}
				secondary += math.Abs(cxMid-nxMid) * 0.2
			}
		case "left":
			if nx2 <= cx1 || (nxMid < cxMid && nx1 < cx1) {
				valid, primary = true, cx1-nx2
				if primary < 0 {
					primary = cxMid - nxMid
				}
				overlap = min(cy2, ny2) - max(cy1, ny1)
				if overlap <= 0 {
					secondary = max(0, max(cy1-ny2, ny1-cy2))
				}
				secondary += math.Abs(cyMid-nyMid) * 0.2
			}
		case "right":
			if nx1 >= cx2 || (nxMid > cxMid && nx2 > cx2) {
				valid, primary = true, nx1-cx2
				if primary < 0 {
					primary = nxMid - cxMid
				}
				overlap = min(cy2, ny2) - max(cy1, ny1)
				if overlap <= 0 {
					secondary = max(0, max(cy1-ny2, ny1-cy2))
				}
				secondary += math.Abs(cyMid-nyMid) * 0.2
			}
		}
		if valid && primary >= 0 {
			score := primary + secondary*2.5
			if overlap > 0 {
				score *= 0.6
			}
			if score < bestScore {
				bestScore, best = score, n
			}
		}
	}
	return best
}

func (r *Runtime) focusDirection(dir string) bool {
	r.ReconcileFocus()
	curr := r.Nodes[r.Focus]
	if curr == nil {
		return false
	}
	target := r.findDirectionNode(curr, dir)
	if target != nil && target.ID != curr.ID {
		r.Focus = target.ID
		return true
	}
	return false
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

func keyAlias(s string) tcell.Key {
	switch strings.ToLower(s) {
	case "enter", "return":
		return tcell.KeyEnter
	case "esc", "escape":
		return tcell.KeyEscape
	case "tab":
		return tcell.KeyTab
	case "backspace":
		return tcell.KeyBackspace2
	case "up":
		return tcell.KeyUp
	case "down":
		return tcell.KeyDown
	case "left":
		return tcell.KeyLeft
	case "right":
		return tcell.KeyRight
	case "home":
		return tcell.KeyHome
	case "end":
		return tcell.KeyEnd
	case "pgup":
		return tcell.KeyPgUp
	case "pgdown":
		return tcell.KeyPgDn
	case "delete":
		return tcell.KeyDelete
	case "insert":
		return tcell.KeyInsert
	default:
		return tcell.KeyRune
	}
}

func keyMatches(pattern string, e *tcell.EventKey) bool {
	name := e.Name()
	if pattern == name || (strings.ContainsRune(name, '-') && pattern == strings.ReplaceAll(name, "-", "+")) {
		return true
	}
	if e.Key() == tcell.KeyRune && (e.Modifiers() == tcell.ModNone || e.Modifiers() == tcell.ModShift) {
		r := e.Rune()
		if (pattern == "Space" || pattern == "space") && r == ' ' {
			return true
		}
		if len(pattern) == 1 && pattern[0] == byte(r) && r < 128 {
			return true
		}
		if pattern == string(r) {
			return true
		}
	}
	if e.Modifiers() == tcell.ModNone {
		if k := keyAlias(pattern); k != tcell.KeyRune && k == e.Key() {
			return true
		}
	}
	return e.Key() != tcell.KeyRune && strings.EqualFold(strings.ReplaceAll(pattern, "+", "-"), strings.ReplaceAll(name, "+", "-"))
}

func (r *Runtime) Key(e *tcell.EventKey) error {
	r.ReconcileFocus()
	scope := r.scope()
	if e.Key() == tcell.KeyEscape && scope != r.Root {
		r.Staged = []Operation{{ID: scope.ID, Props: map[string]any{"hidden": true}}}
		return r.Commit()
	}
	var action string
	if scope == r.Root {
		for pattern, act := range r.keymap {
			if act != "" && keyMatches(pattern, e) {
				action = act
				break
			}
		}
	} else if scopeKm, ok := scope.Props["keymap"].(map[string]any); ok {
		for pattern, actionVal := range scopeKm {
			if s, ok := actionVal.(string); ok && s != "" && keyMatches(pattern, e) {
				action = s
				break
			}
		}
	}
	if action != "" {
		return r.Action(action, map[string]any{"key": e.Name(), "focus": r.Focus})
	}
	n := r.Nodes[r.Focus]
	if n != nil {
		if km, ok := n.Props["keymap"].(map[string]any); ok {
			var act string
			for pattern, actionVal := range km {
				if s, ok := actionVal.(string); ok && s != "" && keyMatches(pattern, e) {
					act = s
					break
				}
			}
			if act != "" {
				return r.Action(act, map[string]any{
					"id":    n.ID,
					"key":   e.Name(),
					"focus": r.Focus,
					"rune":  string(e.Rune()),
					"ctrl":  e.Modifiers()&tcell.ModCtrl != 0,
					"alt":   e.Modifiers()&tcell.ModAlt != 0,
					"shift": e.Modifiers()&tcell.ModShift != 0,
				})
			}
		}
		if onKey := str(n, "on_key", ""); onKey != "" {
			return r.Action(onKey, map[string]any{
				"id":    n.ID,
				"key":   e.Name(),
				"focus": r.Focus,
				"rune":  string(e.Rune()),
				"ctrl":  e.Modifiers()&tcell.ModCtrl != 0,
				"alt":   e.Modifiers()&tcell.ModAlt != 0,
				"shift": e.Modifiers()&tcell.ModShift != 0,
			})
		}
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
	if n == nil {
		return nil
	}
	var dir string
	switch e.Key() {
	case tcell.KeyUp:
		dir = "up"
	case tcell.KeyDown:
		dir = "down"
	case tcell.KeyLeft:
		dir = "left"
	case tcell.KeyRight:
		dir = "right"
	}
	if dir != "" {
		if target, disabled := explicitNavTarget(n, dir); target != "" || disabled {
			if disabled {
				return nil
			}
			if r.focusDirection(dir) {
				return nil
			}
		}
		if n.Type == "button" {
			if r.focusDirection(dir) {
				return nil
			}
			return nil
		}
		if n.Type == "input" && (dir == "up" || dir == "down") {
			if flag(n, "spatial_nav") || r.spatialNav {
				if r.focusDirection(dir) {
					return nil
				}
			}
		}
		if (n.Type == "table" || n.Type == "list" || n.Type == "scroll" || n.Type == "viewport" || n.Type == "tree") && (dir == "left" || dir == "right") {
			if flag(n, "spatial_nav") || r.spatialNav {
				if r.focusDirection(dir) {
					return nil
				}
			}
		}
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
