package core

import (
	"fmt"
	"maps"

	"github.com/gdamore/tcell/v2"
)

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

func (n *Node) Bind(key string) *Node               { return n.Prop("bind", key) }
func (n *Node) Source(key string) *Node             { return n.Prop("source", key) }
func (n *Node) Text(text string) *Node              { return n.Prop("text", text) }
func (n *Node) Textf(format string, a ...any) *Node { return n.Prop("text", fmt.Sprintf(format, a...)) }
func (n *Node) Title(title string) *Node            { return n.Prop("title", title) }
func (n *Node) Placeholder(p string) *Node          { return n.Prop("placeholder", p) }
func (n *Node) Width(w int) *Node                   { return n.Prop("width", w) }
func (n *Node) Height(h int) *Node                  { return n.Prop("height", h) }
func (n *Node) Size(w, h int) *Node                 { return n.Prop("width", w).Prop("height", h) }
func (n *Node) MinWidth(w int) *Node                { return n.Prop("min_width", w) }
func (n *Node) MinHeight(h int) *Node               { return n.Prop("min_height", h) }
func (n *Node) MaxWidth(w int) *Node                { return n.Prop("max_width", w) }
func (n *Node) MaxHeight(h int) *Node               { return n.Prop("max_height", h) }
func (n *Node) Grow(g float64) *Node                { return n.Prop("grow", g) }
func (n *Node) Shrink(s float64) *Node              { return n.Prop("shrink", s) }
func (n *Node) Flex(g float64) *Node                { return n.Prop("grow", g) }
func (n *Node) Padding(p int) *Node                 { return n.Prop("padding", p) }
func (n *Node) Margin(m int) *Node                  { return n.Prop("margin", m) }
func (n *Node) Gap(g int) *Node                     { return n.Prop("gap", g) }

func (n *Node) Border(v ...any) *Node {
	if len(v) == 0 {
		return n.Prop("border", true)
	}
	switch val := v[0].(type) {
	case bool:
		return n.Prop("border", val)
	case string:
		return n.Prop("border", val)
	case BorderSides:
		return n.Prop("border", val)
	default:
		return n.Prop("border", true)
	}
}

func (n *Node) BorderStyle(style string) *Node { return n.Prop("border_style", style) }
func (n *Node) BorderFG(color string) *Node    { return n.Prop("border_fg", color) }
func (n *Node) BorderBG(color string) *Node    { return n.Prop("border_bg", color) }

func (n *Node) BorderColor(fg string, bg ...string) *Node {
	n.Prop("border_fg", fg)
	if len(bg) > 0 && bg[0] != "" {
		n.Prop("border_bg", bg[0])
	}
	return n
}

func (n *Node) BorderGlyphs(bg BorderGlyphs) *Node {
	return n.Prop("border", true).Prop("border_glyphs", bg)
}

func (n *Node) TitleFG(color string) *Node     { return n.Prop("title_fg", color) }
func (n *Node) ButtonStyle(style string) *Node { return n.Prop("button_style", style) }
func (n *Node) Plain() *Node                   { return n.ButtonStyle("plain") }
func (n *Node) Filled() *Node                  { return n.ButtonStyle("filled") }
func (n *Node) Chevrons() *Node                { return n.ButtonStyle("chevrons") }
func (n *Node) Parens() *Node                  { return n.ButtonStyle("parens") }
func (n *Node) Prefix(p string) *Node          { return n.Prop("prefix", p) }
func (n *Node) Suffix(s string) *Node          { return n.Prop("suffix", s) }
func (n *Node) CursorPrefix(cp string) *Node   { return n.Prop("cursor_prefix", cp) }
func (n *Node) SelectedFG(color string) *Node  { return n.Prop("selected_fg", color) }
func (n *Node) SelectedBG(color string) *Node  { return n.Prop("selected_bg", color) }

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

func (n *Node) FG(color string) *Node        { return n.Prop("fg", color) }
func (n *Node) BG(color string) *Node        { return n.Prop("bg", color) }
func (n *Node) Color(fg, bg string) *Node    { return n.Prop("fg", fg).Prop("bg", bg) }
func (n *Node) Bold(v bool) *Node            { return n.Prop("bold", v) }
func (n *Node) Align(align string) *Node     { return n.Prop("align", align) }
func (n *Node) Justify(justify string) *Node { return n.Prop("justify", justify) }
func (n *Node) Hidden(v bool) *Node          { return n.Prop("hidden", v) }
func (n *Node) Disabled(v bool) *Node        { return n.Prop("disabled", v) }
func (n *Node) Modal(v bool) *Node           { return n.Prop("modal", v) }
func (n *Node) Backdrop(mode string) *Node   { return n.Prop("backdrop", mode) }
func (n *Node) CloseOnEsc(v bool) *Node      { return n.Prop("close_on_esc", v) }
func (n *Node) ZIndex(z int) *Node           { return n.Prop("z_index", z) }
func (n *Node) Top(v int) *Node              { return n.Prop("top", v) }
func (n *Node) Right(v int) *Node            { return n.Prop("right", v) }
func (n *Node) Bottom(v int) *Node           { return n.Prop("bottom", v) }
func (n *Node) Left(v int) *Node             { return n.Prop("left", v) }

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

func (n *Node) CharLimit(limit int) *Node            { return n.Prop("char_limit", limit) }
func (n *Node) MaxLength(limit int) *Node            { return n.Prop("char_limit", limit) }
func (n *Node) ValidationPattern(regex string) *Node { return n.Prop("validation_pattern", regex) }
func (n *Node) Filterable(v bool) *Node              { return n.Prop("filterable", v) }
func (n *Node) MultiSelect(v bool) *Node             { return n.Prop("multi_select", v) }
func (n *Node) ItemHeight(h int) *Node               { return n.Prop("item_height", h) }
func (n *Node) ShowIcons(v bool) *Node               { return n.Prop("show_icons", v) }
func (n *Node) ShowHidden(v bool) *Node              { return n.Prop("show_hidden", v) }
func (n *Node) CurrentDir(dir string) *Node          { return n.Prop("current_dir", dir) }
func (n *Node) NavUp(id string) *Node                { return n.Prop("nav_up", id) }
func (n *Node) NavDown(id string) *Node              { return n.Prop("nav_down", id) }
func (n *Node) NavLeft(id string) *Node              { return n.Prop("nav_left", id) }
func (n *Node) NavRight(id string) *Node             { return n.Prop("nav_right", id) }

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

func (n *Node) SetInlineAction(prop string, fn any) *Node {
	if fn == nil {
		return n
	}
	if n.InlineActions == nil {
		n.InlineActions = make(map[string]any)
	}
	n.InlineActions[prop] = fn
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
		if f, ok := FloatVal(m["index"]); ok {
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

func (n *Node) OnClickFunc(fn any) *Node {
	return n.SetInlineAction("on_click", fn)
}

func (n *Node) OnClickDo(fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_click", func() error {
		fn()
		return nil
	})
}

func (n *Node) OnClickErr(fn func() error) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_click", fn)
}

func (n *Node) OnChangeFunc(fn any) *Node {
	return n.SetInlineAction("on_change", fn)
}

func (n *Node) OnChangeText(fn func(value string)) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_change", func(payload map[string]any) error {
		fn(extractPayloadText(payload))
		return nil
	})
}

func (n *Node) OnChangeTextErr(fn func(value string) error) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_change", func(payload map[string]any) error {
		return fn(extractPayloadText(payload))
	})
}

func (n *Node) OnSubmitFunc(fn any) *Node {
	return n.SetInlineAction("on_submit", fn)
}

func (n *Node) OnSubmitText(fn func(value string)) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_submit", func(payload map[string]any) error {
		fn(extractPayloadText(payload))
		return nil
	})
}

func (n *Node) OnSubmitTextErr(fn func(value string) error) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_submit", func(payload map[string]any) error {
		return fn(extractPayloadText(payload))
	})
}

func (n *Node) OnSelectFunc(fn any) *Node {
	return n.SetInlineAction("on_select", fn)
}

func (n *Node) OnSelectItem(fn func(index int, value any)) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_select", func(payload map[string]any) error {
		idx, val := extractPayloadIndexAndValue(payload)
		fn(idx, val)
		return nil
	})
}

func (n *Node) OnSelectItemErr(fn func(index int, value any) error) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_select", func(payload map[string]any) error {
		idx, val := extractPayloadIndexAndValue(payload)
		return fn(idx, val)
	})
}

func (n *Node) OnNavigateFunc(fn any) *Node {
	return n.SetInlineAction("on_navigate", fn)
}

func (n *Node) OnTimeoutFunc(fn any) *Node {
	return n.SetInlineAction("on_timeout", fn)
}

func (n *Node) OnTimeoutDo(fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("on_timeout", func() error {
		fn()
		return nil
	})
}

func (n *Node) OnKeyFunc(fn any) *Node {
	return n.SetInlineAction("on_key", fn)
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

func (n *Node) KeyFunc(key string, fn any) *Node {
	return n.SetInlineAction("keymap:"+key, fn)
}

func (n *Node) KeyDo(key string, fn func()) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("keymap:"+key, func() error {
		fn()
		return nil
	})
}

func (n *Node) KeyErr(key string, fn func() error) *Node {
	if fn == nil {
		return n
	}
	return n.SetInlineAction("keymap:"+key, fn)
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

func (n *Node) Selected(idx int) *Node       { return n.Prop("selected", idx) }
func (n *Node) Running(v bool) *Node         { return n.Prop("running", v) }
func (n *Node) Percent(p float64) *Node      { return n.Prop("percent", p) }
func (n *Node) ShowPercentage(v bool) *Node  { return n.Prop("show_percentage", v) }
func (n *Node) ShowHeader(v bool) *Node      { return n.Prop("show_header", v) }
func (n *Node) ShowLineNumbers(v bool) *Node { return n.Prop("show_line_numbers", v) }
func (n *Node) SoftWrap(v bool) *Node        { return n.Prop("soft_wrap", v) }
func (n *Node) Mode(mode string) *Node       { return n.Prop("mode", mode) }
func (n *Node) Preset(preset string) *Node   { return n.Prop("preset", preset) }
func (n *Node) Duration(ms int) *Node        { return n.Prop("duration", ms) }
func (n *Node) Interval(ms int) *Node        { return n.Prop("interval", ms) }

func (n *Node) Page(page, perPage, total int) *Node {
	if perPage < 1 {
		perPage = 1
	}
	return n.Prop("page", page).Prop("per_page", perPage).Prop("total", total)
}
