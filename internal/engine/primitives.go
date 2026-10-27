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
	if v, ok := n.Props["interval"].(float64); ok && v > 0 {
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
			f, ok := v.(float64)
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

func (r *Runtime) Tick(now time.Time) error {
	if r.Root == nil {
		return nil
	}
	for original := range visibleNodes(r.Root) {
		n := original
		if r.Nodes != nil {
			n = r.Nodes[original.ID]
			if n == nil {
				continue
			}
		}
		switch n.Type {
		case "spinner", "progress", "timer", "stopwatch", "cursor", "input", "textarea":
		default:
			continue
		}
		p := r.primitive(n)
		frame, percent, blink := p.frame, p.percent, p.cursorBlinked
		clock := ""
		if n.Type == "timer" {
			clock = primitiveClock(n, time.Duration(p.duration*float64(time.Millisecond))-p.elapsed, true)
		} else if n.Type == "stopwatch" {
			clock = primitiveClock(n, p.elapsed, false)
		}
		delta := time.Duration(0)
		if p.initialized && !now.Before(p.last) {
			delta = now.Sub(p.last)
		}
		if !p.initialized || !now.Before(p.last) {
			p.last = now
		}
		p.initialized = true
		switch n.Type {
		case "cursor":
			mode := str(n, "mode", "blink")
			if mode == "blink" {
				speed := time.Duration(num(n, "blink_speed", 530)) * time.Millisecond
				if speed <= 0 {
					speed = 530 * time.Millisecond
				}
				p.cursorElapsed += delta
				if p.cursorElapsed >= speed {
					if (p.cursorElapsed/speed)%2 != 0 {
						p.cursorBlinked = !p.cursorBlinked
					}
					p.cursorElapsed %= speed
				}
			} else {
				p.cursorBlinked = (mode == "hidden" || mode == "hide")
			}
		case "input", "textarea":
			if r.Focus == n.ID {
				mode := str(n, "cursor_mode", "blink")
				if mode == "blink" {
					speed := time.Duration(num(n, "cursor_blink_speed", 530)) * time.Millisecond
					if speed <= 0 {
						speed = 530 * time.Millisecond
					}
					p.cursorElapsed += delta
					if p.cursorElapsed >= speed {
						if (p.cursorElapsed/speed)%2 != 0 {
							p.cursorBlinked = !p.cursorBlinked
						}
						p.cursorElapsed %= speed
					}
				} else {
					p.cursorBlinked = (mode == "hidden" || mode == "hide")
				}
			}
		case "spinner":
			if p.running {
				interval := primitiveInterval(n)
				if interval <= 0 {
					interval = time.Millisecond
				}
				p.frameElapsed += delta
				steps := p.frameElapsed / interval
				p.frameElapsed %= interval
				p.frame = (p.frame + int(steps%time.Duration(len(primitiveFrames(n))))) % len(primitiveFrames(n))
			}
		case "progress":
			p.progressElapsed += delta
			duration := time.Duration(num(n, "duration", 250)) * time.Millisecond
			if duration <= 0 || !primitiveBool(n, "animated", true) {
				p.percent = p.target
			} else {
				fraction := min(1, float64(p.progressElapsed)/float64(duration))
				p.percent = p.from + (p.target-p.from)*fraction
			}
		case "timer", "stopwatch":
			if !p.running || p.timedOut {
				continue
			}
			p.elapsed += delta
			if n.Type == "timer" && p.elapsed >= time.Duration(p.duration*float64(time.Millisecond)) {
				p.elapsed = time.Duration(p.duration * float64(time.Millisecond))
				p.timedOut = true
				if action := str(n, "on_timeout", ""); action != "" {
					if err := r.Action(action, map[string]any{"id": n.ID, "elapsed": float64(p.elapsed / time.Millisecond)}); err != nil {
						return err
					}
				}
			}
		}
		if n.Type == "timer" {
			if clock != primitiveClock(n, time.Duration(p.duration*float64(time.Millisecond))-p.elapsed, true) {
				r.AnimationRevision++
			}
		} else if n.Type == "stopwatch" {
			if clock != primitiveClock(n, p.elapsed, false) {
				r.AnimationRevision++
			}
		} else if frame != p.frame || percent != p.percent || blink != p.cursorBlinked {
			r.AnimationRevision++
		}
	}
	return nil
}

func (r *Runtime) NeedsTick() bool {
	if r.Root == nil {
		return false
	}
	for n := range visibleNodes(r.Root) {
		switch n.Type {
		case "cursor":
			if str(n, "mode", "blink") == "blink" {
				return true
			}
		case "input", "textarea":
			if r.Focus == n.ID && str(n, "cursor_mode", "blink") == "blink" {
				return true
			}
		case "spinner", "timer", "stopwatch":
			p := r.primitive(n)
			if p.running && !p.timedOut {
				return true
			}
		case "progress":
			p := r.primitive(n)
			if p.percent != p.target {
				return true
			}
		}
	}
	return false
}

func primitiveClock(n *Node, d time.Duration, countdown bool) string {
	if d < 0 {
		d = 0
	}
	subsecond := flag(n, "subsecond") || (num(n, "interval", 1000) > 0 && num(n, "interval", 1000) < 1000)
	fmtType := str(n, "format", "")
	if fmtType == "duration" || fmtType == "compact" {
		if d == 0 {
			return "0s"
		}
		if subsecond {
			return fmt.Sprintf("%.1fs", float64(d/time.Millisecond)/1000.0)
		}
		secs := int64(d / time.Second)
		if countdown && d%time.Second != 0 {
			secs++
		}
		return fmt.Sprintf("%ds", secs)
	}
	totalMillis := int64(d / time.Millisecond)
	seconds := totalMillis / 1000
	if countdown && !subsecond && totalMillis%1000 != 0 {
		seconds++
	}
	if subsecond {
		tenth := (totalMillis % 1000) / 100
		return fmt.Sprintf("%02d:%02d:%02d.%d", seconds/3600, (seconds/60)%60, seconds%60, tenth)
	}
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds/60)%60, seconds%60)
}

func (r *Runtime) paintPrimitive(screen tcell.Screen, n *Node, st tcell.Style, cr, clip Rect) bool {
	switch n.Type {
	case "spinner", "progress", "paginator", "timer", "stopwatch", "help", "cursor":
	default:
		return false
	}
	p := r.primitive(n)
	switch n.Type {
	case "cursor":
		mode := str(n, "mode", "blink")
		if mode == "hidden" || mode == "hide" || (mode == "blink" && p.cursorBlinked) {
			put(screen, cr.X, cr.Y, " ", st, clip)
		} else {
			ch := str(n, "char", "█")
			put(screen, cr.X, cr.Y, ch, st.Reverse(true), clip)
		}
	case "spinner":
		a := primitiveFrames(n)
		put(screen, cr.X, cr.Y, a[p.frame%len(a)], st, clip)
	case "progress":
		label := ""
		if primitiveBool(n, "show_percentage", true) {
			fmtStr := str(n, "percent_format", " %3.0f%%")
			label = fmt.Sprintf(fmtStr, p.percent*100)
		}
		width := max(0, cr.W-uniseg.StringWidth(label))
		filled := int(math.Round(float64(width) * p.percent))
		colors, _ := primitiveStrings(n.Props["gradient"])
		for x := 0; x < width; x++ {
			glyph := str(n, "empty", "░")
			style := st
			if x < filled {
				glyph = str(n, "fill", "█")
				if len(colors) == 2 {
					a, b := tcell.GetColor(colors[0]), tcell.GetColor(colors[1])
					ar, ag, ab := a.RGB()
					br, bg, bb := b.RGB()
					f := float64(x) / float64(max(1, width-1))
					style = style.Foreground(tcell.NewRGBColor(int32(float64(ar)+(float64(br-ar)*f)), int32(float64(ag)+(float64(bg-ag)*f)), int32(float64(ab)+(float64(bb-ab)*f))))
				}
			}
			put(screen, cr.X+x, cr.Y, glyph, style, clip)
		}
		put(screen, cr.X+width, cr.Y, label, st, clip)
	case "paginator":
		pages := primitivePages(n)
		if str(n, "mode", "numeric") == "dots" {
			viewport := intersect(cr, clip)
			width := min(pages, max(0, viewport.W))
			start := max(0, min(p.page-width/2, pages-width))
			for x := 0; x < width; x++ {
				glyph := str(n, "inactive_dot", "○")
				if start+x == p.page {
					glyph = str(n, "active_dot", "●")
				}
				put(screen, viewport.X+x, cr.Y, glyph, st, clip)
			}
		} else {
			put(screen, cr.X, cr.Y, fmt.Sprintf("%d/%d", p.page+1, pages), st, clip)
		}
	case "timer":
		stClock := st
		if r.Focus == n.ID {
			stClock = stClock.Underline(true).Bold(true)
		}
		remaining := time.Duration(p.duration*float64(time.Millisecond)) - p.elapsed
		if remaining < 0 {
			remaining = 0
		}
		put(screen, cr.X, cr.Y, primitiveClock(n, remaining, true), stClock, clip)
	case "stopwatch":
		stClock := st
		if r.Focus == n.ID {
			stClock = stClock.Underline(true).Bold(true)
		}
		put(screen, cr.X, cr.Y, primitiveClock(n, p.elapsed, false), stClock, clip)
	case "help":
		bindings, _ := n.Props["keys"].([]any)
		x, y := cr.X, cr.Y
		for _, v := range bindings {
			b, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if enabled, ok := b["enabled"].(bool); ok && !enabled {
				continue
			}
			keys, _ := primitiveStrings(b["keys"])
			if len(keys) == 0 {
				continue
			}
			help, _ := b["help"].(string)
			label := strings.Join(keys, "/") + " " + help
			if p.full {
				put(screen, cr.X, y, label, st, clip)
				y++
				if y >= cr.Y+cr.H {
					break
				}
			} else {
				if x > cr.X {
					sep := str(n, "separator", " • ")
					put(screen, x, y, sep, st, clip)
					x += uniseg.StringWidth(sep)
				}
				put(screen, x, y, label, st, clip)
				x += uniseg.StringWidth(label)
				if x >= cr.X+cr.W {
					break
				}
			}
		}
	}
	return true
}

func (r *Runtime) keyPrimitive(n *Node, e *tcell.EventKey) (bool, error) {
	if flag(n, "disabled") {
		return false, nil
	}
	switch n.Type {
	case "paginator":
		p := r.primitive(n)
		old := p.page
		switch e.Key() {
		case tcell.KeyLeft, tcell.KeyUp, tcell.KeyPgUp:
			p.page--
		case tcell.KeyRight, tcell.KeyDown, tcell.KeyPgDn:
			p.page++
		case tcell.KeyHome:
			p.page = 0
		case tcell.KeyEnd:
			p.page = primitivePages(n) - 1
		case tcell.KeyRune:
			switch e.Rune() {
			case 'h', 'k':
				p.page--
			case 'l', 'j':
				p.page++
			default:
				return false, nil
			}
		default:
			return false, nil
		}
		p.page = max(0, min(p.page, primitivePages(n)-1))
		if p.page == old {
			return true, nil
		}
		if action := str(n, "on_change", ""); action != "" {
			err := r.Action(action, map[string]any{"id": n.ID, "page": p.page, "per_page": num(n, "per_page", 1), "total": num(n, "total", 0)})
			if err != nil {
				p.page = old
			}
			return true, err
		}
		return true, nil
	case "timer", "stopwatch":
		p := r.primitive(n)
		switch {
		case e.Key() == tcell.KeyEnter || (e.Key() == tcell.KeyRune && e.Rune() == ' '):
			if p.timedOut {
				p.elapsed = 0
				p.timedOut = false
				p.running = true
			} else {
				p.running = !p.running
			}
			p.last = time.Time{}
			p.initialized = false
			p.runningProp = p.running
			if action := str(n, "on_change", ""); action != "" {
				_ = r.Action(action, map[string]any{"id": n.ID, "running": p.running, "elapsed": float64(p.elapsed / time.Millisecond)})
			}
			return true, nil
		case e.Key() == tcell.KeyRune && (e.Rune() == 'r' || e.Rune() == 'R'):
			p.elapsed = 0
			p.timedOut = false
			p.last = time.Time{}
			p.initialized = false
			p.running = primitiveBool(n, "running", true)
			p.runningProp = p.running
			if action := str(n, "on_change", ""); action != "" {
				_ = r.Action(action, map[string]any{"id": n.ID, "running": p.running, "elapsed": float64(p.elapsed / time.Millisecond)})
			}
			return true, nil
		default:
			return false, nil
		}
	case "help":
		if e.Key() == tcell.KeyRune && e.Rune() == '?' {
			p := r.primitive(n)
			p.full = !p.full
			return true, nil
		}
	}
	return false, nil
}

func primitiveKeyMatches(key string, e *tcell.EventKey) bool {
	if key == e.Name() || key == strings.ReplaceAll(e.Name(), "-", "+") {
		return true
	}
	if e.Key() == tcell.KeyRune && e.Modifiers() == tcell.ModNone && key == string(e.Rune()) {
		return true
	}
	aliases := map[string]tcell.Key{"enter": tcell.KeyEnter, "esc": tcell.KeyEscape, "escape": tcell.KeyEscape, "tab": tcell.KeyTab, "backspace": tcell.KeyBackspace2, "up": tcell.KeyUp, "down": tcell.KeyDown, "left": tcell.KeyLeft, "right": tcell.KeyRight, "home": tcell.KeyHome, "end": tcell.KeyEnd, "pgup": tcell.KeyPgUp, "pgdown": tcell.KeyPgDn}
	if k, ok := aliases[strings.ToLower(key)]; ok && k == e.Key() && e.Modifiers() == tcell.ModNone {
		return true
	}
	return e.Key() != tcell.KeyRune && strings.EqualFold(strings.ReplaceAll(key, "+", "-"), strings.ReplaceAll(e.Name(), "+", "-"))
}

func (r *Runtime) bindingKey(e *tcell.EventKey) (bool, error) {
	if r.Root == nil {
		return false, nil
	}
	for n := range visibleNodes(r.scope()) {
		if n.Type != "help" || flag(n, "disabled") {
			continue
		}
		bindings, _ := n.Props["keys"].([]any)
		for _, v := range bindings {
			b, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if enabled, ok := b["enabled"].(bool); ok && !enabled {
				continue
			}
			action, _ := b["action"].(string)
			if action == "" {
				continue
			}
			keys, _ := primitiveStrings(b["keys"])
			for _, key := range keys {
				if primitiveKeyMatches(key, e) {
					return true, r.Action(action, map[string]any{"id": n.ID, "key": e.Name(), "focus": r.Focus})
				}
			}
		}
	}
	return false, nil
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
