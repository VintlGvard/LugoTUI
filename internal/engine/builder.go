package engine

type TableColumn struct {
	Title string
	Width int
	Align string
}

type TreeItem struct {
	ID       string
	Text     string
	Expanded bool
	Children []TreeItem
}

func (t TreeItem) toMap() map[string]any {
	kids := make([]any, len(t.Children))
	for i, c := range t.Children {
		kids[i] = c.toMap()
	}
	return map[string]any{
		"id":       t.ID,
		"text":     t.Text,
		"expanded": t.Expanded,
		"children": kids,
	}
}

type FileEntry struct {
	Text        string
	Name        string
	Path        string
	IsDir       bool
	Size        any
	Permissions string
}

func (f FileEntry) toMap() map[string]any {
	label := f.Text
	if label == "" {
		label = f.Name
	}
	path := f.Path
	if path == "" {
		path = label
	}
	m := map[string]any{
		"text":   label,
		"path":   path,
		"is_dir": f.IsDir,
	}
	if f.Size != nil && f.Size != "" {
		m["size"] = f.Size
	}
	if f.Permissions != "" {
		m["permissions"] = f.Permissions
	}
	return m
}

type HelpBinding struct {
	Keys        []string
	Description string
	Action      string
	Disabled    bool
}

type HighlightSpan struct {
	Start int
	End   int
}

func NewNode(kind string, children ...*Node) *Node {
	n := &Node{
		Type:  kind,
		Props: make(map[string]any),
	}
	return n.Add(children...)
}

func Column(children ...*Node) *Node  { return NewNode("column", children...) }
func Row(children ...*Node) *Node     { return NewNode("row", children...) }
func Box(children ...*Node) *Node     { return NewNode("box", children...) }
func Overlay(children ...*Node) *Node { return NewNode("overlay", children...) }
func Spacer() *Node                   { return NewNode("box").Grow(1) }

func Panel(title string, children ...*Node) *Node {
	return NewNode("panel", children...).Title(title).Border(true).Padding(1)
}

func Scroll(children ...*Node) *Node {
	return NewNode("scroll", children...)
}

func Modal(id, title string, children ...*Node) *Node {
	return NewNode("panel", children...).
		SetID(id).
		Title(title).
		Border(true).
		Modal(true)
}

func Custom(paintFn func(c Canvas)) *Node {
	n := NewNode("custom")
	n.CustomPaint = paintFn
	return n
}

func Text(text string) *Node {
	return NewNode("text").Text(text)
}

func Textf(format string, a ...any) *Node {
	return NewNode("text").Textf(format, a...)
}

func Button(label string, onClick ...string) *Node {
	n := NewNode("button").Text(label)
	if len(onClick) > 0 && onClick[0] != "" {
		n.OnClick(onClick[0])
	}
	return n
}

func Input(bindKey string, placeholder ...string) *Node {
	n := NewNode("input")
	if bindKey != "" {
		n.Bind(bindKey)
	}
	if len(placeholder) > 0 && placeholder[0] != "" {
		n.Placeholder(placeholder[0])
	}
	return n
}

func Textarea(bindKey string, placeholder ...string) *Node {
	n := NewNode("textarea")
	if bindKey != "" {
		n.Bind(bindKey)
	}
	if len(placeholder) > 0 && placeholder[0] != "" {
		n.Placeholder(placeholder[0])
	}
	return n
}

func List(items ...string) *Node {
	return NewNode("list").Items(items...)
}

func Table(columns []TableColumn, rows [][]string) *Node {
	cols := make([]any, len(columns))
	for i, c := range columns {
		width := c.Width
		if width <= 0 {
			width = max(textWidth(c.Title)+2, 8)
		}
		m := map[string]any{
			"title": c.Title,
			"width": width,
		}
		if c.Align != "" {
			m["align"] = c.Align
		}
		cols[i] = m
	}
	rawRows := make([]any, len(rows))
	for i, r := range rows {
		cells := make([]any, len(columns))
		for j := range columns {
			if j < len(r) {
				cells[j] = r[j]
			} else {
				cells[j] = ""
			}
		}
		rawRows[i] = cells
	}
	return NewNode("table").Prop("columns", cols).Prop("rows", rawRows).ShowHeader(true)
}

func Tree(items ...TreeItem) *Node {
	raw := make([]any, len(items))
	for i, item := range items {
		raw[i] = item.toMap()
	}
	return NewNode("tree").Prop("items", raw)
}

func Viewport(text string, highlights ...HighlightSpan) *Node {
	hl := make([]any, len(highlights))
	for i, h := range highlights {
		hl[i] = map[string]any{"start": h.Start, "end": h.End}
	}
	return NewNode("viewport").Prop("text", text).Prop("highlights", hl)
}

func Filepicker(entries ...FileEntry) *Node {
	items := make([]any, len(entries))
	for i, e := range entries {
		items[i] = e.toMap()
	}
	return NewNode("filepicker").Prop("items", items)
}

func Spinner(preset ...string) *Node {
	n := NewNode("spinner").Prop("running", true)
	if len(preset) > 0 && preset[0] != "" {
		n.Prop("preset", preset[0])
	}
	return n
}

func Progress(percent float64) *Node {
	return NewNode("progress").Prop("percent", percent).Prop("show_percentage", true)
}

func Paginator(page, perPage, total int) *Node {
	if perPage < 1 {
		perPage = 1
	}
	return NewNode("paginator").Prop("page", page).Prop("per_page", perPage).Prop("total", total)
}

func Timer(durationMs int, onTimeout ...string) *Node {
	n := NewNode("timer").Prop("duration", durationMs).Prop("running", true)
	if len(onTimeout) > 0 && onTimeout[0] != "" {
		n.OnTimeout(onTimeout[0])
	}
	return n
}

func Stopwatch() *Node {
	return NewNode("stopwatch").Prop("running", true)
}

func Help(bindings ...HelpBinding) *Node {
	keys := make([]any, len(bindings))
	for i, b := range bindings {
		m := map[string]any{
			"keys": b.Keys,
			"help": b.Description,
		}
		if b.Action != "" {
			m["action"] = b.Action
		}
		if b.Disabled {
			m["enabled"] = false
		}
		keys[i] = m
	}
	return NewNode("help").Prop("keys", keys)
}

func Cursor() *Node {
	return NewNode("cursor").Prop("mode", "blink")
}

func If(cond bool, node *Node) *Node {
	if cond {
		return node
	}
	return nil
}

func IfElse(cond bool, thenNode, elseNode *Node) *Node {
	if cond {
		return thenNode
	}
	return elseNode
}

func Map[T any](items []T, fn func(index int, item T) *Node) []*Node {
	out := make([]*Node, 0, len(items))
	for i, item := range items {
		if n := fn(i, item); n != nil {
			out = append(out, n)
		}
	}
	return out
}
