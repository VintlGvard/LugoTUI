package engine

import (
	"bytes"
	"fmt"
	"math"
	"strings"

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
	if isASCII(text) {
		for i := 0; i < len(text); i++ {
			if x >= clip.X && x < clip.X+clip.W {
				s.SetContent(x, y, rune(text[i]), nil, st)
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
		v := g.Runes()
		w := g.Width()
		if w == 0 {
			continue
		}
		if x >= clip.X && x+w <= clip.X+clip.W {
			s.SetContent(x, y, v[0], v[1:], st)
		}
		x += w
		if x >= clip.X+clip.W {
			break
		}
	}
}

func fill(s tcell.Screen, r Rect, st tcell.Style) {
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
	if flag(n, "border") || n.Type == "panel" {
		p++
	}
	return Rect{n.Rect.X + p, n.Rect.Y + p, max(0, n.Rect.W-2*p), max(0, n.Rect.H-2*p)}
}

func (r *Runtime) Draw(s tcell.Screen, diagnostic string) {
	w, h := s.Size()
	var bindText func(*Node)
	bindText = func(n *Node) {
		n.layoutText, n.layoutBound = "", false
		if n.Type == "text" || n.Type == "button" {
			if key := str(n, "bind", ""); key != "" {
				n.layoutText, n.layoutBound = r.Data[key].(string)
			}
		}
		for _, child := range n.Children {
			bindText(child)
		}
	}
	bindText(r.Root)
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
	r.ReconcileFocus()
	s.Clear()
	s.HideCursor()
	var paint func(*Node)
	paint = func(n *Node) {
		if flag(n, "hidden") || n.Clip.W == 0 || n.Clip.H == 0 {
			return
		}
		st := r.Theme.style(n, r.Focus == n.ID)
		fill(s, n.Clip, st)
		rect := n.Rect
		if (flag(n, "border") || n.Type == "panel") && rect.W >= 2 && rect.H >= 2 {
			put(s, rect.X, rect.Y, "┌"+strings.Repeat("─", rect.W-2)+"┐", st, n.Clip)
			put(s, rect.X, rect.Y+rect.H-1, "└"+strings.Repeat("─", rect.W-2)+"┘", st, n.Clip)
			for y := rect.Y + 1; y < rect.Y+rect.H-1; y++ {
				put(s, rect.X, y, "│", st, n.Clip)
				put(s, rect.X+rect.W-1, y, "│", st, n.Clip)
			}
			put(s, rect.X+2, rect.Y, str(n, "title", ""), st, intersect(n.Clip, Rect{rect.X + 1, rect.Y, max(0, rect.W-2), 1}))
		}
		cr := contentRect(n)
		clip := intersect(cr, n.Clip)
		state := r.State[n.ID]
		if r.paintCollection(s, n, st, cr, clip) || r.paintPrimitive(s, n, st, cr, clip) {
			for _, c := range sortedChildren(n) {
				paint(c)
			}
			return
		}
		switch n.Type {
		case "text", "button":
			text := r.text(n)
			if n.Type == "button" {
				text = "[ " + text + " ]"
			}
			paintText(s, n, text, st, cr, clip)
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
				for y := 0; y < cr.H && y+state.Scroll < len(items); y++ {
					index := y + state.Scroll
					ls := st
					if index == state.Selected {
						ls = ls.Reverse(true)
					}
					fill(s, intersect(Rect{cr.X, cr.Y + y, cr.W, 1}, clip), ls)
					put(s, cr.X, cr.Y+y, fmt.Sprint(items[index]), ls, clip)
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
						put(s, outer.X, y, editorLinePrefix(n, state.Value, logical), st, prefixClip)
						previousRow = p[1]
					}
					if i < len(gs) && gs[i] == "\n" {
						line++
					}
				}
				if state.Value == "" {
					paintText(s, n, editorDisplay(n, state), st.Dim(true), cr, clip)
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
					put(s, x, y, g, is, clip)
				}
				if suffix := editorSuggestion(n, state); suffix != "" && r.Focus == n.ID {
					p := positions[len(gs)]
					x, y := cr.X+p[0], cr.Y+p[1]
					if n.Type == "input" {
						x -= state.Scroll
					} else {
						y -= state.Scroll
					}
					put(s, x, y, suffix, st.Dim(true), clip)
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
							}
						}
					}
				}
			}
		}
		for _, c := range sortedChildren(n) {
			paint(c)
		}
	}
	paint(r.Root)
	if diagnostic != "" && h > 0 {
		st := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorMaroon)
		fill(s, Rect{0, h - 1, w, 1}, st)
		put(s, 0, h-1, diagnostic, st, Rect{0, h - 1, w, 1})
		s.HideCursor()
	}
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

func paintText(screen tcell.Screen, n *Node, text string, base tcell.Style, cr, clip Rect) {
	styles := []tcell.Style{}
	ends := []int{}
	total := 0
	if spans, ok := n.Props["spans"].([]any); ok {
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
	for y, line := range LimitedLines(text, cr.W, str(n, "wrap", "") != "none", max(0, clip.Y+clip.H-cr.Y)) {
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
				v := g.Runes()
				screen.SetContent(x, cr.Y+y, v[0], v[1:], st)
			}
			x += w
			offset += len(g.Str())
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
				continue
			}
			cut := bytes.LastIndexAny(buf, " \t")
			if cut > 0 {
				trimmed := bytes.TrimRight(buf[:cut], " \t")
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
		iterator := uniseg.NewGraphemes(line)
		for iterator.Next() {
			if len(out) >= limit {
				return out
			}
			g := iterator.Str()
			gw := iterator.Width()
			if w+gw > width && current != "" {
				if strings.TrimSpace(g) == "" {
					out = append(out, strings.TrimRight(current, " \t"))
					current = ""
					w = 0
					continue
				}
				cut := strings.LastIndexAny(current, " \t")
				if cut > 0 {
					out = append(out, strings.TrimRight(current[:cut], " \t"))
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
		out = append(out, current)
	}
	return out
}
