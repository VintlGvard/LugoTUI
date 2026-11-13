package core

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
)

func FindNode(root *Node, id string) *Node {
	if root == nil || id == "" {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, c := range root.Children {
		if found := FindNode(c, id); found != nil {
			return found
		}
	}
	return nil
}

func CloneNode(n *Node) *Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Props = maps.Clone(n.Props)
	if c.Props == nil {
		c.Props = make(map[string]any)
	}
	if km, ok := n.Props["keymap"].(map[string]any); ok {
		c.Props["keymap"] = maps.Clone(km)
	}
	c.InlineActions = maps.Clone(n.InlineActions)
	c.Children = make([]*Node, len(n.Children))
	for i, ch := range n.Children {
		c.Children[i] = CloneNode(ch)
	}
	return &c
}

func SortedChildren(n *Node) []*Node {
	if n == nil {
		return nil
	}
	compare := func(a, b *Node) int {
		return cmp.Compare(Num(a, "z_index", 0), Num(b, "z_index", 0))
	}
	if slices.IsSortedFunc(n.Children, compare) {
		return n.Children
	}
	children := slices.Clone(n.Children)
	slices.SortStableFunc(children, compare)
	return children
}

func Ordered(n *Node) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		if n == nil {
			return
		}
		var walk func(*Node) bool
		walk = func(n *Node) bool {
			if n == nil {
				return true
			}
			if !yield(n) {
				return false
			}
			for _, child := range n.Children {
				if !walk(child) {
					return false
				}
			}
			return true
		}
		walk(n)
	}
}

func VisibleNodes(n *Node) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		if n == nil {
			return
		}
		var walk func(*Node) bool
		walk = func(n *Node) bool {
			if n == nil || Flag(n, "hidden") {
				return true
			}
			if !yield(n) {
				return false
			}
			for _, child := range SortedChildren(n) {
				if !walk(child) {
					return false
				}
			}
			return true
		}
		walk(n)
	}
}

func AutoAssignIDs(root *Node) {
	if root == nil {
		return
	}
	used := map[string]bool{}
	seenCollect := map[*Node]bool{}
	var collect func(*Node, int)
	collect = func(n *Node, depth int) {
		if n == nil || depth > MaxDepth || seenCollect[n] {
			return
		}
		seenCollect[n] = true
		if n.ID != "" {
			used[n.ID] = true
		}
		for _, c := range n.Children {
			collect(c, depth+1)
		}
	}
	collect(root, 0)

	seenAssign := map[*Node]bool{}
	var assign func(*Node, string, int, int)
	assign = func(n *Node, parentID string, idx int, depth int) {
		if n == nil || depth > MaxDepth || seenAssign[n] {
			return
		}
		seenAssign[n] = true
		if n.Props == nil {
			n.Props = map[string]any{}
		}
		if n.ID == "" {
			base := "root"
			if parentID != "" {
				suffix := fmt.Sprintf("%s_%d", n.Type, idx)
				if b, ok := n.Props["bind"].(string); ok && b != "" {
					suffix = n.Type + "_" + b
				} else if act, ok := n.Props["on_click"].(string); ok && act != "" {
					suffix = n.Type + "_" + act
				}
				base = parentID + "." + suffix
			}
			candidate := base
			for seq := 2; used[candidate]; seq++ {
				candidate = fmt.Sprintf("%s_%d", base, seq)
			}
			n.ID = candidate
			used[candidate] = true
		}
		for i, c := range n.Children {
			assign(c, n.ID, i, depth+1)
		}
	}
	assign(root, "", 0, 0)
}

func validateSidesProp(v any, key string) error {
	checkNum := func(x any) error {
		if f, ok := FloatVal(x); !ok || !Finite(f) || f < 0 || f > 1e6 {
			return fmt.Errorf("invalid %s", key)
		}
		return nil
	}
	switch x := v.(type) {
	case Sides:
		if x.Top < 0 || x.Right < 0 || x.Bottom < 0 || x.Left < 0 || x.Top > 1e6 || x.Right > 1e6 || x.Bottom > 1e6 || x.Left > 1e6 {
			return fmt.Errorf("invalid %s", key)
		}
		return nil
	case map[string]any:
		for k, val := range x {
			switch k {
			case "top", "right", "bottom", "left", "x", "y", "horizontal", "vertical":
				if err := checkNum(val); err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid %s key %q", key, k)
			}
		}
		return nil
	case []int:
		if len(x) == 0 || len(x) > 4 {
			return fmt.Errorf("invalid %s", key)
		}
		for _, val := range x {
			if err := checkNum(val); err != nil {
				return err
			}
		}
		return nil
	case []any:
		if len(x) == 0 || len(x) > 4 {
			return fmt.Errorf("invalid %s", key)
		}
		for _, val := range x {
			if err := checkNum(val); err != nil {
				return err
			}
		}
		return nil
	default:
		return checkNum(v)
	}
}

func validateBorderMap(m map[string]any) error {
	for dir, val := range m {
		switch dir {
		case "top", "right", "bottom", "left":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("border.%s must be boolean", dir)
			}
		default:
			return fmt.Errorf("invalid border side %q", dir)
		}
	}
	return nil
}

func ValidateTree(root *Node, validators ...func(*Node) error) (map[string]*Node, error) {
	ids := map[string]*Node{}
	count, total := 0, 0
	var walk func(*Node, int) error
	walk = func(n *Node, d int) (err error) {
		property := ""
		defer func() {
			var contextual *NodeError
			if err != nil && !errors.As(err, &contextual) {
				err = &NodeError{Node: n, Property: property, Err: err}
			}
		}()
		if n == nil {
			return errors.New("nil node")
		}
		count++
		if count > MaxNodes || d > MaxDepth {
			return errors.New("tree exceeds limits")
		}
		switch n.Type {
		case "box", "row", "column", "panel", "text", "input", "textarea", "button", "list", "scroll", "overlay", "table", "viewport", "tree", "filepicker", "spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor", "custom":
		default:
			return fmt.Errorf("unknown component %q", n.Type)
		}
		if n.ID != "" {
			if len(n.ID) > MaxIDBytes {
				return errors.New("id exceeds 128 bytes")
			}
			if ids[n.ID] != nil {
				return fmt.Errorf("duplicate id %q", n.ID)
			}
			ids[n.ID] = n
		}
		if (Focusable(n) || Flag(n, "modal") || n.Type == "scroll" || n.Type == "spinner" || n.Type == "progress" || n.Type == "cursor") && n.ID == "" {
			return fmt.Errorf("%s requires id", n.Type)
		}
		for k, v := range n.Props {
			property = k
			switch k {
			case "width", "height", "basis", "min_width", "max_width", "min_height", "max_height", "top", "right", "bottom", "left":
				if s, ok := v.(string); ok {
					if s != "auto" {
						if !strings.HasSuffix(s, "%") {
							return fmt.Errorf("%s: invalid %s", n.ID, k)
						}
						f, e := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
						if e != nil || !Finite(f) || f < 0 || f > 10000 {
							return fmt.Errorf("invalid percentage %s", s)
						}
					}
				} else if f, ok := FloatVal(v); !ok || !Finite(f) || f < 0 || f > 1e6 {
					return fmt.Errorf("invalid %s", k)
				}
			case "grow", "shrink", "gap",
				"padding_top", "padding_right", "padding_bottom", "padding_left", "padding_x", "padding_y", "padding_horizontal", "padding_vertical",
				"margin_top", "margin_right", "margin_bottom", "margin_left", "margin_x", "margin_y", "margin_horizontal", "margin_vertical":
				if f, ok := FloatVal(v); !ok || !Finite(f) || f < 0 || f > 1e6 {
					return fmt.Errorf("invalid %s", k)
				}
			case "padding", "margin":
				if err := validateSidesProp(v, k); err != nil {
					return err
				}
			case "direction":
				if v != "row" && v != "column" {
					return errors.New("invalid direction")
				}
			case "align", "align_self":
				if v != "start" && v != "center" && v != "end" && v != "stretch" {
					return errors.New("invalid alignment")
				}
			case "justify":
				if v != "start" && v != "center" && v != "end" && v != "space-between" && v != "space-around" {
					return errors.New("invalid justify")
				}
			case "position":
				if v != "absolute" && v != "relative" {
					return errors.New("invalid position")
				}
			case "text", "title", "bind", "source", "on_submit", "on_change", "on_select", "on_click", "on_key", "fg", "bg", "wrap", "nav_up", "nav_down", "nav_left", "nav_right", "border_style", "border_fg", "border_bg", "border_color", "title_fg", "cursor_prefix", "button_style", "prefix", "suffix", "selected_fg", "selected_bg":
				if _, ok := v.(string); !ok {
					return fmt.Errorf("%s must be string", k)
				}
			case "border":
				switch b := v.(type) {
				case bool, BorderSides:
				case map[string]any:
					if err := validateBorderMap(b); err != nil {
						return err
					}
				case string:
					switch strings.ToLower(b) {
					case "single", "rounded", "double", "thick", "heavy", "ascii", "none", "true", "false":
					default:
						return fmt.Errorf("invalid border style %q", b)
					}
				default:
					return errors.New("border must be boolean, string, BorderSides, or object")
				}
			case "border_sides":
				switch bs := v.(type) {
				case BorderSides:
				case map[string]any:
					if err := validateBorderMap(bs); err != nil {
						return err
					}
				default:
					return errors.New("border_sides must be BorderSides or object")
				}
			case "border_glyphs":
				if _, ok := v.(BorderGlyphs); !ok {
					return errors.New("border_glyphs must be BorderGlyphs")
				}
			case "hidden", "disabled", "bold", "modal", "spatial_nav", "selected_bold", "focusable", "show_cursor",
				"border_top", "border_right", "border_bottom", "border_left":
				if _, ok := v.(bool); !ok {
					return fmt.Errorf("%s must be boolean", k)
				}
			case "nav":
				m, ok := v.(map[string]any)
				if !ok {
					return errors.New("nav must be object")
				}
				for dir, target := range m {
					if dir != "up" && dir != "down" && dir != "left" && dir != "right" {
						return fmt.Errorf("invalid nav direction %q", dir)
					}
					if _, ok := target.(string); !ok {
						return fmt.Errorf("nav.%s must be string", dir)
					}
				}
			case "keymap":
				m, ok := v.(map[string]any)
				if !ok {
					return errors.New("keymap must be object")
				}
				for keyName, act := range m {
					if _, ok := act.(string); !ok {
						return fmt.Errorf("keymap[%q] action must be string", keyName)
					}
				}
			case "z_index":
				if f, ok := FloatVal(v); !ok || !Finite(f) || math.Abs(f) > 1e6 {
					return errors.New("invalid z_index")
				}
			case "spans":
				a, ok := v.([]any)
				if !ok {
					return errors.New("spans must be array")
				}
				for _, v := range a {
					m, ok := v.(map[string]any)
					if !ok {
						return errors.New("span must be object")
					}
					for key, value := range m {
						switch key {
						case "text", "fg", "bg":
							s, ok := value.(string)
							if !ok {
								return errors.New("span string required")
							}
							total += len(s)
						case "bold":
							if _, ok := value.(bool); !ok {
								return errors.New("span bold must be boolean")
							}
						default:
							return fmt.Errorf("unknown span property %s", key)
						}
					}
				}
			case "items":
				if _, ok := v.([]any); !ok {
					return errors.New("items must be array")
				}
			case "columns", "rows", "show_header", "soft_wrap", "highlights", "filterable", "show_title", "show_status", "show_help", "show_pagination", "page_size", "query", "on_navigate", "allowed_extensions", "show_hidden", "file_allowed", "dir_allowed", "frames", "preset", "interval", "running", "percent", "animated", "duration", "gradient", "show_percentage", "fill", "empty", "page", "per_page", "total", "mode", "active_dot", "inactive_dot", "reset_key", "on_timeout", "keys", "full", "separator", "echo", "echo_character", "placeholder", "prompt", "char_limit", "suggestions", "show_suggestions", "validation_pattern", "validation_error", "line_numbers", "cursor_mode", "cursor_blink_speed", "char", "blink_speed", "show_size", "show_permissions", "show_line_numbers", "percent_format", "open_char", "close_char", "cursor_char", "branch_indent", "subsecond", "format":
			default:
				return fmt.Errorf("unknown property %q", k)
			}
			if s, ok := v.(string); ok {
				total += len(s)
			}
		}
		property = ""
		for _, validate := range validators {
			if validate == nil {
				continue
			}
			if err := validate(n); err != nil {
				return err
			}
		}
		if total > MaxText {
			return errors.New("tree text exceeds 1 MiB")
		}
		for _, c := range n.Children {
			if e := walk(c, d+1); e != nil {
				return e
			}
		}
		return nil
	}
	return ids, walk(root, 0)
}
