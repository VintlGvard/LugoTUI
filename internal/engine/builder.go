package engine

import (
	"fmt"
	"maps"

	"github.com/gdamore/tcell/v2"
)

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

func (n *Node) SetID(id string) *Node {
	n.ID = id
	return n
}

func (n *Node) Prop(key string, val any) *Node {
	if n.Props == nil {
		n.Props = make(map[string]any)
	}
	n.Props[key] = val
	return n
}

func (n *Node) PropsMap(props map[string]any) *Node {
	if n.Props == nil {
		n.Props = make(map[string]any, len(props))
	}
	maps.Copy(n.Props, props)
	return n
}

func (n *Node) Add(children ...*Node) *Node {
	for _, c := range children {
		if c != nil {
			n.Children = append(n.Children, c)
		}
	}
	return n
}

func (n *Node) When(cond bool, fn func(*Node)) *Node {
	if cond && fn != nil {
		fn(n)
	}
	return n
}

func (n *Node) Bind(key string) *Node              { return n.Prop("bind", key) }
func (n *Node) Source(key string) *Node            { return n.Prop("source", key) }
func (n *Node) Text(text string) *Node             { return n.Prop("text", text) }
func (n *Node) Textf(format string, a ...any) *Node { return n.Prop("text", fmt.Sprintf(format, a...)) }
func (n *Node) Title(title string) *Node           { return n.Prop("title", title) }
func (n *Node) Placeholder(p string) *Node         { return n.Prop("placeholder", p) }
func (n *Node) Width(w int) *Node                  { return n.Prop("width", w) }
func (n *Node) Height(h int) *Node                 { return n.Prop("height", h) }
func (n *Node) Size(w, h int) *Node                { return n.Prop("width", w).Prop("height", h) }
func (n *Node) MinWidth(w int) *Node               { return n.Prop("min_width", w) }
func (n *Node) MinHeight(h int) *Node              { return n.Prop("min_height", h) }
func (n *Node) MaxWidth(w int) *Node               { return n.Prop("max_width", w) }
func (n *Node) MaxHeight(h int) *Node              { return n.Prop("max_height", h) }
func (n *Node) Grow(g float64) *Node               { return n.Prop("grow", g) }
func (n *Node) Shrink(s float64) *Node             { return n.Prop("shrink", s) }
func (n *Node) Flex(g float64) *Node               { return n.Prop("grow", g) }
func (n *Node) Padding(p int) *Node                { return n.Prop("padding", p) }
func (n *Node) Margin(m int) *Node                 { return n.Prop("margin", m) }
func (n *Node) Gap(g int) *Node                    { return n.Prop("gap", g) }
func (n *Node) Border(v ...any) *Node {
	if len(v) == 0 {
		return n.Prop("border", true)
	}
	switch val := v[0].(type) {
	case bool:
		return n.Prop("border", val)
	case string:
		return n.Prop("border", val)
	default:
		return n.Prop("border", true)
	}
}
func (n *Node) BorderStyle(style string) *Node                  { return n.Prop("border_style", style) }
func (n *Node) BorderFG(color string) *Node                     { return n.Prop("border_fg", color) }
func (n *Node) BorderBG(color string) *Node                     { return n.Prop("border_bg", color) }
func (n *Node) BorderColor(fg string, bg ...string) *Node {
	n.Prop("border_fg", fg)
	if len(bg) > 0 && bg[0] != "" {
		n.Prop("border_bg", bg[0])
	}
	return n
}
func (n *Node) BorderGlyphs(bg BorderGlyphs) *Node              { return n.Prop("border", true).Prop("border_glyphs", bg) }
func (n *Node) TitleFG(color string) *Node                      { return n.Prop("title_fg", color) }

func (n *Node) ButtonStyle(style string) *Node                  { return n.Prop("button_style", style) }
func (n *Node) Plain() *Node                                    { return n.ButtonStyle("plain") }
func (n *Node) Filled() *Node                                   { return n.ButtonStyle("filled") }
func (n *Node) Chevrons() *Node                                 { return n.ButtonStyle("chevrons") }
func (n *Node) Parens() *Node                                   { return n.ButtonStyle("parens") }
func (n *Node) Prefix(p string) *Node                           { return n.Prop("prefix", p) }
func (n *Node) Suffix(s string) *Node                           { return n.Prop("suffix", s) }

func (n *Node) CursorPrefix(cp string) *Node                    { return n.Prop("cursor_prefix", cp) }
func (n *Node) SelectedFG(color string) *Node                   { return n.Prop("selected_fg", color) }
func (n *Node) SelectedBG(color string) *Node                   { return n.Prop("selected_bg", color) }
func (n *Node) SelectedBold(v ...bool) *Node {
	b := true
	if len(v) > 0 {
		b = v[0]
	}
	return n.Prop("selected_bold", b)
}

func (n *Node) Focusable(v ...bool) *Node {
	b := true
	if len(v) > 0 {
		b = v[0]
	}
	return n.Prop("focusable", b)
}

func (n *Node) OnPaint(paintFn func(c Canvas)) *Node {
	n.CustomPaint = paintFn
	return n
}

func (n *Node) OnRawKey(keyFn func(e *tcell.EventKey) bool) *Node {
	n.RawKey = keyFn
	return n
}
func (n *Node) FG(color string) *Node              { return n.Prop("fg", color) }
func (n *Node) BG(color string) *Node              { return n.Prop("bg", color) }
func (n *Node) Color(fg, bg string) *Node          { return n.Prop("fg", fg).Prop("bg", bg) }
func (n *Node) Bold(v bool) *Node                  { return n.Prop("bold", v) }
func (n *Node) Align(align string) *Node           { return n.Prop("align", align) }
func (n *Node) Justify(justify string) *Node       { return n.Prop("justify", justify) }
func (n *Node) Hidden(v bool) *Node                { return n.Prop("hidden", v) }
func (n *Node) Disabled(v bool) *Node              { return n.Prop("disabled", v) }
func (n *Node) Modal(v bool) *Node                 { return n.Prop("modal", v) }
func (n *Node) Backdrop(mode string) *Node         { return n.Prop("backdrop", mode) }
func (n *Node) CloseOnEsc(v bool) *Node            { return n.Prop("close_on_esc", v) }
func (n *Node) ZIndex(z int) *Node                 { return n.Prop("z_index", z) }
func (n *Node) Top(v int) *Node                    { return n.Prop("top", v) }
func (n *Node) Right(v int) *Node                  { return n.Prop("right", v) }
func (n *Node) Bottom(v int) *Node                 { return n.Prop("bottom", v) }
func (n *Node) Left(v int) *Node                   { return n.Prop("left", v) }

func (n *Node) Wrap(enabled bool) *Node {
	if enabled {
		return n.Prop("wrap", "word")
	}
	return n.Prop("wrap", "none")
}

func (n *Node) WrapMode(mode string) *Node {
	return n.Prop("wrap", mode)
}

func (n *Node) Absolute(left, top int) *Node {
	return n.Prop("position", "absolute").Prop("left", left).Prop("top", top)
}

func (n *Node) Inset(top, right, bottom, left int) *Node {
	return n.Prop("position", "absolute").Prop("top", top).Prop("right", right).Prop("bottom", bottom).Prop("left", left)
}

func (n *Node) Echo(mode string) *Node {
	return n.Prop("echo", mode)
}

func (n *Node) Masked(v bool) *Node {
	if v {
		return n.Prop("echo", "password")
	}
	return n.Prop("echo", "normal")
}

func (n *Node) CharLimit(limit int) *Node           { return n.Prop("char_limit", limit) }
func (n *Node) MaxLength(limit int) *Node           { return n.Prop("char_limit", limit) }
func (n *Node) ValidationPattern(regex string) *Node { return n.Prop("validation_pattern", regex) }
func (n *Node) Filterable(v bool) *Node             { return n.Prop("filterable", v) }
func (n *Node) MultiSelect(v bool) *Node            { return n.Prop("multi_select", v) }
func (n *Node) ItemHeight(h int) *Node              { return n.Prop("item_height", h) }
func (n *Node) ShowIcons(v bool) *Node              { return n.Prop("show_icons", v) }
func (n *Node) ShowHidden(v bool) *Node             { return n.Prop("show_hidden", v) }
func (n *Node) CurrentDir(dir string) *Node         { return n.Prop("current_dir", dir) }
func (n *Node) NavUp(id string) *Node               { return n.Prop("nav_up", id) }
func (n *Node) NavDown(id string) *Node             { return n.Prop("nav_down", id) }
func (n *Node) NavLeft(id string) *Node             { return n.Prop("nav_left", id) }
func (n *Node) NavRight(id string) *Node            { return n.Prop("nav_right", id) }

func (n *Node) Nav(up, down, left, right string) *Node {
	if up != "" {
		n.Prop("nav_up", up)
	}
	if down != "" {
		n.Prop("nav_down", down)
	}
	if left != "" {
		n.Prop("nav_left", left)
	}
	if right != "" {
		n.Prop("nav_right", right)
	}
	return n
}

func (n *Node) Gradient(startColor, endColor string) *Node {
	return n.Prop("gradient", []any{startColor, endColor})
}

func (n *Node) OnClick(action string) *Node    { return n.Prop("on_click", action) }
func (n *Node) OnChange(action string) *Node   { return n.Prop("on_change", action) }
func (n *Node) OnSubmit(action string) *Node   { return n.Prop("on_submit", action) }
func (n *Node) OnSelect(action string) *Node   { return n.Prop("on_select", action) }
func (n *Node) OnNavigate(action string) *Node { return n.Prop("on_navigate", action) }
func (n *Node) OnTimeout(action string) *Node  { return n.Prop("on_timeout", action) }
func (n *Node) OnKey(action string) *Node      { return n.Prop("on_key", action) }

func (n *Node) setInlineAction(prop string, fn GoActionFunc) *Node {
	if fn == nil {
		return n
	}
	if n.inlineActions == nil {
		n.inlineActions = make(map[string]GoActionFunc)
	}
	n.inlineActions[prop] = fn
	return n
}

func extractPayloadText(payload any) string {
	switch v := payload.(type) {
	case string:
		return v
	case map[string]any:
		if s, ok := v["value"].(string); ok {
			return s
		}
		if s, ok := v["text"].(string); ok {
			return s
		}
		if s, ok := v["path"].(string); ok {
			return s
		}
	}
	return fmt.Sprint(payload)
}

func extractPayloadIndexAndValue(payload any) (int, any) {
	if m, ok := payload.(map[string]any); ok {
		idx := 0
		if f, ok := floatVal(m["index"]); ok {
			idx = int(f)
		}
		if v, exists := m["value"]; exists {
			return idx, v
		}
		if v, exists := m["path"]; exists {
			return idx, v
		}
		return idx, m
	}
	return 0, payload
}

func (n *Node) OnClickFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_click", fn)
}

func (n *Node) OnClickDo(fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_click", func(_ *Runtime, _ map[string]any) error {
		fn()
		return nil
	})
}

func (n *Node) OnClickErr(fn func() error) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_click", func(_ *Runtime, _ map[string]any) error {
		return fn()
	})
}

func (n *Node) OnChangeFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_change", fn)
}

func (n *Node) OnChangeText(fn func(value string)) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_change", func(_ *Runtime, payload map[string]any) error {
		fn(extractPayloadText(payload))
		return nil
	})
}

func (n *Node) OnChangeTextErr(fn func(value string) error) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_change", func(_ *Runtime, payload map[string]any) error {
		return fn(extractPayloadText(payload))
	})
}

func (n *Node) OnSubmitFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_submit", fn)
}

func (n *Node) OnSubmitText(fn func(value string)) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_submit", func(_ *Runtime, payload map[string]any) error {
		fn(extractPayloadText(payload))
		return nil
	})
}

func (n *Node) OnSubmitTextErr(fn func(value string) error) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_submit", func(_ *Runtime, payload map[string]any) error {
		return fn(extractPayloadText(payload))
	})
}

func (n *Node) OnSelectFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_select", fn)
}

func (n *Node) OnSelectItem(fn func(index int, value any)) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_select", func(_ *Runtime, payload map[string]any) error {
		idx, val := extractPayloadIndexAndValue(payload)
		fn(idx, val)
		return nil
	})
}

func (n *Node) OnSelectItemErr(fn func(index int, value any) error) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_select", func(_ *Runtime, payload map[string]any) error {
		idx, val := extractPayloadIndexAndValue(payload)
		return fn(idx, val)
	})
}

func (n *Node) OnNavigateFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_navigate", fn)
}

func (n *Node) OnTimeoutFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_timeout", fn)
}

func (n *Node) OnTimeoutDo(fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("on_timeout", func(_ *Runtime, _ map[string]any) error {
		fn()
		return nil
	})
}

func (n *Node) OnKeyFunc(fn GoActionFunc) *Node {
	return n.setInlineAction("on_key", fn)
}

func (n *Node) Key(key, action string) *Node {
	if n.Props == nil {
		n.Props = make(map[string]any)
	}
	km, ok := n.Props["keymap"].(map[string]any)
	if !ok || km == nil {
		km = make(map[string]any)
		n.Props["keymap"] = km
	}
	km[key] = action
	return n
}

func (n *Node) KeyFunc(key string, fn GoActionFunc) *Node {
	return n.setInlineAction("keymap:"+key, fn)
}

func (n *Node) KeyDo(key string, fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("keymap:"+key, func(_ *Runtime, _ map[string]any) error {
		fn()
		return nil
	})
}

func (n *Node) KeyErr(key string, fn func() error) *Node {
	if fn == nil {
		return n
	}
	return n.setInlineAction("keymap:"+key, func(_ *Runtime, _ map[string]any) error {
		return fn()
	})
}

func (n *Node) Keymap(bindings map[string]string) *Node {
	for k, act := range bindings {
		n.Key(k, act)
	}
	return n
}

func (n *Node) Items(items ...string) *Node {
	arr := make([]any, len(items))
	for i, s := range items {
		arr[i] = s
	}
	return n.Prop("items", arr)
}

func (n *Node) Selected(idx int) *Node          { return n.Prop("selected", idx) }
func (n *Node) Running(v bool) *Node            { return n.Prop("running", v) }
func (n *Node) Percent(p float64) *Node         { return n.Prop("percent", p) }
func (n *Node) ShowPercentage(v bool) *Node     { return n.Prop("show_percentage", v) }
func (n *Node) ShowHeader(v bool) *Node         { return n.Prop("show_header", v) }
func (n *Node) ShowLineNumbers(v bool) *Node    { return n.Prop("show_line_numbers", v) }
func (n *Node) SoftWrap(v bool) *Node           { return n.Prop("soft_wrap", v) }
func (n *Node) Mode(mode string) *Node          { return n.Prop("mode", mode) }
func (n *Node) Preset(preset string) *Node      { return n.Prop("preset", preset) }
func (n *Node) Duration(ms int) *Node           { return n.Prop("duration", ms) }
func (n *Node) Interval(ms int) *Node           { return n.Prop("interval", ms) }
func (n *Node) Page(page, perPage, total int) *Node {
	if perPage < 1 {
		perPage = 1
	}
	return n.Prop("page", page).Prop("per_page", perPage).Prop("total", total)
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
