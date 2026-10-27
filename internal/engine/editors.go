package engine

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

func validateEditor(n *Node) error {
	if n.Type != "input" && n.Type != "textarea" {
		for _, key := range []string{"placeholder", "prompt", "echo", "echo_character", "validation_pattern", "validation_error", "char_limit", "suggestions", "show_suggestions", "line_numbers", "cursor_mode", "cursor_char", "cursor_blink_speed"} {
			if _, ok := n.Props[key]; ok {
				return fmt.Errorf("%s requires input or textarea", key)
			}
		}
		return nil
	}
	for _, key := range []string{"placeholder", "prompt", "echo", "echo_character", "validation_pattern", "validation_error", "cursor_mode", "cursor_char"} {
		if v, ok := n.Props[key]; ok {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s must be a string", key)
			}
		}
	}
	if v, ok := n.Props["cursor_mode"]; ok {
		switch v.(string) {
		case "blink", "static", "hidden", "hide":
		default:
			return fmt.Errorf("cursor_mode must be blink, static, or hidden")
		}
	}
	if v, ok := n.Props["cursor_char"]; ok {
		g := graphemes(v.(string))
		if len(g) > 1 || strings.ContainsAny(v.(string), "\r\n\t") {
			return fmt.Errorf("cursor_char must be at most one grapheme")
		}
	}
	if v, ok := n.Props["cursor_blink_speed"]; ok {
		x, ok := v.(float64)
		if !ok || math.IsNaN(x) || math.IsInf(x, 0) || x < 1 || x > maxText {
			return fmt.Errorf("cursor_blink_speed must be a positive number")
		}
	}
	switch str(n, "echo", "normal") {
	case "normal", "password", "none":
	default:
		return fmt.Errorf("echo must be normal, password, or none")
	}
	if v, ok := n.Props["echo_character"]; ok {
		g := graphemes(v.(string))
		if len(g) != 1 || textWidth(g[0]) < 1 || strings.ContainsAny(g[0], "\r\n\t") {
			return fmt.Errorf("echo_character must be one visible grapheme")
		}
	}
	for _, key := range []string{"show_suggestions", "line_numbers"} {
		if v, ok := n.Props[key]; ok {
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("%s must be a boolean", key)
			}
		}
	}
	if v, ok := n.Props["char_limit"]; ok {
		x, ok := v.(float64)
		if !ok || math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > maxText || x != math.Trunc(x) {
			return fmt.Errorf("char_limit must be an integer between 0 and %d", maxText)
		}
	}
	if _, err := regexp.Compile(str(n, "validation_pattern", "")); err != nil {
		return fmt.Errorf("validation_pattern: %w", err)
	}
	if v, ok := n.Props["suggestions"]; ok {
		total := 0
		check := func(text string) error {
			if strings.ContainsAny(text, "\r\n") {
				return fmt.Errorf("suggestions must be single-line strings")
			}
			total += len(text)
			if total > maxText {
				return fmt.Errorf("suggestions exceed 1 MiB")
			}
			return nil
		}
		switch a := v.(type) {
		case []string:
			if len(a) > maxNodes {
				return fmt.Errorf("suggestions exceed %d entries", maxNodes)
			}
			for _, text := range a {
				if err := check(text); err != nil {
					return err
				}
			}
		case []any:
			if len(a) > maxNodes {
				return fmt.Errorf("suggestions exceed %d entries", maxNodes)
			}
			for _, v := range a {
				text, ok := v.(string)
				if !ok {
					return fmt.Errorf("suggestions must be single-line strings")
				}
				if err := check(text); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("suggestions must be an array of strings")
		}
	}
	return nil
}

func editorValidation(n *Node, value string) string {
	pattern := str(n, "validation_pattern", "")
	if pattern == "" {
		return ""
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err.Error()
	}
	if re.MatchString(value) {
		return ""
	}
	return str(n, "validation_error", "value does not match validation_pattern")
}

func editorGraphemes(n *Node, s *WidgetState) []string {
	g := graphemes(s.Value)
	echo := str(n, "echo", "normal")
	if echo == "normal" {
		return g
	}
	mask := str(n, "echo_character", "*")
	if echo == "none" {
		mask = ""
	}
	for i := range g {
		if g[i] != "\n" || n.Type != "textarea" {
			g[i] = mask
		}
	}
	return g
}

func editorDisplay(n *Node, s *WidgetState) string {
	if s.Value == "" {
		return str(n, "placeholder", "")
	}
	return strings.Join(editorGraphemes(n, s), "")
}

func editorPrompt(n *Node) string { return str(n, "prompt", "") }

func editorSuggestions(n *Node, s *WidgetState) []string {
	if n.Type != "input" || str(n, "echo", "normal") != "normal" || s.Value == "" {
		return nil
	}
	if v, ok := n.Props["show_suggestions"].(bool); ok && !v {
		return nil
	}
	var values []string
	switch a := n.Props["suggestions"].(type) {
	case []string:
		values = a
	case []any:
		for _, v := range a {
			if text, ok := v.(string); ok {
				values = append(values, text)
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	var out []string
	prefix := len(graphemes(s.Value))
	for _, v := range values {
		if strings.HasPrefix(v, s.Value) && v != s.Value {
			gs := graphemes(v)
			if prefix <= len(gs) && strings.Join(gs[:prefix], "") == s.Value {
				out = append(out, v)
			}
		}
	}
	return out
}

func editorSuggestion(n *Node, s *WidgetState) string {
	matches := editorSuggestions(n, s)
	if len(matches) == 0 || s.Cursor != len(graphemes(s.Value)) || (s.Anchor >= 0 && s.Anchor != s.Cursor) {
		return ""
	}
	index := ((s.Selected % len(matches)) + len(matches)) % len(matches)
	return strings.TrimPrefix(matches[index], s.Value)
}

func editorInsert(n *Node, s *WidgetState, text string) (bool, error) {
	g := graphemes(s.Value)
	a, b := max(0, min(s.Cursor, len(g))), max(0, min(s.Cursor, len(g)))
	if s.Anchor >= 0 {
		a = min(a, min(s.Anchor, len(g)))
		b = max(b, min(s.Anchor, len(g)))
	}
	candidate := strings.Join(g[:a], "") + text + strings.Join(g[b:], "")
	if limit := int(num(n, "char_limit", 0)); limit > 0 && len(graphemes(candidate)) > limit {
		remaining := max(0, limit-(len(g)-(b-a)))
		insert := graphemes(text)
		if len(insert) > remaining {
			text = strings.Join(insert[:remaining], "")
		}
		if text == "" {
			return false, nil
		}
	}
	value := strings.Join(g[:a], "") + text + strings.Join(g[b:], "")
	if len(value) > maxText {
		return false, errInputLimit
	}
	if value == s.Value {
		return false, nil
	}
	editSave(s)
	s.Cursor = a
	s.Anchor = b
	replaceSelection(s, text)
	s.Cursor = min(len(graphemes(strings.Join(g[:a], "")+text)), len(graphemes(s.Value)))
	s.Selected = 0
	return true, nil
}

func editorWord(g []string, cursor, direction int) int {
	space := func(i int) bool {
		for _, r := range g[i] {
			return unicode.IsSpace(r)
		}
		return false
	}
	if direction < 0 {
		for cursor > 0 && space(cursor-1) {
			cursor--
		}
		for cursor > 0 && !space(cursor-1) {
			cursor--
		}
	} else {
		for cursor < len(g) && space(cursor) {
			cursor++
		}
		for cursor < len(g) && !space(cursor) {
			cursor++
		}
	}
	return cursor
}

func (r *Runtime) editorKey(n *Node, e *tcell.EventKey) (bool, error) {
	if n == nil || (n.Type != "input" && n.Type != "textarea") {
		return false, nil
	}
	s := r.State[n.ID]
	if s == nil {
		return false, nil
	}
	g := graphemes(s.Value)
	s.Cursor = max(0, min(s.Cursor, len(g)))
	old := s.Cursor
	alt := e.Modifiers()&tcell.ModAlt != 0
	ctrl := e.Modifiers()&tcell.ModCtrl != 0
	shift := e.Modifiers()&tcell.ModShift != 0
	key := e.Key()
	if key == tcell.KeyTab && !ctrl && !alt || key == tcell.KeyRight && !ctrl && !alt && !shift {
		if suggestion := editorSuggestion(n, s); suggestion != "" {
			changed, err := editorInsert(n, s, suggestion)
			if err != nil || !changed {
				return true, err
			}
			return true, r.changed(n, s)
		}
	}
	if key == tcell.KeyCtrlN || key == tcell.KeyCtrlP {
		matches := editorSuggestions(n, s)
		if len(matches) == 0 {
			return false, nil
		}
		direction := 1
		if key == tcell.KeyCtrlP {
			direction = -1
		}
		s.Selected = (s.Selected + direction + len(matches)) % len(matches)
		return true, nil
	}
	target := -1
	deleting := false
	switch {
	case (key == tcell.KeyLeft && (ctrl || alt)) || (key == tcell.KeyRune && alt && e.Rune() == 'b'):
		target = editorWord(g, old, -1)
	case (key == tcell.KeyRight && (ctrl || alt)) || (key == tcell.KeyRune && alt && e.Rune() == 'f'):
		target = editorWord(g, old, 1)
	case key == tcell.KeyCtrlB:
		target = max(0, old-1)
	case key == tcell.KeyCtrlF:
		target = min(len(g), old+1)
	case key == tcell.KeyCtrlD:
		target = min(len(g), old+1)
		deleting = true
	case key == tcell.KeyCtrlE:
		target = old
		for target < len(g) && g[target] != "\n" {
			target++
		}
	case key == tcell.KeyHome && ctrl:
		target = 0
	case key == tcell.KeyEnd && ctrl:
		target = len(g)
	case key == tcell.KeyCtrlW || ((key == tcell.KeyBackspace || key == tcell.KeyBackspace2) && (alt || ctrl)):
		target = editorWord(g, old, -1)
		deleting = true
	case (key == tcell.KeyDelete && (alt || ctrl)) || (key == tcell.KeyRune && alt && e.Rune() == 'd'):
		target = editorWord(g, old, 1)
		deleting = true
	case key == tcell.KeyCtrlU:
		target = old
		for target > 0 && g[target-1] != "\n" {
			target--
		}
		deleting = true
	case key == tcell.KeyCtrlK:
		target = old
		for target < len(g) && g[target] != "\n" {
			target++
		}
		if target == old && target < len(g) {
			target++
		}
		deleting = true
	case key == tcell.KeyRune && !alt && !ctrl:
		if unicode.IsControl(e.Rune()) {
			return true, nil
		}
		changed, err := editorInsert(n, s, string(e.Rune()))
		if err != nil || !changed {
			return true, err
		}
		return true, r.changed(n, s)
	case key == tcell.KeyEnter && n.Type == "textarea" && !ctrl:
		changed, err := editorInsert(n, s, "\n")
		if err != nil || !changed {
			return true, err
		}
		return true, r.changed(n, s)
	default:
		return key == tcell.KeyRune && (alt || ctrl), nil
	}
	if deleting {
		if s.Anchor < 0 || s.Anchor == old {
			s.Anchor = target
		}
		if s.Anchor == old {
			s.Anchor = -1
			return true, nil
		}
		editSave(s)
		replaceSelection(s, "")
		s.Selected = 0
		return true, r.changed(n, s)
	}
	s.Cursor = target
	if shift {
		if s.Anchor < 0 {
			s.Anchor = old
		}
	} else {
		s.Anchor = -1
	}
	return true, nil
}

func editorLinePrefix(n *Node, value string, line int) string {
	prefix := editorPrompt(n)
	if n.Type == "textarea" && flag(n, "line_numbers") {
		digits := len(fmt.Sprint(strings.Count(value, "\n") + 1))
		if line > 0 {
			prefix += fmt.Sprintf("%*d ", digits, line)
		} else {
			prefix += strings.Repeat(" ", digits+1)
		}
	}
	return prefix
}

func editorPrefixWidth(n *Node, value string) int { return textWidth(editorLinePrefix(n, value, 1)) }

func editorRect(n *Node, s *WidgetState) Rect {
	cr := contentRect(n)
	width := min(cr.W, editorPrefixWidth(n, s.Value))
	cr.X += width
	cr.W -= width
	return cr
}

func editorIndex(n *Node, s *WidgetState, width int) ([]string, [][2]int) {
	if str(n, "echo", "normal") == "normal" {
		return s.EditorIndex(max(1, width), n.Type == "textarea")
	}
	g := editorGraphemes(n, s)
	return g, editorPositions(g, max(1, width), n.Type == "textarea")
}
