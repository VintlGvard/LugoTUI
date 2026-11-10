package engine

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type primitiveState struct {
	initialized                    bool
	last                           time.Time
	elapsed                        time.Duration
	duration, resetKey             float64
	running, runningProp, timedOut bool
	frame                          int
	frameElapsed                   time.Duration
	percent, from, target          float64
	progressElapsed                time.Duration
	page                           int
	pageProp                       float64
	full, fullProp                 bool
	cursorBlinked                  bool
	cursorElapsed                  time.Duration
}

func clonePrimitiveState(s *primitiveState) *primitiveState {
	if s == nil {
		return nil
	}
	copy := *s
	return &copy
}

var primitiveSpinners = map[string][]string{
	"line":      {"|", "/", "-", "\\"},
	"dot":       {"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	"mini_dot":  {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"jump":      {"⢄", "⢂", "⢁", "⡁", "⡈", "⡐", "⡠"},
	"pulse":     {"█", "▓", "▒", "░"},
	"points":    {"∙∙∙", "●∙∙", "∙●∙", "∙∙●"},
	"globe":     {"🌍", "🌎", "🌏"},
	"moon":      {"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
	"monkey":    {"🙈", "🙉", "🙊"},
	"meter":     {"▱▱▱", "▰▱▱", "▰▰▱", "▰▰▰", "▰▰▱", "▰▱▱", "▱▱▱"},
	"hamburger": {"☱", "☲", "☴", "☲"},
	"ellipsis":  {"", ".", "..", "..."},
}

var primitiveSpinnerFPS = map[string]int{
	"line":      100,
	"dot":       100,
	"mini_dot":  83,
	"jump":      100,
	"pulse":     125,
	"points":    143,
	"globe":     250,
	"moon":      125,
	"monkey":    333,
	"meter":     143,
	"hamburger": 333,
	"ellipsis":  333,
}

func primitiveInterval(n *Node) time.Duration {
	if v, ok := floatVal(n.Props["interval"]); ok && v > 0 {
		return time.Duration(v) * time.Millisecond
	}
	preset := str(n, "preset", "line")
	if fps, ok := primitiveSpinnerFPS[preset]; ok {
		return time.Duration(fps) * time.Millisecond
	}
	return 100 * time.Millisecond
}

func primitiveBool(n *Node, key string, fallback bool) bool {
	v, ok := n.Props[key].(bool)
	if !ok {
		return fallback
	}
	return v
}

func primitiveStrings(v any) ([]string, bool) {
	if a, ok := v.([]string); ok {
		return a, true
	}
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(a))
	for i, v := range a {
		var ok bool
		out[i], ok = v.(string)
		if !ok {
			return nil, false
		}
	}
	return out, true
}

func validatePrimitive(n *Node) error {
	allowed := map[string]string{
		"cursor":    " mode blink_speed char ",
		"spinner":   " frames preset interval running ",
		"progress":  " percent animated duration gradient show_percentage fill empty percent_format ",
		"paginator": " page per_page total mode active_dot inactive_dot on_change ",
		"timer":     " duration running reset_key on_timeout interval format subsecond ",
		"stopwatch": " running reset_key interval format subsecond ",
		"help":      " keys full separator ",
	}
	specialized := " frames preset interval running percent animated duration gradient show_percentage fill empty percent_format page per_page total mode active_dot inactive_dot reset_key on_timeout keys full separator blink_speed char format subsecond "
	for key := range n.Props {
		if strings.Contains(specialized, " "+key+" ") && !strings.Contains(allowed[n.Type], " "+key+" ") {
			return &nodeError{Node: n, Property: key, Err: fmt.Errorf("property unsupported by %s", n.Type)}
		}
	}
	switch n.Type {
	case "spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor":
	default:
		return nil
	}
	fail := func(k, msg string) error { return &nodeError{Node: n, Property: k, Err: fmt.Errorf("%s", msg)} }
	numeric := map[string][2]float64{}
	switch n.Type {
	case "cursor":
		numeric["blink_speed"] = [2]float64{1, 86400000}
	case "spinner":
		numeric["interval"] = [2]float64{1, 86400000}
	case "progress":
		numeric["percent"] = [2]float64{0, 1}
		numeric["duration"] = [2]float64{0, 86400000}
	case "paginator":
		numeric["page"] = [2]float64{0, 1e9}
		numeric["per_page"] = [2]float64{1, 1e9}
		numeric["total"] = [2]float64{0, 1e9}
	case "timer":
		numeric["duration"] = [2]float64{0, 86400000000}
		numeric["reset_key"] = [2]float64{-1e15, 1e15}
		numeric["interval"] = [2]float64{1, 86400000}
	case "stopwatch":
		numeric["reset_key"] = [2]float64{-1e15, 1e15}
		numeric["interval"] = [2]float64{1, 86400000}
	}
	for k, bounds := range numeric {
		if v, exists := n.Props[k]; exists {
			f, ok := floatVal(v)
			if !ok || !finite(f) || f < bounds[0] || f > bounds[1] {
				return fail(k, "number outside supported range")
			}
			if n.Type == "paginator" && f != math.Trunc(f) {
				return fail(k, "must be an integer")
			}
		}
	}
	for _, k := range []string{"running", "animated", "show_percentage", "full"} {
		if v, ok := n.Props[k]; ok {
			if _, ok := v.(bool); !ok {
				return fail(k, "must be boolean")
			}
		}
	}
	for _, k := range []string{"fill", "empty", "active_dot", "inactive_dot"} {
		if v, ok := n.Props[k]; ok {
			s, ok := v.(string)
			if !ok || len(graphemes(s)) != 1 || uniseg.StringWidth(s) != 1 || strings.ContainsAny(s, "\r\n\t") {
				return fail(k, "must be one single-cell grapheme")
			}
		}
	}
	if n.Type == "spinner" {
		if v, exists := n.Props["preset"]; exists {
			s, ok := v.(string)
			if !ok || primitiveSpinners[s] == nil {
				return fail("preset", "unknown spinner preset")
			}
		}
		if v, exists := n.Props["frames"]; exists {
			a, ok := primitiveStrings(v)
			if !ok || len(a) == 0 || len(a) > 256 {
				return fail("frames", "requires 1..256 strings")
			}
			for _, frame := range a {
				if !utf8.ValidString(frame) || utf8.RuneCountInString(frame) > 128 || strings.ContainsAny(frame, "\r\n\t") {
					return fail("frames", "frames must be valid single-line strings of at most 128 runes")
				}
			}
		}
	}
	if n.Type == "progress" {
		if v, exists := n.Props["percent_format"]; exists {
			if _, ok := v.(string); !ok {
				return fail("percent_format", "must be a string")
			}
		}
		if v, exists := n.Props["gradient"]; exists {
			a, ok := primitiveStrings(v)
			if !ok || len(a) != 2 {
				return fail("gradient", "requires two color strings")
			}
			for _, c := range a {
				if !tcell.GetColor(c).Valid() {
					return fail("gradient", "invalid color")
				}
			}
		}
	}
	if n.Type == "cursor" {
		if v, exists := n.Props["mode"]; exists && v != "blink" && v != "static" && v != "hidden" && v != "hide" {
			return fail("mode", "expected blink, static, or hidden")
		}
		if v, exists := n.Props["char"]; exists {
			s, ok := v.(string)
			if !ok || (len(graphemes(s)) > 1) || strings.ContainsAny(s, "\r\n\t") {
				return fail("char", "must be at most one grapheme")
			}
		}
	}
	if n.Type == "paginator" {
		if v, exists := n.Props["mode"]; exists && v != "numeric" && v != "dots" {
			return fail("mode", "expected numeric or dots")
		}
	}
	if n.Type == "timer" || n.Type == "stopwatch" {
		if v, exists := n.Props["format"]; exists {
			s, ok := v.(string)
			if !ok || (s != "clock" && s != "duration" && s != "compact") {
				return fail("format", "expected clock, duration, or compact")
			}
		}
		if v, exists := n.Props["subsecond"]; exists {
			if _, ok := v.(bool); !ok {
				return fail("subsecond", "expected boolean")
			}
		}
	}
	if n.Type == "help" {
		if v, exists := n.Props["separator"]; exists {
			s, ok := v.(string)
			if !ok || len(s) > 128 || strings.ContainsAny(s, "\r\n\t") {
				return fail("separator", "requires single-line string of at most 128 bytes")
			}
		}
		if v, exists := n.Props["keys"]; exists {
			a, ok := v.([]any)
			if !ok || len(a) > 256 {
				return fail("keys", "requires at most 256 key bindings")
			}
			for _, v := range a {
				b, ok := v.(map[string]any)
				if !ok {
					return fail("keys", "binding must be a table")
				}
				keys, ok := primitiveStrings(b["keys"])
				if !ok || len(keys) == 0 || len(keys) > 32 {
					return fail("keys", "binding requires 1..32 keys")
				}
				for _, k := range keys {
					if len(k) == 0 || len(k) > 128 || strings.ContainsAny(k, "\r\n\t") {
						return fail("keys", "invalid key")
					}
				}
				for _, k := range []string{"help", "action"} {
					if v, exists := b[k]; exists {
						s, ok := v.(string)
						if !ok || len(s) > 4096 || strings.ContainsAny(s, "\r\n\t") {
							return fail("keys", "invalid binding "+k)
						}
					}
				}
				if v, exists := b["enabled"]; exists {
					if _, ok := v.(bool); !ok {
						return fail("keys", "enabled must be boolean")
					}
				}
			}
		}
	}
	return nil
}

func (r *Runtime) primitive(n *Node) *primitiveState {
	if r.State == nil {
		r.State = map[string]*WidgetState{}
	}
	if r.State[n.ID] == nil {
		r.State[n.ID] = &WidgetState{Anchor: -1}
	}
	w := r.State[n.ID]
	running := primitiveBool(n, "running", true)
	duration, reset := num(n, "duration", 0), num(n, "reset_key", 0)
	if w.primitive == nil {
		w.primitive = &primitiveState{duration: duration, resetKey: reset, running: running, runningProp: running, page: int(num(n, "page", 0)), pageProp: num(n, "page", 0), full: flag(n, "full"), fullProp: flag(n, "full")}
	}
	p := w.primitive
	if duration != p.duration || reset != p.resetKey {
		p.duration, p.resetKey = duration, reset
		if n.Type == "timer" || n.Type == "stopwatch" {
			p.elapsed = 0
			p.timedOut = false
			p.last = time.Time{}
			p.initialized = false
			p.running = running
		}
	}
	if running != p.runningProp {
		p.runningProp = running
		p.running = running
		if (n.Type == "timer" || n.Type == "stopwatch") && p.timedOut && running {
			p.timedOut = false
			p.elapsed = 0
		}
		p.last = time.Time{}
		p.initialized = false
	}
	if page := num(n, "page", 0); page != p.pageProp {
		p.pageProp = page
		p.page = int(page)
	}
	if full := flag(n, "full"); full != p.fullProp {
		p.fullProp = full
		p.full = full
	}
	target := num(n, "percent", 0)
	if target != p.target {
		p.from = p.percent
		p.target = target
		p.progressElapsed = 0
	}
	if !primitiveBool(n, "animated", true) || num(n, "duration", 250) == 0 {
		p.percent = p.target
	}
	if n.Type == "paginator" {
		p.page = max(0, min(p.page, primitivePages(n)-1))
	}
	return p
}

func primitivePages(n *Node) int {
	return max(1, int(math.Ceil(num(n, "total", 0)/num(n, "per_page", 1))))
}

func primitiveFrames(n *Node) []string {
	if a, ok := primitiveStrings(n.Props["frames"]); ok && len(a) > 0 {
		return a
	}
	if a := primitiveSpinners[str(n, "preset", "dot")]; len(a) > 0 {
		return a
	}
	return primitiveSpinners["dot"]
}

func primitiveText(n *Node) string {
	switch n.Type {
	case "spinner":
		widest := ""
		for _, frame := range primitiveFrames(n) {
			if uniseg.StringWidth(frame) > uniseg.StringWidth(widest) {
				widest = frame
			}
		}
		return widest
	case "progress":
		return "██████████ 100%"
	case "paginator":
		if str(n, "mode", "numeric") == "dots" {
			return strings.Repeat("○", min(primitivePages(n), 256))
		}
		return fmt.Sprintf("%d/%d", primitivePages(n), primitivePages(n))
	case "timer", "stopwatch":
		if flag(n, "subsecond") || (num(n, "interval", 1000) > 0 && num(n, "interval", 1000) < 1000) {
			return "00:00:00.0"
		}
		return "00:00:00"
	case "help":
		a, _ := n.Props["keys"].([]any)
		lines := []string{}
		for _, v := range a {
			b, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if enabled, ok := b["enabled"].(bool); ok && !enabled {
				continue
			}
			keys, _ := primitiveStrings(b["keys"])
			help, _ := b["help"].(string)
			lines = append(lines, strings.Join(keys, "/")+" "+help)
		}
		sep := str(n, "separator", " • ")
		if flag(n, "full") {
			sep = "\n"
		}
		return strings.Join(lines, sep)
	}
	return ""
}
