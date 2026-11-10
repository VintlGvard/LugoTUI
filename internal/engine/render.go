package engine

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type Theme struct {
	Foreground      string
	Background      string
	FocusForeground string
}

func DefaultTheme() Theme {
	return Theme{Foreground: "#d8dee9", Background: "#171c26", FocusForeground: "#88c0d0"}
}

func (theme Theme) style(n *Node, focused bool) tcell.Style {
	defaults := DefaultTheme()
	if theme.Foreground == "" {
		theme.Foreground = defaults.Foreground
	}
	if theme.Background == "" {
		theme.Background = defaults.Background
	}
	if theme.FocusForeground == "" {
		theme.FocusForeground = defaults.FocusForeground
	}
	fg := theme.Foreground
	if focused {
		fg = theme.FocusForeground
	}
	return tcell.StyleDefault.Foreground(tcell.GetColor(str(n, "fg", fg))).Background(tcell.GetColor(str(n, "bg", theme.Background))).Bold(flag(n, "bold"))
}

type diffCell struct {
	mainc rune
	comb  string
	style tcell.Style
}

type cellBuffer struct {
	tcell.Screen
	w, h       int
	front      []diffCell
	back       []diffCell
	lastScreen tcell.Screen
	forceAll   bool
}

func (cb *cellBuffer) begin(s tcell.Screen, w, h int) {
	cb.Screen = s
	w, h = max(0, w), max(0, h)
	size := w * h
	needReset := cb.lastScreen != s || cb.w != w || cb.h != h || len(cb.front) != size
	if !needReset && size > 0 {
		mc0, _, st0, _ := s.GetContent(0, 0)
		mcN, _, stN, _ := s.GetContent(w-1, h-1)
		if mc0 != cb.front[0].mainc || st0 != cb.front[0].style || mcN != cb.front[size-1].mainc || stN != cb.front[size-1].style {
			needReset = true
		}
	}
	if needReset {
		cb.lastScreen = s
		cb.w, cb.h = w, h
		if cap(cb.front) < size {
			cb.front = make([]diffCell, size)
		} else {
			cb.front = cb.front[:size]
		}
		if cap(cb.back) < size {
			cb.back = make([]diffCell, size)
		} else {
			cb.back = cb.back[:size]
		}
		cb.forceAll = true
		s.Clear()
	} else {
		cb.forceAll = false
	}
	empty := diffCell{mainc: ' ', style: tcell.StyleDefault}
	for i := range cb.back {
		cb.back[i] = empty
	}
}

func (cb *cellBuffer) putCell(x, y int, mainc rune, comb string, st tcell.Style) {
	if x >= 0 && x < cb.w && y >= 0 && y < cb.h {
		cb.back[y*cb.w+x] = diffCell{mainc: mainc, comb: comb, style: st}
	}
}

func (cb *cellBuffer) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	var comb string
	if len(combc) > 0 {
		comb = string(combc)
	}
	cb.putCell(x, y, mainc, comb, style)
}

func (cb *cellBuffer) flush(s tcell.Screen) {
	if cb.w <= 0 || cb.h <= 0 {
		return
	}
	for i, c := range cb.back {
		if cb.forceAll || c != cb.front[i] {
			cb.front[i] = c
			x, y := i%cb.w, i/cb.w
			var comb []rune
			if c.comb != "" {
				comb = []rune(c.comb)
			}
			s.SetContent(x, y, c.mainc, comb, c.style)
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7f {
			return false
		}
	}
	return true
}

func put(s tcell.Screen, x, y int, text string, st tcell.Style, clip Rect) {
	if y < clip.Y || y >= clip.Y+clip.H {
		return
	}
	cb, isBuf := s.(*cellBuffer)
	if isASCII(text) {
		for i := 0; i < len(text); i++ {
			if x >= clip.X && x < clip.X+clip.W {
				if isBuf {
					cb.putCell(x, y, rune(text[i]), "", st)
				} else {
					s.SetContent(x, y, rune(text[i]), nil, st)
				}
			}
			x++
			if x >= clip.X+clip.W {
				break
			}
		}
		return
	}
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		w := g.Width()
		if w == 0 {
			continue
		}
		if x >= clip.X && x+w <= clip.X+clip.W {
			cluster := g.Str()
			first, size := utf8.DecodeRuneInString(cluster)
			if isBuf {
				comb := ""
				if len(cluster) > size {
					comb = cluster[size:]
				}
				cb.putCell(x, y, first, comb, st)
			} else {
				var comb []rune
				if len(cluster) > size {
					comb = []rune(cluster[size:])
				}
				s.SetContent(x, y, first, comb, st)
			}
		}
		x += w
		if x >= clip.X+clip.W {
			break
		}
	}
}

func putRune(s tcell.Screen, x, y int, r rune, st tcell.Style, clip Rect) {
	if x >= clip.X && x < clip.X+clip.W && y >= clip.Y && y < clip.Y+clip.H {
		if cb, ok := s.(*cellBuffer); ok {
			cb.putCell(x, y, r, "", st)
		} else {
			s.SetContent(x, y, r, nil, st)
		}
	}
}

func fill(s tcell.Screen, r Rect, st tcell.Style) {
	if cb, ok := s.(*cellBuffer); ok {
		x0, y0 := max(0, r.X), max(0, r.Y)
		x1, y1 := min(cb.w, r.X+r.W), min(cb.h, r.Y+r.H)
		cell := diffCell{mainc: ' ', style: st}
		for y := y0; y < y1; y++ {
			row := y * cb.w
			for x := x0; x < x1; x++ {
				cb.back[row+x] = cell
			}
		}
		return
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			s.SetContent(x, y, ' ', nil, st)
		}
	}
}

func (r *Runtime) text(n *Node) string {
	if key := str(n, "bind", ""); key != "" {
		if v, ok := r.Data[key].(string); ok {
			return v
		}
	}
	return str(n, "text", "")
}

func (r *Runtime) Items(n *Node) []any {
	if key := str(n, "source", ""); key != "" {
		if a, ok := r.Data[key].([]any); ok {
			return a
		}
	}
	a, _ := n.Props["items"].([]any)
	return a
}

func (r *Runtime) items(n *Node) []any { return r.Items(n) }

func contentRect(n *Node) Rect {
	p := int(num(n, "padding", 0))
	if hasB, _ := borderGlyphsFor(n); hasB {
		p++
	}
	return Rect{n.Rect.X + p, n.Rect.Y + p, max(0, n.Rect.W-2*p), max(0, n.Rect.H-2*p)}
}

func (r *Runtime) scrollDigest() uint64 {
	var h uint64 = 14695981039346656037
	for _, n := range r.Nodes {
		if n.Type == "scroll" {
			if st := r.State[n.ID]; st != nil {
				h ^= uint64(st.Scroll) + 0x9e3779b97f4a7c15
				h *= 1099511628211
			}
		}
	}
	return h
}

func borderGlyphsFor(n *Node) (bool, BorderGlyphs) {
	if bg, ok := n.Props["border_glyphs"].(BorderGlyphs); ok {
		return true, bg
	}
	bVal := n.Props["border"]
	styleName := str(n, "border_style", "")
	if s, ok := bVal.(string); ok && s != "" && s != "false" {
		styleName = s
	}
	hasBorder := flag(n, "border") || n.Type == "panel"
	if s, ok := bVal.(string); ok && s != "" && s != "none" && s != "false" {
		hasBorder = true
	}
	if !hasBorder || styleName == "none" {
		return false, BorderGlyphs{}
	}
	switch strings.ToLower(styleName) {
	case "rounded":
		return true, BorderRounded
	case "double":
		return true, BorderDouble
	case "thick", "heavy":
		return true, BorderThick
	case "ascii":
		return true, BorderASCII
	default:
		return true, BorderSingle
	}
}

type canvasImpl struct {
	screen  tcell.Screen
	cr      Rect
	clip    Rect
	focused bool
	baseSt  tcell.Style
}

func (c *canvasImpl) Width() int      { return c.cr.W }
func (c *canvasImpl) Height() int     { return c.cr.H }
func (c *canvasImpl) Bounds() Rect    { return Rect{0, 0, c.cr.W, c.cr.H} }
func (c *canvasImpl) IsFocused() bool { return c.focused }

func (c *canvasImpl) SetCell(x, y int, r rune, st ...tcell.Style) {
	if x < 0 || x >= c.cr.W || y < 0 || y >= c.cr.H {
		return
	}
	s := c.baseSt
	if len(st) > 0 {
		s = st[0]
	}
	putRune(c.screen, c.cr.X+x, c.cr.Y+y, r, s, c.clip)
}

func (c *canvasImpl) Print(x, y int, text string, st ...tcell.Style) {
	s := c.baseSt
	if len(st) > 0 {
		s = st[0]
	}
	curX := x
	for _, g := range graphemes(text) {
		gw := textWidth(g)
		if curX >= 0 && curX < c.cr.W && y >= 0 && y < c.cr.H {
			put(c.screen, c.cr.X+curX, c.cr.Y+y, g, s, c.clip)
		}
		curX += gw
	}
}

func (c *canvasImpl) Printf(x, y int, st tcell.Style, format string, args ...any) {
	c.Print(x, y, fmt.Sprintf(format, args...), st)
}

func (c *canvasImpl) Fill(r Rect, ch rune, st ...tcell.Style) {
	s := c.baseSt
	if len(st) > 0 {
		s = st[0]
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			c.SetCell(x, y, ch, s)
		}
	}
}

func (c *canvasImpl) LineH(x, y, w int, ch rune, st ...tcell.Style) {
	s := c.baseSt
	if len(st) > 0 {
		s = st[0]
	}
	for i := 0; i < w; i++ {
		c.SetCell(x+i, y, ch, s)
	}
}

func (c *canvasImpl) LineV(x, y, h int, ch rune, st ...tcell.Style) {
	s := c.baseSt
	if len(st) > 0 {
		s = st[0]
	}
	for i := 0; i < h; i++ {
		c.SetCell(x, y+i, ch, s)
	}
}

func (r *Runtime) Draw(s tcell.Screen, diagnostic string) {
	w, h := s.Size()
	if r.Inline && r.InlineHeight > 0 && r.InlineHeight < h {
		h = r.InlineHeight
	}
	sd := r.scrollDigest()
	textChanged := false
	var bindText func(*Node)
	bindText = func(n *Node) {
		var bound string
		var isBound bool
		if n.Type == "text" || n.Type == "button" {
			if key := str(n, "bind", ""); key != "" {
				bound, isBound = r.Data[key].(string)
			}
		}
		curText := str(n, "text", "")
		if n.layoutText != bound || n.layoutBound != isBound || n.lastPropText != curText {
			n.layoutText, n.layoutBound, n.lastPropText = bound, isBound, curText
			textChanged = true
		}
		for _, child := range n.Children {
			bindText(child)
		}
	}
	bindText(r.Root)
	if textChanged || !r.layoutValid || r.lastLayoutW != w || r.lastLayoutH != h || r.lastLayoutRev != r.Revision || r.lastScrollDigest != sd {
		Layout(r.Root, w, h)
		var scrollLayout func(*Node)
		scrollLayout = func(n *Node) {
			if n.Type == "scroll" && r.State[n.ID] != nil {
				cr := contentRect(n)
				extent := max(cr.H, int(math.Ceil(measuredHeight(n, n.Rect.W)))-(n.Rect.H-cr.H))
				viewport := n.Rect
				viewport.H += extent - cr.H
				place(n, viewport, n.Clip)
				n.Rect.H -= extent - cr.H
				content := contentRect(n)
				viewportClip := intersect(content, n.Clip)
				state := r.State[n.ID]
				state.Scroll = max(0, min(state.Scroll, extent-cr.H))
				var translate func(*Node, Rect)
				translate = func(c *Node, parentClip Rect) {
					c.Rect.Y -= state.Scroll
					c.Clip = intersect(c.Rect, parentClip)
					childClip := intersect(contentRect(c), c.Clip)
					for _, ch := range c.Children {
						translate(ch, childClip)
					}
				}
				for _, c := range n.Children {
					translate(c, viewportClip)
				}
			}
			for _, c := range n.Children {
				scrollLayout(c)
			}
		}
		scrollLayout(r.Root)
		r.layoutValid = true
		r.lastLayoutW = w
		r.lastLayoutH = h
		r.lastLayoutRev = r.Revision
		r.lastScrollDigest = r.scrollDigest()
	}
	r.ReconcileFocus()
	if r.cellBuf == nil {
		r.cellBuf = &cellBuffer{}
	}
	r.cellBuf.begin(s, w, h)
	target := tcell.Screen(r.cellBuf)
	s.HideCursor()
	cursorY := -1
	var paint func(*Node, tcell.Style)
	paint = func(n *Node, bgStyle tcell.Style) {
		if flag(n, "hidden") || n.Clip.W == 0 || n.Clip.H == 0 {
			return
		}
		st := r.Theme.style(n, r.Focus == n.ID)
		forceFill := str(n, "position", "") == "absolute" || num(n, "z_index", 0) != 0 || flag(n, "modal")
		if forceFill || st != bgStyle {
			fill(target, n.Clip, st)
		}
		rect := n.Rect
		if hasB, bg := borderGlyphsFor(n); hasB && rect.W >= 2 && rect.H >= 2 {
			borderSt := st
			if bfg := str(n, "border_fg", str(n, "border_color", "")); bfg != "" {
				borderSt = borderSt.Foreground(tcell.GetColor(bfg))
			}
			if bbg := str(n, "border_bg", ""); bbg != "" {
				borderSt = borderSt.Background(tcell.GetColor(bbg))
			}
			putRune(target, rect.X, rect.Y, bg.TopLeft, borderSt, n.Clip)
			putRune(target, rect.X+rect.W-1, rect.Y, bg.TopRight, borderSt, n.Clip)
			putRune(target, rect.X, rect.Y+rect.H-1, bg.BottomLeft, borderSt, n.Clip)
			putRune(target, rect.X+rect.W-1, rect.Y+rect.H-1, bg.BottomRight, borderSt, n.Clip)
			for x := rect.X + 1; x < rect.X+rect.W-1; x++ {
				putRune(target, x, rect.Y, bg.Top, borderSt, n.Clip)
				putRune(target, x, rect.Y+rect.H-1, bg.Bottom, borderSt, n.Clip)
			}
			for y := rect.Y + 1; y < rect.Y+rect.H-1; y++ {
				putRune(target, rect.X, y, bg.Left, borderSt, n.Clip)
				putRune(target, rect.X+rect.W-1, y, bg.Right, borderSt, n.Clip)
			}
			title := str(n, "title", "")
			if title != "" {
				titleSt := borderSt
				if tfg := str(n, "title_fg", ""); tfg != "" {
					titleSt = titleSt.Foreground(tcell.GetColor(tfg))
				}
				put(target, rect.X+2, rect.Y, title, titleSt, intersect(n.Clip, Rect{rect.X + 1, rect.Y, max(0, rect.W-2), 1}))
			}
		}
		cr := contentRect(n)
		clip := intersect(cr, n.Clip)
		state := r.State[n.ID]
		if n.CustomPaint != nil {
			cv := &canvasImpl{
				screen:  target,
				cr:      cr,
				clip:    clip,
				focused: r.Focus == n.ID,
				baseSt:  st,
			}
			n.CustomPaint(cv)
			for _, c := range sortedChildren(n) {
				paint(c, st)
			}
			return
		}
		if r.paintCollection(target, n, st, cr, clip) || r.paintPrimitive(target, n, st, cr, clip) {
			for _, c := range sortedChildren(n) {
				paint(c, st)
			}
			return
		}
		switch n.Type {
		case "custom":
			// Handled by CustomPaint or container children.
		case "text", "button":
			text := r.text(n)
			if n.Type == "button" {
				bStyle := str(n, "button_style", "")
				prefix := str(n, "prefix", "")
				suffix := str(n, "suffix", "")
				if prefix == "" && suffix == "" {
					switch strings.ToLower(bStyle) {
					case "plain":
						prefix, suffix = "", ""
					case "parens":
						prefix, suffix = "( ", " )"
					case "chevrons":
						prefix, suffix = "< ", " >"
					case "filled":
						prefix, suffix = " ", " "
					default:
						prefix, suffix = "[ ", " ]"
					}
				}
				reqW := textWidth(text) + textWidth(prefix) + textWidth(suffix)
				compact := strings.TrimSpace(prefix) + strings.TrimSpace(suffix)
				if cr.W > 0 {
					if cr.W >= reqW {
						text = prefix + text + suffix
					} else if cr.W >= textWidth(prefix)+textWidth(suffix) {
						avail := cr.W - textWidth(prefix) - textWidth(suffix)
						inner := ""
						curW := 0
						for _, g := range graphemes(text) {
							gw := textWidth(g)
							if curW+gw > avail {
								break
							}
							inner += g
							curW += gw
						}
						text = prefix + inner + suffix
					} else if compact != "" && cr.W >= textWidth(compact) {
						text = compact
					} else if compact != "" {
						text = string([]rune(compact)[:1])
					} else {
						text = ""
					}
				} else {
					text = prefix + text + suffix
				}
			}
			paintText(target, n, text, st, cr, clip)
		case "list":
			if state != nil {
				items := r.items(n)
				state.Selected = max(0, min(state.Selected, len(items)-1))
				if state.Selected < state.Scroll {
					state.Scroll = state.Selected
				}
				if state.Selected >= state.Scroll+cr.H {
					state.Scroll = max(0, state.Selected-cr.H+1)
				}
				cursorPrefix := str(n, "cursor_prefix", "")
				if cursorPrefix == "" && flag(n, "show_cursor") {
					cursorPrefix = "❯ "
				}
				for y := 0; y < cr.H && y+state.Scroll < len(items); y++ {
					index := y + state.Scroll
					ls := st
					if index == state.Selected {
						ls = ls.Reverse(true)
						if sfg := str(n, "selected_fg", ""); sfg != "" {
							ls = ls.Foreground(tcell.GetColor(sfg)).Reverse(false)
						}
						if sbg := str(n, "selected_bg", ""); sbg != "" {
							ls = ls.Background(tcell.GetColor(sbg)).Reverse(false)
						}
						if flag(n, "selected_bold") {
							ls = ls.Bold(true)
						}
					}
					fill(target, intersect(Rect{cr.X, cr.Y + y, cr.W, 1}, clip), ls)
					itemText := fmt.Sprint(items[index])
					if cursorPrefix != "" {
						if index == state.Selected {
							itemText = cursorPrefix + itemText
						} else {
							itemText = strings.Repeat(" ", textWidth(cursorPrefix)) + itemText
						}
					}
					put(target, cr.X, cr.Y+y, itemText, ls, clip)
				}
			}
		case "input", "textarea":
			if state != nil {
				outer := cr
				cr = editorRect(n, state)
				clip = intersect(cr, n.Clip)
				gs, positions := editorIndex(n, state, cr.W)
				state.Cursor = max(0, min(state.Cursor, len(gs)))
				pos := positions[state.Cursor]
				if n.Type == "input" {
					if pos[0] < state.Scroll {
						state.Scroll = pos[0]
					}
					if pos[0] >= state.Scroll+cr.W {
						state.Scroll = max(0, pos[0]-cr.W+1)
					}
				} else {
					if pos[1] < state.Scroll {
						state.Scroll = pos[1]
					}
					if pos[1] >= state.Scroll+cr.H {
						state.Scroll = max(0, pos[1]-cr.H+1)
					}
				}
				prefixClip := intersect(outer, n.Clip)
				line, previousRow := 1, -1
				for i, p := range positions {
					if cr.X == outer.X {
						break
					}
					if p[1] != previousRow {
						logical := 0
						if i == 0 || gs[i-1] == "\n" {
							logical = line
						}
						y := outer.Y + p[1]
						if n.Type == "textarea" {
							y -= state.Scroll
						}
						put(target, outer.X, y, editorLinePrefix(n, state.Value, logical), st, prefixClip)
						previousRow = p[1]
					}
					if i < len(gs) && gs[i] == "\n" {
						line++
					}
				}
				if state.Value == "" {
					paintText(target, n, editorDisplay(n, state), st.Dim(true), cr, clip)
				}
				start, end := 0, len(gs)
				if n.Type == "textarea" {
					start, end = visibleEditorRange(positions, state.Scroll, cr.H)
				}
				for i := start; i < end; i++ {
					g := gs[i]
					if g == "\n" {
						continue
					}
					p := positions[i]
					x, y := cr.X+p[0], cr.Y+p[1]
					if n.Type == "input" {
						x -= state.Scroll
					} else {
						y -= state.Scroll
					}
					is := st
					if state.Anchor >= 0 && i >= min(state.Anchor, state.Cursor) && i < max(state.Anchor, state.Cursor) {
						is = is.Reverse(true)
					}
					put(target, x, y, g, is, clip)
				}
				if suffix := editorSuggestion(n, state); suffix != "" && r.Focus == n.ID {
					p := positions[len(gs)]
					x, y := cr.X+p[0], cr.Y+p[1]
					if n.Type == "input" {
						x -= state.Scroll
					} else {
						y -= state.Scroll
					}
					put(target, x, y, suffix, st.Dim(true), clip)
				}
				if r.Focus == n.ID {
					x, y := cr.X+pos[0], cr.Y+pos[1]
					if n.Type == "input" {
						x -= state.Scroll
					} else {
						y -= state.Scroll
					}
					if x >= clip.X && x < clip.X+clip.W && y >= clip.Y && y < clip.Y+clip.H {
						mode := str(n, "cursor_mode", "blink")
						if mode != "hidden" && mode != "hide" {
							p := r.primitive(n)
							if mode != "blink" || !p.cursorBlinked {
								s.ShowCursor(x, y)
								cursorY = y
							}
						}
					}
				}
			}
		}
		for _, c := range sortedChildren(n) {
			paint(c, st)
		}
	}
	paint(r.Root, tcell.StyleDefault)
	if diagnostic != "" && h > 0 {
		st := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorMaroon)
		fill(target, Rect{0, h - 1, w, 1}, st)
		put(target, 0, h-1, diagnostic, st, Rect{0, h - 1, w, 1})
		if cursorY == h-1 {
			s.HideCursor()
		}
	}
	r.cellBuf.flush(s)
	s.Show()
}

func plainText(n *Node) string {
	if spans, ok := n.Props["spans"].([]any); ok {
		var b strings.Builder
		for _, v := range spans {
			m, _ := v.(map[string]any)
			s, _ := m["text"].(string)
			b.WriteString(s)
		}
		return b.String()
	}
	return str(n, "text", str(n, "title", ""))
}

func (n *Node) cachedLines(text string, width int, wrap bool, limit int) []string {
	if n != nil && n.linesCacheValid && n.linesCacheText == text && n.linesCacheWidth == width && n.linesCacheWrap == wrap && n.linesCacheLimit == limit {
		return n.linesCache
	}
	lines := LimitedLines(text, width, wrap, limit)
	if n != nil {
		n.linesCacheText = text
		n.linesCacheWidth = width
		n.linesCacheWrap = wrap
		n.linesCacheLimit = limit
		n.linesCache = lines
		n.linesCacheValid = true
	}
	return lines
}

func paintText(screen tcell.Screen, n *Node, text string, base tcell.Style, cr, clip Rect) {
	var styles []tcell.Style
	var ends []int
	total := 0
	if spans, ok := n.Props["spans"].([]any); ok {
		styles = make([]tcell.Style, 0, len(spans))
		ends = make([]int, 0, len(spans))
		for _, v := range spans {
			m := v.(map[string]any)
			st := base
			if fg, ok := m["fg"].(string); ok {
				st = st.Foreground(tcell.GetColor(fg))
			}
			if bg, ok := m["bg"].(string); ok {
				st = st.Background(tcell.GetColor(bg))
			}
			if b, ok := m["bold"].(bool); ok {
				st = st.Bold(b)
			}
			s, _ := m["text"].(string)
			total += len(s)
			ends = append(ends, total)
			styles = append(styles, st)
		}
		text = plainText(n)
	}
	offset := 0
	spanIndex := 0
	for y, line := range n.cachedLines(text, cr.W, str(n, "wrap", "") != "none", max(0, clip.Y+clip.H-cr.Y)) {
		if y >= cr.H {
			break
		}
		if len(styles) == 0 {
			put(screen, cr.X, cr.Y+y, line, base, clip)
			continue
		}
		start := strings.Index(text[offset:], line)
		if start < 0 {
			start = 0
		}
		offset += start
		x := cr.X
		g := uniseg.NewGraphemes(line)
		for g.Next() {
			st := base
			for spanIndex < len(ends) && offset >= ends[spanIndex] {
				spanIndex++
			}
			if spanIndex < len(styles) {
				st = styles[spanIndex]
			}
			w := g.Width()
			if w > 0 && x >= clip.X && x+w <= clip.X+clip.W && cr.Y+y >= clip.Y && cr.Y+y < clip.Y+clip.H {
				cluster := g.Str()
				first, size := utf8.DecodeRuneInString(cluster)
				var comb []rune
				if len(cluster) > size {
					comb = []rune(cluster[size:])
				}
				screen.SetContent(x, cr.Y+y, first, comb, st)
			}
			x += w
			offset += len(g.Str())
		}
		if offset < len(text) && text[offset] == '\n' {
			offset++
		}
	}
}

func TextWidth(s string) int { return uniseg.StringWidth(s) }
func textWidth(s string) int { return TextWidth(s) }

func graphemes(s string) []string {
	g := uniseg.NewGraphemes(s)
	out := []string{}
	for g.Next() {
		out = append(out, g.Str())
	}
	return out
}

func limitedLinesASCII(line string, width int, limit int, out []string) []string {
	if len(line) == 0 {
		return append(out, "")
	}
	buf := make([]byte, 0, width+16)
	for i := 0; i < len(line); i++ {
		if len(out) >= limit {
			return out
		}
		b := line[i]
		if len(buf)+1 > width && len(buf) > 0 {
			if b == ' ' || b == '\t' {
				trimmed := bytes.TrimRight(buf, " \t")
				out = append(out, string(trimmed))
				buf = buf[:0]
				for i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
					i++
				}
				if i+1 >= len(line) {
					return out
				}
				continue
			}
			cut := bytes.LastIndexAny(buf, " \t")
			trimmed := bytes.TrimRight(buf[:max(0, cut)], " \t")
			if cut > 0 && len(trimmed) > 0 {
				out = append(out, string(trimmed))
				rem := bytes.TrimLeft(buf[cut+1:], " \t")
				buf = append(buf[:0], rem...)
				if len(buf)+1 > width && len(buf) > 0 {
					if len(out) >= limit {
						return out
					}
					out = append(out, string(buf))
					buf = buf[:0]
				}
			} else {
				out = append(out, string(buf))
				buf = buf[:0]
			}
			if (b == ' ' || b == '\t') && len(buf) == 0 {
				continue
			}
		}
		buf = append(buf, b)
	}
	if len(out) >= limit {
		return out
	}
	return append(out, string(buf))
}

func Lines(text string, width int, wrap bool) []string {
	return LimitedLines(text, width, wrap, int(^uint(0)>>1))
}
func LimitedLines(text string, width int, wrap bool, limit int) []string {
	out := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		if len(out) >= limit {
			return out
		}
		if !wrap || width <= 0 {
			out = append(out, line)
			continue
		}
		if isASCII(line) {
			out = limitedLinesASCII(line, width, limit, out)
			continue
		}
		current := ""
		w := 0
		wrappedOnSpace := false
		iterator := uniseg.NewGraphemes(line)
		for iterator.Next() {
			if len(out) >= limit {
				return out
			}
			g := iterator.Str()
			gw := iterator.Width()
			if wrappedOnSpace {
				if strings.TrimSpace(g) == "" {
					continue
				}
				wrappedOnSpace = false
			}
			if w+gw > width && current != "" {
				if strings.TrimSpace(g) == "" {
					out = append(out, strings.TrimRight(current, " \t"))
					current = ""
					w = 0
					wrappedOnSpace = true
					continue
				}
				cut := strings.LastIndexAny(current, " \t")
				trimmed := strings.TrimRight(current[:max(0, cut)], " \t")
				if cut > 0 && len(trimmed) > 0 {
					out = append(out, trimmed)
					current = strings.TrimLeft(current[cut+1:], " \t")
					w = textWidth(current)
					if w+gw > width && current != "" {
						if len(out) >= limit {
							return out
						}
						out = append(out, current)
						current = ""
						w = 0
					}
				} else {
					out = append(out, current)
					current = ""
					w = 0
				}
				if strings.TrimSpace(g) == "" && current == "" {
					continue
				}
			}
			current += g
			w += gw
		}
		if len(out) >= limit {
			return out
		}
		if !wrappedOnSpace || current != "" || len(line) == 0 {
			out = append(out, current)
		}
	}
	return out
}
