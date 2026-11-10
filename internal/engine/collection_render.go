package engine

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func viewportLines(text string, width int, wrap bool) [][]string {
	lines := [][]string{{}}
	x := 0
	for _, g := range graphemes(text) {
		if g == "\n" {
			lines = append(lines, []string{})
			x = 0
			continue
		}
		w := textWidth(g)
		if wrap && x > 0 && x+w > max(1, width) {
			lines = append(lines, []string{})
			x = 0
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], g)
		x += w
	}
	return lines
}

func (r *Runtime) paintTableRow(screen tcell.Screen, n *Node, value any, header bool, x, y int, st tcell.Style, clip Rect, offset int) {
	columns, _ := collectionArray(n.Props["columns"])
	cells, _ := collectionArray(value)
	x -= offset
	for i, v := range columns {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		width, ok := collectionNumber(m["width"])
		if !ok {
			continue
		}
		text := ""
		cellStyle := st
		align, _ := m["align"].(string)
		if align == "" {
			align = "left"
		}
		if header {
			text, _ = m["title"].(string)
		} else if i < len(cells) {
			if s, ok := cells[i].(string); ok {
				text = s
			} else if cm, ok := cells[i].(map[string]any); ok {
				text, _ = cm["text"].(string)
				if a, ok := cm["align"].(string); ok && a != "" {
					align = a
				}
				if fg, ok := cm["fg"].(string); ok && fg != "" {
					cellStyle = cellStyle.Foreground(tcell.GetColor(fg))
				}
				if bg, ok := cm["bg"].(string); ok && bg != "" {
					cellStyle = cellStyle.Background(tcell.GetColor(bg))
				}
				if bold, ok := cm["bold"].(bool); ok && bold {
					cellStyle = cellStyle.Bold(true)
				}
			}
		}
		drawX := x
		if align != "left" {
			if tw := textWidth(text); tw < width {
				if align == "right" {
					drawX = x + (width - tw)
				} else if align == "center" {
					drawX = x + (width-tw)/2
				}
			}
		}
		put(screen, drawX, y, text, cellStyle, intersect(clip, Rect{x, y, width, 1}))
		x += width + 1
	}
}

func (r *Runtime) paintViewport(screen tcell.Screen, n *Node, s *WidgetState, c *collectionState, st tcell.Style, cr, clip Rect) {
	text := r.text(n)
	contentW := cr.W
	gutterW := 0
	linesTotal := strings.Count(text, "\n") + 1
	if flag(n, "show_line_numbers") {
		gutterW = len(fmt.Sprintf("%d", linesTotal)) + 2
		contentW = max(1, cr.W-gutterW)
	}
	lines := viewportLines(text, contentW, flag(n, "soft_wrap"))
	s.Scroll = max(0, min(s.Scroll, max(0, len(lines)-cr.H)))
	maxWidth := 0
	for _, line := range lines {
		maxWidth = max(maxWidth, textWidth(strings.Join(line, "")))
	}
	if flag(n, "soft_wrap") {
		c.X = 0
	} else {
		c.X = max(0, min(c.X, max(0, maxWidth-contentW)))
	}
	if gutterW > 0 {
		for row := 0; row < cr.H && s.Scroll+row < len(lines); row++ {
			numStr := fmt.Sprintf("%*d ", gutterW-1, s.Scroll+row+1)
			put(screen, cr.X, cr.Y+row, numStr, st.Dim(true), clip)
		}
	}
	highlights, _ := collectionArray(n.Props["highlights"])
	gs := graphemes(text)
	x, y := 0, 0
	for i, g := range gs {
		if g == "\n" {
			x = 0
			y++
			continue
		}
		w := textWidth(g)
		if flag(n, "soft_wrap") && x > 0 && x+w > max(1, contentW) {
			x = 0
			y++
		}
		if y >= s.Scroll+cr.H {
			break
		}
		if y >= s.Scroll {
			style := st
			for _, v := range highlights {
				m, ok := v.(map[string]any)
				if !ok {
					continue
				}
				a, ok := collectionNumber(m["start"])
				b, ok2 := collectionNumber(m["end"])
				if ok && ok2 && i+1 >= a && i+1 <= b {
					style = style.Reverse(true)
					break
				}
			}
			put(screen, cr.X+gutterW+x-c.X, cr.Y+y-s.Scroll, g, style, clip)
		}
		x += w
	}
}

func (r *Runtime) paintCollection(screen tcell.Screen, n *Node, st tcell.Style, cr, clip Rect) bool {
	if !isCollection(n) {
		return false
	}
	s, c := r.collectionState(n)
	if n.Type == "viewport" {
		r.paintViewport(screen, n, s, c, st, cr, clip)
		return true
	}
	items := r.collectionItems(n, c)
	body := collectionBody(n, cr)
	stride := collectionStride(n, items)
	page := min(collectionPageSize(n, body), max(1, body.H/stride))
	collectionReveal(s, len(items), page)
	if n.Type == "table" && collectionOption(n, "show_header", true) {
		r.paintTableRow(screen, n, nil, true, cr.X, cr.Y, st.Bold(true), clip, c.X)
	}
	if n.Type == "list" || n.Type == "filepicker" {
		y := cr.Y
		if collectionOption(n, "show_title", true) && str(n, "title", "") != "" {
			put(screen, cr.X, y, str(n, "title", ""), st.Bold(true), clip)
			y++
		}
		if flag(n, "filterable") || n.Type == "filepicker" {
			prefix := "/ "
			if c.Editing {
				prefix = "Filter: "
			}
			put(screen, cr.X, y, prefix+c.Query, st, clip)
		}
		y = body.Y + body.H
		if collectionOption(n, "show_status", true) {
			put(screen, cr.X, y, fmt.Sprintf("%d items", len(items)), st, clip)
			y++
		}
		if collectionOption(n, "show_help", true) {
			put(screen, cr.X, y, "↑/↓ select • enter open • / filter", st, clip)
			y++
		}
		if collectionOption(n, "show_pagination", true) {
			put(screen, cr.X, y, fmt.Sprintf("%d / %d", s.Selected/page+1, max(1, (len(items)+page-1)/page)), st, clip)
		}
	}
	for row := 0; row < page && row*stride < body.H && s.Scroll+row < len(items); row++ {
		it := items[s.Scroll+row]
		y := body.Y + row*stride
		style := st
		cursorPrefix := str(n, "cursor_prefix", "")
		if cursorPrefix == "" && flag(n, "show_cursor") {
			cursorPrefix = "❯ "
		}
		if s.Scroll+row == s.Selected {
			style = style.Reverse(true)
			if sfg := str(n, "selected_fg", ""); sfg != "" {
				style = style.Foreground(tcell.GetColor(sfg)).Reverse(false)
			}
			if sbg := str(n, "selected_bg", ""); sbg != "" {
				style = style.Background(tcell.GetColor(sbg)).Reverse(false)
			}
			if flag(n, "selected_bold") {
				style = style.Bold(true)
			}
		}
		if n.Type == "filepicker" && !collectionFileAllowed(n, it) {
			style = style.Dim(true)
		}
		fill(screen, intersect(Rect{body.X, y, body.W, min(stride, body.Y+body.H-y)}, clip), style)
		if n.Type == "table" {
			r.paintTableRow(screen, n, it.value, false, body.X, y, style, intersect(body, clip), c.X)
			continue
		}
		label := it.text
		if cursorPrefix != "" {
			if s.Scroll+row == s.Selected {
				label = cursorPrefix + label
			} else {
				label = strings.Repeat(" ", textWidth(cursorPrefix)) + label
			}
		}
		if n.Type == "tree" {
			prefix := "  "
			if it.branch {
				if collectionExpanded(c, it) {
					prefix = "▾ "
				} else {
					prefix = "▸ "
				}
			}
			label = strings.Repeat("  ", it.depth) + prefix + label
		}
		if n.Type == "filepicker" {
			if it.dir {
				label += "/"
			}
			details := []string{}
			if flag(n, "show_permissions") && it.permissions != "" {
				details = append(details, it.permissions)
			}
			if flag(n, "show_size") && it.size != "" {
				details = append(details, it.size)
			}
			if len(details) > 0 {
				detailStr := strings.Join(details, " ")
				detailW := textWidth(detailStr)
				detailX := max(body.X+textWidth(label)+1, body.X+body.W-detailW)
				put(screen, detailX, y, detailStr, style.Dim(true), intersect(body, clip))
			}
		}
		put(screen, body.X, y, label, style, intersect(body, clip))
		if stride == 2 && y+1 < body.Y+body.H {
			put(screen, body.X+2, y+1, it.description, style.Dim(true), intersect(body, clip))
		}
	}
	return true
}
