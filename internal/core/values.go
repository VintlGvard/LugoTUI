package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	MaxNodes      = 4096
	MaxText       = 1 << 20
	MaxDepth      = 64
	MaxIDBytes    = 128
	MaxModelKeys  = 1024
	MaxValueDepth = 32
)

func Finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func Str(n *Node, key, fallback string) string {
	if n == nil || n.Props == nil {
		return fallback
	}
	if v, ok := n.Props[key].(string); ok {
		return v
	}
	return fallback
}

func FloatVal(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func Num(n *Node, key string, fallback float64) float64 {
	if n == nil || n.Props == nil {
		return fallback
	}
	if v, ok := FloatVal(n.Props[key]); ok {
		return v
	}
	return fallback
}

func Flag(n *Node, key string) bool {
	if n == nil || n.Props == nil {
		return false
	}
	v, _ := n.Props[key].(bool)
	return v
}

func Focusable(n *Node) bool {
	if n == nil {
		return false
	}
	return !Flag(n, "disabled") && !Flag(n, "hidden") && (n.Type == "input" || n.Type == "textarea" || n.Type == "button" || n.Type == "list" || n.Type == "scroll" || n.Type == "table" || n.Type == "viewport" || n.Type == "tree" || n.Type == "filepicker" || n.Type == "paginator" || n.Type == "timer" || n.Type == "stopwatch" || n.Type == "help" || n.Type == "cursor" || Flag(n, "focusable") || n.RawKey != nil)
}

func estimateStringBytes(s string) int {
	total := 2 + len(s)
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '"' || b == '\\' {
			total++
		} else if b < 0x20 {
			switch b {
			case '\b', '\f', '\n', '\r', '\t':
				total++
			default:
				total += 5
			}
		}
	}
	return total
}

func estimateIntBytes(v int64) int {
	if v == 0 {
		return 1
	}
	n := 0
	u := uint64(v)
	if v < 0 {
		n = 1
		u = uint64(-v)
	}
	for u > 0 {
		n++
		u /= 10
	}
	return n
}

var errModelTooLarge = errors.New("model exceeds 1 MiB")

func estimateBytes(v any, depth, limit int) (int, error) {
	if depth > MaxDepth {
		return 0, errors.New("value nesting exceeds 64")
	}
	var size int
	switch x := v.(type) {
	case nil:
		size = 4
	case bool:
		if x {
			size = 4
		} else {
			size = 5
		}
	case int:
		size = estimateIntBytes(int64(x))
	case int64:
		size = estimateIntBytes(x)
	case float64:
		if !Finite(x) {
			return 0, errors.New("unsupported non-finite float")
		}
		var buf [32]byte
		size = len(strconv.AppendFloat(buf[:0], x, 'g', -1, 64))
	case float32:
		f := float64(x)
		if !Finite(f) {
			return 0, errors.New("unsupported non-finite float")
		}
		var buf [32]byte
		size = len(strconv.AppendFloat(buf[:0], f, 'g', -1, 32))
	case string:
		size = estimateStringBytes(x)
	case []string:
		if x == nil {
			size = 4
			break
		}
		total := 2
		for i, s := range x {
			if i > 0 {
				total++
			}
			total += estimateStringBytes(s)
			if limit > 0 && total > limit {
				return 0, errModelTooLarge
			}
		}
		size = total
	case []any:
		if x == nil {
			size = 4
			break
		}
		total := 2
		for i, item := range x {
			if i > 0 {
				total++
			}
			subLimit := 0
			if limit > 0 {
				subLimit = limit - total
				if subLimit <= 0 {
					return 0, errModelTooLarge
				}
			}
			n, err := estimateBytes(item, depth+1, subLimit)
			if err != nil {
				return 0, err
			}
			total += n
			if limit > 0 && total > limit {
				return 0, errModelTooLarge
			}
		}
		size = total
	case map[string]any:
		if x == nil {
			size = 4
			break
		}
		total := 2
		first := true
		for k, item := range x {
			if !first {
				total++
			}
			first = false
			total += estimateStringBytes(k) + 1
			subLimit := 0
			if limit > 0 {
				subLimit = limit - total
				if subLimit <= 0 {
					return 0, errModelTooLarge
				}
			}
			vn, err := estimateBytes(item, depth+1, subLimit)
			if err != nil {
				return 0, err
			}
			total += vn
			if limit > 0 && total > limit {
				return 0, errModelTooLarge
			}
		}
		size = total
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return 0, err
		}
		size = len(b)
	}
	if limit > 0 && size > limit {
		return 0, errModelTooLarge
	}
	return size, nil
}

func EstimateBytes(v any) (int, error) {
	return estimateBytes(v, 0, 0)
}

func ValidateModel(data map[string]any) error {
	if len(data) > MaxModelKeys {
		return errors.New("model exceeds 1024 keys")
	}
	_, err := estimateBytes(data, 0, MaxText)
	return err
}

func cloneValue(v any, depth int) (any, error) {
	if depth > MaxValueDepth {
		return nil, errors.New("value nesting exceeds 32")
	}
	switch value := v.(type) {
	case map[string]any:
		cp := make(map[string]any, len(value))
		for key, item := range value {
			cloned, err := cloneValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			cp[key] = cloned
		}
		return cp, nil
	case []any:
		cp := make([]any, len(value))
		for i, item := range value {
			cloned, err := cloneValue(item, depth+1)
			if err != nil {
				return nil, err
			}
			cp[i] = cloned
		}
		return cp, nil
	case []int:
		return append([]int(nil), value...), nil
	case []float64:
		return append([]float64(nil), value...), nil
	case []string:
		return append([]string(nil), value...), nil
	case BorderGlyphs, Sides, BorderSides, Rect, Theme:
		return v, nil
	case nil, string, bool, int, int64, float32, float64:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported Go type %T", v)
	}
}

func CloneValue(v any) (any, error) {
	return cloneValue(v, 0)
}

func CopyValue(v any) any {
	cloned, err := cloneValue(v, 0)
	if err != nil {
		return nil
	}
	return cloned
}

func nonNegInt(v any) (int, bool) {
	f, ok := FloatVal(v)
	if !ok || !Finite(f) {
		return 0, false
	}
	return max(0, int(f)), true
}

func PropSides(n *Node, base string) Sides {
	if n == nil || n.Props == nil || base == "" {
		return Sides{}
	}
	var sides Sides
	switch raw := n.Props[base].(type) {
	case Sides:
		sides = Sides{
			Top:    max(0, raw.Top),
			Right:  max(0, raw.Right),
			Bottom: max(0, raw.Bottom),
			Left:   max(0, raw.Left),
		}
	case map[string]any:
		if v, ok := nonNegInt(raw["x"]); ok {
			sides.Left, sides.Right = v, v
		}
		if v, ok := nonNegInt(raw["horizontal"]); ok {
			sides.Left, sides.Right = v, v
		}
		if v, ok := nonNegInt(raw["y"]); ok {
			sides.Top, sides.Bottom = v, v
		}
		if v, ok := nonNegInt(raw["vertical"]); ok {
			sides.Top, sides.Bottom = v, v
		}
		if v, ok := nonNegInt(raw["top"]); ok {
			sides.Top = v
		}
		if v, ok := nonNegInt(raw["right"]); ok {
			sides.Right = v
		}
		if v, ok := nonNegInt(raw["bottom"]); ok {
			sides.Bottom = v
		}
		if v, ok := nonNegInt(raw["left"]); ok {
			sides.Left = v
		}
	case []int:
		sides = sidesFromSlice(raw)
	case []float64:
		sides = sidesFromSlice(raw)
	case []any:
		sides = sidesFromSlice(raw)
	default:
		if v, ok := nonNegInt(raw); ok {
			sides = Sides{Top: v, Right: v, Bottom: v, Left: v}
		}
	}
	for _, k := range [2]string{base + "_x", base + "_horizontal"} {
		if v, ok := nonNegInt(n.Props[k]); ok {
			sides.Left, sides.Right = v, v
		}
	}
	for _, k := range [2]string{base + "_y", base + "_vertical"} {
		if v, ok := nonNegInt(n.Props[k]); ok {
			sides.Top, sides.Bottom = v, v
		}
	}
	if v, ok := nonNegInt(n.Props[base+"_top"]); ok {
		sides.Top = v
	}
	if v, ok := nonNegInt(n.Props[base+"_right"]); ok {
		sides.Right = v
	}
	if v, ok := nonNegInt(n.Props[base+"_bottom"]); ok {
		sides.Bottom = v
	}
	if v, ok := nonNegInt(n.Props[base+"_left"]); ok {
		sides.Left = v
	}
	return sides
}

func sidesFromSlice[T any](vals []T) Sides {
	get := func(i int) int {
		if i >= len(vals) {
			return 0
		}
		v, _ := nonNegInt(vals[i])
		return v
	}
	switch len(vals) {
	case 0:
		return Sides{}
	case 1:
		v := get(0)
		return Sides{Top: v, Right: v, Bottom: v, Left: v}
	case 2:
		v, h := get(0), get(1)
		return Sides{Top: v, Right: h, Bottom: v, Left: h}
	case 3:
		h := get(1)
		return Sides{Top: get(0), Right: h, Bottom: get(2), Left: h}
	default:
		return Sides{Top: get(0), Right: get(1), Bottom: get(2), Left: get(3)}
	}
}

func borderSidesFromMap(m map[string]any) BorderSides {
	var bs BorderSides
	if v, ok := m["top"].(bool); ok {
		bs.Top = v
	}
	if v, ok := m["right"].(bool); ok {
		bs.Right = v
	}
	if v, ok := m["bottom"].(bool); ok {
		bs.Bottom = v
	}
	if v, ok := m["left"].(bool); ok {
		bs.Left = v
	}
	return bs
}

func NodeBorderSides(n *Node) BorderSides {
	if n == nil {
		return BorderSides{}
	}
	var sides BorderSides
	switch b := n.Props["border"].(type) {
	case BorderSides:
		sides = b
	case map[string]any:
		sides = borderSidesFromMap(b)
	default:
		all := n.Type == "panel"
		if flagBool, ok := b.(bool); ok {
			all = flagBool
		} else if s, ok := b.(string); ok {
			lower := strings.ToLower(s)
			if lower == "none" || lower == "false" {
				all = false
			} else if s != "" {
				all = true
			}
		}
		if _, ok := n.Props["border_glyphs"].(BorderGlyphs); ok {
			all = true
		}
		if strings.EqualFold(Str(n, "border_style", ""), "none") {
			all = false
		}
		sides = BorderSides{Top: all, Right: all, Bottom: all, Left: all}
	}
	switch bs := n.Props["border_sides"].(type) {
	case BorderSides:
		sides = bs
	case map[string]any:
		sides = borderSidesFromMap(bs)
	}
	if v, ok := n.Props["border_top"].(bool); ok {
		sides.Top = v
	}
	if v, ok := n.Props["border_right"].(bool); ok {
		sides.Right = v
	}
	if v, ok := n.Props["border_bottom"].(bool); ok {
		sides.Bottom = v
	}
	if v, ok := n.Props["border_left"].(bool); ok {
		sides.Left = v
	}
	return sides
}
