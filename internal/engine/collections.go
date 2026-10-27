package engine

import (
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type collectionState struct {
	Query    string
	Editing  bool
	X        int
	Expanded map[string]bool
}

func cloneCollectionState(s *collectionState) *collectionState {
	if s == nil {
		return nil
	}
	c := *s
	c.Expanded = map[string]bool{}
	for k, v := range s.Expanded {
		c.Expanded[k] = v
	}
	return &c
}
func isCollection(n *Node) bool {
	return n.Type == "table" || n.Type == "viewport" || n.Type == "tree" || n.Type == "filepicker" || n.Type == "list" && flag(n, "filterable")
}
func (r *Runtime) collectionState(n *Node) (*WidgetState, *collectionState) {
	if r.State == nil {
		r.State = map[string]*WidgetState{}
	}
	s := r.State[n.ID]
	if s == nil {
		s = &WidgetState{Anchor: -1}
		r.State[n.ID] = s
	}
	if s.collection == nil {
		s.collection = &collectionState{Query: str(n, "query", ""), Expanded: map[string]bool{}}
	}
	return s, s.collection
}
func collectionArray(v any) ([]any, bool) {
	if v == nil {
		return nil, true
	}
	a, ok := v.([]any)
	return a, ok
}
func collectionNumber(v any) (int, bool) {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case int:
		f = float64(x)
	default:
		return 0, false
	}
	return int(f), !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 1 && f <= maxText && math.Trunc(f) == f
}
func collectionKeys(m map[string]any, keys ...string) error {
	for k := range m {
		found := false
		for _, allowed := range keys {
			if k == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown collection field %q", k)
		}
	}
	return nil
}
func validateCollection(n *Node) error {
	if !isCollection(n) {
		return nil
	}
	b, err := json.Marshal(n.Props)
	if err != nil {
		return fmt.Errorf("collection data: %w", err)
	}
	if len(b) > maxText {
		return fmt.Errorf("collection data exceeds 1 MiB")
	}
	for _, k := range []string{"soft_wrap", "show_header", "filterable", "show_title", "show_status", "show_help", "show_pagination", "show_hidden", "file_allowed", "dir_allowed", "show_size", "show_permissions", "show_line_numbers"} {
		if v, ok := n.Props[k]; ok {
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%s must be boolean", k)
			}
		}
	}
	if v, ok := n.Props["page_size"]; ok {
		if _, ok := collectionNumber(v); !ok {
			return fmt.Errorf("page_size must be a positive integer")
		}
	}
	if v, ok := n.Props["query"]; ok {
		if _, ok := v.(string); !ok {
			return fmt.Errorf("query must be string")
		}
	}
	if v, ok := n.Props["allowed_extensions"]; ok {
		a, ok := collectionArray(v)
		if !ok {
			return fmt.Errorf("allowed_extensions must be an array")
		}
		for _, v := range a {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("allowed_extensions must contain strings")
			}
		}
	}
	switch n.Type {
	case "table":
		cols, ok := collectionArray(n.Props["columns"])
		if !ok || len(cols) == 0 {
			return fmt.Errorf("table columns must be a nonempty array")
		}
		for _, v := range cols {
			m, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("table column must be an object")
			}
			if err := collectionKeys(m, "title", "width"); err != nil {
				return err
			}
			if _, ok := m["title"].(string); !ok {
				return fmt.Errorf("table column title must be string")
			}
			if _, ok := collectionNumber(m["width"]); !ok {
				return fmt.Errorf("table column width must be a positive integer")
			}
		}
		rows, ok := collectionArray(n.Props["rows"])
		if !ok {
			return fmt.Errorf("table rows must be an array")
		}
		for _, v := range rows {
			row, ok := collectionArray(v)
			if !ok || len(row) != len(cols) {
				return fmt.Errorf("table row must have %d string cells", len(cols))
			}
			for _, cell := range row {
				if _, ok := cell.(string); !ok {
					return fmt.Errorf("table cells must be strings")
				}
			}
		}
	case "viewport":
		if v, ok := n.Props["text"]; ok {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("viewport text must be string")
			}
		}
		a, ok := collectionArray(n.Props["highlights"])
		if !ok {
			return fmt.Errorf("highlights must be an array")
		}
		for _, v := range a {
			m, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("highlight must be an object")
			}
			if err := collectionKeys(m, "start", "end"); err != nil {
				return err
			}
			start, ok := collectionNumber(m["start"])
			end, ok2 := collectionNumber(m["end"])
			if !ok || !ok2 || end < start {
				return fmt.Errorf("highlight start/end must be ordered positive integers")
			}
		}
	case "tree":
		count := 0
		ids := map[string]bool{}
		var walk func(any, int) error
		walk = func(v any, depth int) error {
			if depth > 32 {
				return fmt.Errorf("tree nesting exceeds 32")
			}
			a, ok := collectionArray(v)
			if !ok {
				return fmt.Errorf("tree items must be an array")
			}
			for _, v := range a {
				count++
				if count > 10000 {
					return fmt.Errorf("tree exceeds 10000 items")
				}
				m, ok := v.(map[string]any)
				if !ok {
					return fmt.Errorf("tree item must be an object")
				}
				if err := collectionKeys(m, "id", "text", "children", "expanded"); err != nil {
					return err
				}
				id, ok := m["id"].(string)
				if !ok || id == "" || ids[id] {
					return fmt.Errorf("tree item id must be unique nonempty string")
				}
				ids[id] = true
				if _, ok := m["text"].(string); !ok {
					return fmt.Errorf("tree text must be string")
				}
				if v, ok := m["expanded"]; ok {
					if _, ok := v.(bool); !ok {
						return fmt.Errorf("tree expanded must be boolean")
					}
				}
				if err := walk(m["children"], depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		return walk(n.Props["items"], 0)
	default:
		a, ok := collectionArray(n.Props["items"])
		if !ok {
			return fmt.Errorf("collection items must be an array")
		}
		if len(a) > 10000 {
			return fmt.Errorf("collection exceeds 10000 items")
		}
		for _, v := range a {
			if _, ok := v.(string); ok && n.Type != "filepicker" {
				continue
			}
			m, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("collection item must be text or object")
			}
			if err := collectionKeys(m, "id", "text", "description", "path", "is_dir", "size", "permissions"); err != nil {
				return err
			}
			if _, ok := m["text"].(string); !ok {
				return fmt.Errorf("collection item text must be string")
			}
			for _, k := range []string{"id", "description", "path", "permissions"} {
				if v, ok := m[k]; ok {
					if _, ok := v.(string); !ok {
						return fmt.Errorf("item %s must be string", k)
					}
				}
			}
			if v, ok := m["is_dir"]; ok {
				if _, ok := v.(bool); !ok {
					return fmt.Errorf("item is_dir must be boolean")
				}
			}
			if n.Type == "filepicker" {
				if _, ok := m["path"].(string); !ok {
					return fmt.Errorf("file entry path must be string")
				}
			}
		}
	}
	return nil
}

type collectionItem struct {
	text, description, id, path string
	size, permissions           string
	value                       any
	index, depth                int
	branch, dir                 bool
}

func collectionLabel(v any) (string, string) {
	if m, ok := v.(map[string]any); ok {
		t, _ := m["text"].(string)
		d, _ := m["description"].(string)
		return t, d
	}
	s, _ := v.(string)
	return s, ""
}
func fuzzyScore(text, query string) (int, bool) {
	hay := []rune(strings.ToLower(text))
	needle := []rune(strings.ToLower(query))
	pos, score, last := 0, 0, -2
	for _, q := range needle {
		found := false
		for pos < len(hay) {
			i := pos
			pos++
			if hay[i] != q {
				continue
			}
			score += i
			if i == last+1 {
				score -= 8
			}
			if i == 0 || unicode.IsSpace(hay[i-1]) || hay[i-1] == '/' || hay[i-1] == '_' || hay[i-1] == '-' {
				score -= 5
			}
			last = i
			found = true
			break
		}
		if !found {
			return 0, false
		}
	}
	return score, true
}
func formatFileSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (r *Runtime) collectionItems(n *Node, c *collectionState) []collectionItem {
	values := r.items(n)
	if n.Type == "table" && str(n, "source", "") == "" {
		values, _ = collectionArray(n.Props["rows"])
	}
	out := []collectionItem{}
	if n.Type == "tree" {
		index := 0
		var walk func([]any, int)
		walk = func(a []any, depth int) {
			if depth > 32 || index > 10000 {
				return
			}
			for _, v := range a {
				m, ok := v.(map[string]any)
				if !ok {
					continue
				}
				index++
				text, _ := m["text"].(string)
				id, _ := m["id"].(string)
				kids, _ := collectionArray(m["children"])
				out = append(out, collectionItem{text: text, id: id, value: v, index: index, depth: depth, branch: len(kids) > 0})
				expanded, exists := c.Expanded[id]
				if !exists {
					expanded, _ = m["expanded"].(bool)
				}
				if expanded {
					walk(kids, depth+1)
				}
			}
		}
		walk(values, 0)
		return out
	}
	for i, v := range values {
		text, description := collectionLabel(v)
		it := collectionItem{text: text, description: description, value: v, index: i + 1}
		if m, ok := v.(map[string]any); ok {
			it.id, _ = m["id"].(string)
			it.path, _ = m["path"].(string)
			it.dir, _ = m["is_dir"].(bool)
			it.permissions, _ = m["permissions"].(string)
			if sz, ok := m["size"]; ok {
				switch x := sz.(type) {
				case float64:
					it.size = formatFileSize(int64(x))
				case int:
					it.size = formatFileSize(int64(x))
				case string:
					it.size = x
				}
			}
		}
		if n.Type == "filepicker" && !flag(n, "show_hidden") && strings.HasPrefix(filepath.Base(it.path), ".") {
			continue
		}
		out = append(out, it)
	}
	if c.Query != "" && (n.Type == "list" || n.Type == "filepicker") {
		type ranked struct {
			item  collectionItem
			score int
		}
		matches := []ranked{}
		for _, it := range out {
			if score, ok := fuzzyScore(it.text+" "+it.description, c.Query); ok {
				matches = append(matches, ranked{it, score})
			}
		}
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
		out = nil
		for _, m := range matches {
			out = append(out, m.item)
		}
	}
	return out
}
func collectionOption(n *Node, key string, def bool) bool {
	v, ok := n.Props[key].(bool)
	if !ok {
		return def
	}
	return v
}
func collectionPageSize(n *Node, cr Rect) int {
	v, ok := collectionNumber(n.Props["page_size"])
	if ok {
		return v
	}
	return max(1, cr.H)
}
func collectionBody(n *Node, cr Rect) Rect {
	if n.Type == "table" {
		if collectionOption(n, "show_header", true) {
			cr.Y++
			cr.H = max(0, cr.H-1)
		}
		return cr
	}
	if n.Type == "list" || n.Type == "filepicker" {
		if collectionOption(n, "show_title", true) && str(n, "title", "") != "" {
			cr.Y++
			cr.H = max(0, cr.H-1)
		}
		if flag(n, "filterable") || n.Type == "filepicker" {
			cr.Y++
			cr.H = max(0, cr.H-1)
		}
		for _, key := range []string{"show_status", "show_help", "show_pagination"} {
			if collectionOption(n, key, true) {
				cr.H = max(0, cr.H-1)
			}
		}
	}
	return cr
}
func collectionStride(n *Node, items []collectionItem) int {
	if n.Type == "list" {
		for _, it := range items {
			if it.description != "" {
				return 2
			}
		}
	}
	return 1
}
func collectionClamp(s *WidgetState, count int) { s.Selected = max(0, min(s.Selected, count-1)) }
func collectionReveal(s *WidgetState, count, page int) {
	collectionClamp(s, count)
	if s.Selected < s.Scroll {
		s.Scroll = s.Selected
	}
	if s.Selected >= s.Scroll+page {
		s.Scroll = s.Selected - page + 1
	}
	s.Scroll = max(0, min(s.Scroll, max(0, count-page)))
}
func collectionExpanded(c *collectionState, it collectionItem) bool {
	v, ok := c.Expanded[it.id]
	if !ok {
		m, _ := it.value.(map[string]any)
		v, _ = m["expanded"].(bool)
	}
	return v
}
func collectionFileAllowed(n *Node, it collectionItem) bool {
	if it.dir {
		return collectionOption(n, "dir_allowed", false)
	}
	if !collectionOption(n, "file_allowed", true) {
		return false
	}
	a, _ := collectionArray(n.Props["allowed_extensions"])
	if len(a) == 0 {
		return true
	}
	for _, v := range a {
		ext, _ := v.(string)
		if strings.EqualFold(filepath.Ext(it.path), ext) {
			return true
		}
	}
	return false
}
func (r *Runtime) collectionActivate(n *Node, s *WidgetState, c *collectionState, items []collectionItem) error {
	if len(items) == 0 {
		return nil
	}
	collectionClamp(s, len(items))
	it := items[s.Selected]
	if n.Type == "filepicker" {
		if it.dir {
			return r.Action(str(n, "on_navigate", ""), map[string]any{"id": n.ID, "path": it.path})
		}
		if !collectionFileAllowed(n, it) {
			return nil
		}
	}
	return r.Action(str(n, "on_select", ""), map[string]any{"id": n.ID, "index": it.index, "value": it.value})
}
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
		if header {
			text, _ = m["title"].(string)
		} else if i < len(cells) {
			text, _ = cells[i].(string)
		}
		put(screen, x, y, text, st, intersect(clip, Rect{x, y, width, 1}))
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
		if y >= s.Scroll && y < s.Scroll+cr.H {
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
		if s.Scroll+row == s.Selected {
			style = style.Reverse(true)
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
func (r *Runtime) keyCollection(n *Node, e *tcell.EventKey) (bool, error) {
	if !isCollection(n) {
		return false, nil
	}
	if flag(n, "disabled") {
		return true, nil
	}
	s, c := r.collectionState(n)
	items := r.collectionItems(n, c)
	body := collectionBody(n, contentRect(n))
	page := min(collectionPageSize(n, body), max(1, body.H/collectionStride(n, items)))
	if c.Editing && (n.Type == "list" || n.Type == "filepicker") {
		old := c.Query
		switch e.Key() {
		case tcell.KeyEscape, tcell.KeyEnter:
			c.Editing = false
		case tcell.KeyCtrlU:
			c.Query = ""
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			g := graphemes(c.Query)
			if len(g) > 0 {
				c.Query = strings.Join(g[:len(g)-1], "")
			}
		case tcell.KeyRune:
			if len(c.Query)+len(string(e.Rune())) > maxText {
				return true, fmt.Errorf("query exceeds 1 MiB")
			}
			c.Query += string(e.Rune())
		default:
			return true, nil
		}
		if old != c.Query {
			s.Selected = 0
			s.Scroll = 0
			return true, r.Action(str(n, "on_change", ""), map[string]any{"id": n.ID, "value": c.Query, "query": c.Query})
		}
		return true, nil
	}
	if (n.Type == "list" || n.Type == "filepicker") && e.Key() == tcell.KeyRune && e.Rune() == '/' {
		c.Editing = true
		return true, nil
	}
	if n.Type == "viewport" {
		lines := viewportLines(r.text(n), body.W, flag(n, "soft_wrap"))
		switch e.Key() {
		case tcell.KeyUp:
			s.Scroll--
		case tcell.KeyDown:
			s.Scroll++
		case tcell.KeyPgUp:
			s.Scroll -= page
		case tcell.KeyPgDn:
			s.Scroll += page
		case tcell.KeyHome:
			s.Scroll = 0
			c.X = 0
		case tcell.KeyEnd:
			s.Scroll = len(lines) - body.H
		case tcell.KeyLeft:
			c.X = max(0, c.X-1)
		case tcell.KeyRight:
			if !flag(n, "soft_wrap") {
				c.X++
			}
		case tcell.KeyCtrlD:
			s.Scroll += max(1, page/2)
		case tcell.KeyCtrlU:
			s.Scroll -= max(1, page/2)
		case tcell.KeyRune:
			switch e.Rune() {
			case 'k':
				s.Scroll--
			case 'j':
				s.Scroll++
			case 'h':
				c.X = max(0, c.X-1)
			case 'l':
				if !flag(n, "soft_wrap") {
					c.X++
				}
			case 'g':
				s.Scroll = 0
				c.X = 0
			case 'G':
				s.Scroll = len(lines) - body.H
			case 'd':
				s.Scroll += max(1, page/2)
			case 'u':
				s.Scroll -= max(1, page/2)
			default:
				return false, nil
			}
		default:
			return false, nil
		}
		s.Scroll = max(0, min(s.Scroll, max(0, len(lines)-body.H)))
		return true, nil
	}
	collectionClamp(s, len(items))
	switch e.Key() {
	case tcell.KeyUp:
		s.Selected--
	case tcell.KeyDown:
		s.Selected++
	case tcell.KeyHome:
		s.Selected = 0
	case tcell.KeyEnd:
		s.Selected = len(items) - 1
	case tcell.KeyPgUp:
		s.Selected -= page
	case tcell.KeyPgDn:
		s.Selected += page
	case tcell.KeyCtrlD:
		s.Selected += page
	case tcell.KeyCtrlU:
		s.Selected -= page
	case tcell.KeyRune:
		switch e.Rune() {
		case 'k':
			s.Selected--
		case 'j':
			s.Selected++
		case 'g':
			s.Selected = 0
		case 'G':
			s.Selected = len(items) - 1
		case 'h':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch && collectionExpanded(c, it) {
					c.Expanded[it.id] = false
					return true, nil
				}
			}
			return false, nil
		case 'l':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch && !collectionExpanded(c, it) {
					c.Expanded[it.id] = true
					return true, nil
				}
			}
			return false, nil
		case ' ':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch {
					c.Expanded[it.id] = !collectionExpanded(c, it)
					return true, nil
				}
			}
			return false, nil
		default:
			return false, nil
		}
	case tcell.KeyEnter:
		if n.Type == "filepicker" && e.Modifiers()&tcell.ModCtrl != 0 && len(items) > 0 {
			it := items[s.Selected]
			if collectionFileAllowed(n, it) {
				return true, r.Action(str(n, "on_select", ""), map[string]any{"id": n.ID, "index": it.index, "value": it.value})
			}
			return true, nil
		}
		return true, r.collectionActivate(n, s, c, items)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n.Type == "filepicker" {
			return true, r.Action(str(n, "on_navigate", ""), map[string]any{"id": n.ID, "path": filepath.Dir(str(n, "path", "."))})
		}
	case tcell.KeyLeft, tcell.KeyRight:
		if n.Type == "table" {
			if e.Key() == tcell.KeyRight {
				c.X++
			} else {
				c.X = max(0, c.X-1)
			}
		}
		if n.Type == "tree" && len(items) > 0 {
			it := items[s.Selected]
			expanded := collectionExpanded(c, it)
			if e.Key() == tcell.KeyRight {
				if it.branch {
					if expanded {
						if s.Selected+1 < len(items) {
							s.Selected++
						}
					} else {
						c.Expanded[it.id] = true
					}
				}
			} else {
				if it.branch && expanded {
					c.Expanded[it.id] = false
				} else {
					for i := s.Selected - 1; i >= 0; i-- {
						if items[i].depth < it.depth {
							s.Selected = i
							break
						}
					}
				}
			}
		}
	}
	collectionClamp(s, len(r.collectionItems(n, c)))
	return true, nil
}
func (r *Runtime) mouseCollection(n *Node, e *tcell.EventMouse) (bool, error) {
	if !isCollection(n) {
		return false, nil
	}
	if flag(n, "disabled") {
		return true, nil
	}
	s, c := r.collectionState(n)
	x, y := e.Position()
	body := collectionBody(n, contentRect(n))
	buttons := e.Buttons()
	if buttons&tcell.WheelUp != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyUp, 0, 0))
	}
	if buttons&tcell.WheelDown != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyDown, 0, 0))
	}
	if buttons&tcell.WheelLeft != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	}
	if buttons&tcell.WheelRight != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyRight, 0, 0))
	}
	if buttons&tcell.Button1 == 0 {
		return true, nil
	}
	if x < body.X || x >= body.X+body.W || y < body.Y || y >= body.Y+body.H {
		return true, nil
	}
	if n.Type == "viewport" {
		return true, nil
	}
	items := r.collectionItems(n, c)
	index := s.Scroll + (y-body.Y)/collectionStride(n, items)
	if index >= len(items) {
		return true, nil
	}
	s.Selected = index
	if n.Type == "tree" {
		it := items[index]
		if it.branch && x < body.X+it.depth*2+2 {
			c.Expanded[it.id] = !collectionExpanded(c, it)
			return true, nil
		}
	}
	return true, r.collectionActivate(n, s, c, items)
}
