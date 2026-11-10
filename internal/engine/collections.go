package engine

import (
	"encoding/json"
	"fmt"
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
	f, ok := floatVal(v)
	if !ok {
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
			if err := collectionKeys(m, "title", "width", "align"); err != nil {
				return err
			}
			if _, ok := m["title"].(string); !ok {
				return fmt.Errorf("table column title must be string")
			}
			if _, ok := collectionNumber(m["width"]); !ok {
				return fmt.Errorf("table column width must be a positive integer")
			}
			if a, ok := m["align"]; ok {
				if as, ok := a.(string); !ok || (as != "left" && as != "right" && as != "center") {
					return fmt.Errorf("table column align must be left, right, or center")
				}
			}
		}
		rows, ok := collectionArray(n.Props["rows"])
		if !ok {
			return fmt.Errorf("table rows must be an array")
		}
		for _, v := range rows {
			row, ok := collectionArray(v)
			if !ok || len(row) != len(cols) {
				return fmt.Errorf("table row must have %d cells", len(cols))
			}
			for _, cell := range row {
				if _, ok := cell.(string); ok {
					continue
				}
				cm, ok := cell.(map[string]any)
				if !ok {
					return fmt.Errorf("table cells must be strings or cell objects")
				}
				if err := collectionKeys(cm, "text", "fg", "bg", "align", "bold"); err != nil {
					return err
				}
				if _, ok := cm["text"].(string); !ok {
					return fmt.Errorf("table cell text must be string")
				}
				if a, ok := cm["align"]; ok {
					if as, ok := a.(string); !ok || (as != "left" && as != "right" && as != "center") {
						return fmt.Errorf("table cell align must be left, right, or center")
					}
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
				if f, ok := floatVal(sz); ok {
					it.size = formatFileSize(int64(f))
				} else if s, ok := sz.(string); ok {
					it.size = s
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
