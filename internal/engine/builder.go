package engine

import "fmt"

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
	children := make([]any, len(t.Children))
	for i, c := range t.Children {
		children[i] = c.toMap()
	}
	return map[string]any{
		"id":       t.ID,
		"text":     t.Text,
		"expanded": t.Expanded,
		"children": children,
	}
}

type FileEntry struct {
	ID          string
	Text        string
	Path        string
	IsDir       bool
	Size        int
	Permissions string
	Description string
}

func (f FileEntry) toMap() map[string]any {
	name := f.Text
	if name == "" {
		name = f.Path
	}
	m := map[string]any{
		"text":   name,
		"path":   f.Path,
		"is_dir": f.IsDir,
	}
	if f.ID != "" {
		m["id"] = f.ID
	}
	if f.Size > 0 {
		m["size"] = f.Size
	}
	if f.Permissions != "" {
		m["permissions"] = f.Permissions
	}
	if f.Description != "" {
		m["description"] = f.Description
	}
	return m
}

type HelpBinding struct {
	Keys        []string
	Description string
}

type HighlightSpan struct {
	Start int
	End   int
}

func NewNode(kind string, children ...*Node) *Node {
	n := &Node{
		Type:  kind,
		Props: map[string]any{},
	}
	return n.Add(children...)
}

func (n *Node) ensureProps() {
	if n.Props == nil {
		n.Props = map[string]any{}
	}
}

func (n *Node) SetID(id string) *Node {
	if n == nil {
		return nil
	}
	n.ID = id
	return n
}

func (n *Node) Prop(key string, val any) *Node {
	if n == nil {
		return nil
	}
	n.ensureProps()
	if val == nil {
		delete(n.Props, key)
	} else {
		n.Props[key] = val
	}
	return n
}

func (n *Node) Add(children ...*Node) *Node {
	if n == nil {
		return nil
	}
	for _, c := range children {
		if c != nil {
			n.Children = append(n.Children, c)
		}
	}
	return n
}

func (n *Node) When(cond bool) *Node {
	if !cond {
		return nil
	}
	return n
}

func (n *Node) Bind(key string) *Node         { return n.Prop("bind", key) }
func (n *Node) Text(text string) *Node        { return n.Prop("text", text) }
func (n *Node) Title(title string) *Node      { return n.Prop("title", title) }
func (n *Node) Placeholder(text string) *Node { return n.Prop("placeholder", text) }
func (n *Node) Width(w int) *Node             { return n.Prop("width", w) }
func (n *Node) Height(h int) *Node            { return n.Prop("height", h) }
func (n *Node) Size(w, h int) *Node           { return n.Prop("width", w).Prop("height", h) }
func (n *Node) MinWidth(w int) *Node          { return n.Prop("min_width", w) }
func (n *Node) MaxWidth(w int) *Node          { return n.Prop("max_width", w) }
func (n *Node) MinHeight(h int) *Node         { return n.Prop("min_height", h) }
func (n *Node) MaxHeight(h int) *Node         { return n.Prop("max_height", h) }
func (n *Node) Grow(g float64) *Node          { return n.Prop("grow", g) }
func (n *Node) Shrink(s float64) *Node        { return n.Prop("shrink", s) }
func (n *Node) Flex(g float64) *Node          { return n.Prop("grow", g) }
func (n *Node) Padding(p int) *Node           { return n.Prop("padding", p) }
func (n *Node) Gap(g int) *Node               { return n.Prop("gap", g) }
func (n *Node) Border(v bool) *Node           { return n.Prop("border", v) }
func (n *Node) Bordered() *Node               { return n.Prop("border", true) }
func (n *Node) FG(color string) *Node         { return n.Prop("fg", color) }
func (n *Node) BG(color string) *Node         { return n.Prop("bg", color) }
func (n *Node) Color(fg, bg string) *Node     { return n.Prop("fg", fg).Prop("bg", bg) }
func (n *Node) Bold(v bool) *Node             { return n.Prop("bold", v) }
func (n *Node) Wrap(v bool) *Node             { return n.Prop("wrap", v) }
func (n *Node) Align(align string) *Node      { return n.Prop("align", align) }
func (n *Node) Justify(justify string) *Node  { return n.Prop("justify", justify) }
func (n *Node) Hidden(v bool) *Node           { return n.Prop("hidden", v) }
func (n *Node) Disabled(v bool) *Node         { return n.Prop("disabled", v) }
func (n *Node) Focusable(v bool) *Node        { return n.Prop("focusable", v) }
func (n *Node) Modal(v bool) *Node            { return n.Prop("modal", v) }
func (n *Node) TrapFocus(v bool) *Node        { return n.Prop("trap_focus", v) }
func (n *Node) ZIndex(z int) *Node            { return n.Prop("z_index", z) }
func (n *Node) Absolute(x, y int) *Node {
	return n.Prop("position", "absolute").Prop("x", x).Prop("y", y)
}
func (n *Node) Variant(variant string) *Node   { return n.Prop("variant", variant) }
func (n *Node) Masked(v bool) *Node            { return n.Prop("masked", v) }
func (n *Node) MaxLength(maxLen int) *Node     { return n.Prop("max_length", maxLen) }
func (n *Node) Filterable(v bool) *Node        { return n.Prop("filterable", v) }
func (n *Node) ShowHeader(v bool) *Node        { return n.Prop("show_header", v) }
func (n *Node) ShowLineNumbers(v bool) *Node   { return n.Prop("show_line_numbers", v) }
func (n *Node) SoftWrap(v bool) *Node          { return n.Prop("soft_wrap", v) }
func (n *Node) PageSize(size int) *Node        { return n.Prop("page_size", size) }
func (n *Node) OnClick(action string) *Node    { return n.Prop("on_click", action) }
func (n *Node) OnChange(action string) *Node   { return n.Prop("on_change", action) }
func (n *Node) OnSubmit(action string) *Node   { return n.Prop("on_submit", action) }
func (n *Node) OnSelect(action string) *Node   { return n.Prop("on_select", action) }
func (n *Node) OnActivate(action string) *Node { return n.Prop("on_activate", action) }
func (n *Node) OnTimeout(action string) *Node  { return n.Prop("on_timeout", action) }

func (n *Node) Key(chord, action string) *Node {
	if n == nil {
		return nil
	}
	n.ensureProps()
	km, ok := n.Props["keymap"].(map[string]any)
	if !ok {
		km = map[string]any{}
		n.Props["keymap"] = km
	}
	km[chord] = action
	return n
}

func (n *Node) Keymap(bindings map[string]string) *Node {
	if n == nil {
		return nil
	}
	n.ensureProps()
	km := make(map[string]any, len(bindings))
	for k, v := range bindings {
		km[k] = v
	}
	n.Props["keymap"] = km
	return n
}

func (n *Node) Items(items ...string) *Node {
	arr := make([]any, len(items))
	for i, s := range items {
		arr[i] = s
	}
	return n.Prop("items", arr)
}

func Column(children ...*Node) *Node  { return NewNode("column", children...) }
func Row(children ...*Node) *Node     { return NewNode("row", children...) }
func Box(children ...*Node) *Node     { return NewNode("box", children...) }
func Overlay(children ...*Node) *Node { return NewNode("overlay", children...) }
func Spacer() *Node                   { return NewNode("box").Grow(1) }

func Panel(title string, children ...*Node) *Node {
	return NewNode("panel", children...).Title(title).Border(true)
}

func Scroll(children ...*Node) *Node {
	return NewNode("scroll", children...)
}

func Modal(id, title string, children ...*Node) *Node {
	return NewNode("panel", children...).SetID(id).Title(title).Border(true).Modal(true).TrapFocus(true)
}

func Text(text string) *Node {
	return NewNode("text").Text(text)
}

func Textf(format string, args ...any) *Node {
	return NewNode("text").Text(fmt.Sprintf(format, args...))
}

func Button(label string, onClick ...string) *Node {
	b := NewNode("button").Text(label)
	if len(onClick) > 0 && onClick[0] != "" {
		b.OnClick(onClick[0])
	}
	return b
}

func Input(bindKey string) *Node {
	n := NewNode("input")
	if bindKey != "" {
		n.Bind(bindKey)
	}
	return n
}

func Textarea(bindKey string) *Node {
	n := NewNode("textarea")
	if bindKey != "" {
		n.Bind(bindKey)
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
			width = max(len(c.Title)+2, 8)
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
		keys[i] = map[string]any{
			"keys":        b.Keys,
			"description": b.Description,
		}
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
