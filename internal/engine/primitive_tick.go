package engine

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

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
		frame, percent, blink := p.Frame, p.Percent, p.CursorBlinked
		clock := ""
		if n.Type == "timer" {
			clock = primitiveClock(n, time.Duration(p.Duration*float64(time.Millisecond))-p.Elapsed, true)
		} else if n.Type == "stopwatch" {
			clock = primitiveClock(n, p.Elapsed, false)
		}
		delta := time.Duration(0)
		if p.Initialized && !now.Before(p.Last) {
			delta = now.Sub(p.Last)
		}
		if !p.Initialized || !now.Before(p.Last) {
			p.Last = now
		}
		p.Initialized = true
		switch n.Type {
		case "cursor":
			mode := str(n, "mode", "blink")
			if mode == "blink" {
				speed := time.Duration(num(n, "blink_speed", 530)) * time.Millisecond
				if speed <= 0 {
					speed = 530 * time.Millisecond
				}
				p.CursorElapsed += delta
				if p.CursorElapsed >= speed {
					if (p.CursorElapsed/speed)%2 != 0 {
						p.CursorBlinked = !p.CursorBlinked
					}
					p.CursorElapsed %= speed
				}
			} else {
				p.CursorBlinked = (mode == "hidden" || mode == "hide")
			}
		case "input", "textarea":
			if r.Focus == n.ID {
				mode := str(n, "cursor_mode", "blink")
				if mode == "blink" {
					speed := time.Duration(num(n, "cursor_blink_speed", 530)) * time.Millisecond
					if speed <= 0 {
						speed = 530 * time.Millisecond
					}
					p.CursorElapsed += delta
					if p.CursorElapsed >= speed {
						if (p.CursorElapsed/speed)%2 != 0 {
							p.CursorBlinked = !p.CursorBlinked
						}
						p.CursorElapsed %= speed
					}
				} else {
					p.CursorBlinked = (mode == "hidden" || mode == "hide")
				}
			}
		case "spinner":
			if p.Running {
				interval := primitiveInterval(n)
				if interval <= 0 {
					interval = time.Millisecond
				}
				p.FrameElapsed += delta
				steps := p.FrameElapsed / interval
				p.FrameElapsed %= interval
				p.Frame = (p.Frame + int(steps%time.Duration(len(primitiveFrames(n))))) % len(primitiveFrames(n))
			}
		case "progress":
			p.ProgressElapsed += delta
			duration := time.Duration(num(n, "duration", 250)) * time.Millisecond
			if duration <= 0 || !primitiveBool(n, "animated", true) {
				p.Percent = p.Target
			} else {
				fraction := min(1, float64(p.ProgressElapsed)/float64(duration))
				p.Percent = p.From + (p.Target-p.From)*fraction
			}
		case "timer", "stopwatch":
			if !p.Running || p.TimedOut {
				continue
			}
			p.Elapsed += delta
			if n.Type == "timer" && p.Elapsed >= time.Duration(p.Duration*float64(time.Millisecond)) {
				p.Elapsed = time.Duration(p.Duration * float64(time.Millisecond))
				p.TimedOut = true
				if action := str(n, "on_timeout", ""); action != "" {
					if err := r.Action(action, map[string]any{"id": n.ID, "elapsed": float64(p.Elapsed / time.Millisecond)}); err != nil {
						return err
					}
				}
			}
		}
		if n.Type == "timer" {
			if clock != primitiveClock(n, time.Duration(p.Duration*float64(time.Millisecond))-p.Elapsed, true) {
				r.AnimationRevision++
			}
		} else if n.Type == "stopwatch" {
			if clock != primitiveClock(n, p.Elapsed, false) {
				r.AnimationRevision++
			}
		} else if frame != p.Frame || percent != p.Percent || blink != p.CursorBlinked {
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
			if p.Running && !p.TimedOut {
				return true
			}
		case "progress":
			p := r.primitive(n)
			if p.Percent != p.Target {
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
		if mode == "hidden" || mode == "hide" || (mode == "blink" && p.CursorBlinked) {
			put(screen, cr.X, cr.Y, " ", st, clip)
		} else {
			ch := str(n, "char", "█")
			put(screen, cr.X, cr.Y, ch, st.Reverse(true), clip)
		}
	case "spinner":
		a := primitiveFrames(n)
		put(screen, cr.X, cr.Y, a[p.Frame%len(a)], st, clip)
	case "progress":
		label := ""
		if primitiveBool(n, "show_percentage", true) {
			fmtStr := str(n, "percent_format", " %3.0f%%")
			label = fmt.Sprintf(fmtStr, p.Percent*100)
		}
		width := max(0, cr.W-uniseg.StringWidth(label))
		filled := int(math.Round(float64(width) * p.Percent))
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
			start := max(0, min(p.Page-width/2, pages-width))
			for x := 0; x < width; x++ {
				glyph := str(n, "inactive_dot", "○")
				if start+x == p.Page {
					glyph = str(n, "active_dot", "●")
				}
				put(screen, viewport.X+x, cr.Y, glyph, st, clip)
			}
		} else {
			put(screen, cr.X, cr.Y, fmt.Sprintf("%d/%d", p.Page+1, pages), st, clip)
		}
	case "timer":
		stClock := st
		if r.Focus == n.ID {
			stClock = stClock.Underline(true).Bold(true)
		}
		remaining := time.Duration(p.Duration*float64(time.Millisecond)) - p.Elapsed
		if remaining < 0 {
			remaining = 0
		}
		put(screen, cr.X, cr.Y, primitiveClock(n, remaining, true), stClock, clip)
	case "stopwatch":
		stClock := st
		if r.Focus == n.ID {
			stClock = stClock.Underline(true).Bold(true)
		}
		put(screen, cr.X, cr.Y, primitiveClock(n, p.Elapsed, false), stClock, clip)
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
			if p.Full {
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
		old := p.Page
		switch e.Key() {
		case tcell.KeyLeft, tcell.KeyUp, tcell.KeyPgUp:
			p.Page--
		case tcell.KeyRight, tcell.KeyDown, tcell.KeyPgDn:
			p.Page++
		case tcell.KeyHome:
			p.Page = 0
		case tcell.KeyEnd:
			p.Page = primitivePages(n) - 1
		case tcell.KeyRune:
			switch e.Rune() {
			case 'h', 'k':
				p.Page--
			case 'l', 'j':
				p.Page++
			default:
				return false, nil
			}
		default:
			return false, nil
		}
		p.Page = max(0, min(p.Page, primitivePages(n)-1))
		if p.Page == old {
			return true, nil
		}
		if action := str(n, "on_change", ""); action != "" {
			err := r.Action(action, map[string]any{"id": n.ID, "page": p.Page, "per_page": num(n, "per_page", 1), "total": num(n, "total", 0)})
			if err != nil {
				p.Page = old
			}
			return true, err
		}
		return true, nil
	case "timer", "stopwatch":
		p := r.primitive(n)
		old := *p
		switch {
		case e.Key() == tcell.KeyEnter || (e.Key() == tcell.KeyRune && e.Rune() == ' '):
			if p.TimedOut {
				p.Elapsed = 0
				p.TimedOut = false
				p.Running = true
			} else {
				p.Running = !p.Running
			}
			p.Last = time.Time{}
			p.Initialized = false
			p.RunningProp = p.Running
			if action := str(n, "on_change", ""); action != "" {
				if err := r.Action(action, map[string]any{"id": n.ID, "running": p.Running, "elapsed": float64(p.Elapsed / time.Millisecond)}); err != nil {
					*p = old
					return true, err
				}
			}
			return true, nil
		case e.Key() == tcell.KeyRune && (e.Rune() == 'r' || e.Rune() == 'R'):
			p.Elapsed = 0
			p.TimedOut = false
			p.Last = time.Time{}
			p.Initialized = false
			p.Running = primitiveBool(n, "running", true)
			p.RunningProp = p.Running
			if action := str(n, "on_change", ""); action != "" {
				if err := r.Action(action, map[string]any{"id": n.ID, "running": p.Running, "elapsed": float64(p.Elapsed / time.Millisecond)}); err != nil {
					*p = old
					return true, err
				}
			}
			return true, nil
		default:
			return false, nil
		}
	case "help":
		if e.Key() == tcell.KeyRune && e.Rune() == '?' {
			p := r.primitive(n)
			p.Full = !p.Full
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
