package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (r *Runtime) keyCollection(n *Node, e *tcell.EventKey) (bool, error) {
	if !isCollection(n) {
		return false, nil
	}
	if flag(n, "disabled") {
		return true, nil
	}
	s, c := r.collectionState(n)
	items := r.collectionItems(n, c)
	body := collectionBody(n, contentRect(n))
	page := min(collectionPageSize(n, body), max(1, body.H/collectionStride(n, items)))
	if c.Editing && (n.Type == "list" || n.Type == "filepicker") {
		old := c.Query
		switch e.Key() {
		case tcell.KeyEscape, tcell.KeyEnter:
			c.Editing = false
		case tcell.KeyCtrlU:
			c.Query = ""
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			g := graphemes(c.Query)
			if len(g) > 0 {
				c.Query = strings.Join(g[:len(g)-1], "")
			}
		case tcell.KeyRune:
			if len(c.Query)+len(string(e.Rune())) > maxText {
				return true, fmt.Errorf("query exceeds 1 MiB")
			}
			c.Query += string(e.Rune())
		default:
			return true, nil
		}
		if old != c.Query {
			oldSelected, oldScroll := s.Selected, s.Scroll
			s.Selected = 0
			s.Scroll = 0
			if changeAction := str(n, "on_change", ""); changeAction != "" {
				err := r.Action(changeAction, map[string]any{"id": n.ID, "value": c.Query, "query": c.Query})
				if err != nil {
					c.Query = old
					s.Selected = oldSelected
					s.Scroll = oldScroll
				}
				return true, err
			}
		}
		return true, nil
	}
	if (n.Type == "list" || n.Type == "filepicker") && e.Key() == tcell.KeyRune && e.Rune() == '/' {
		c.Editing = true
		return true, nil
	}
	if n.Type == "viewport" {
		lines := viewportLines(r.text(n), body.W, flag(n, "soft_wrap"))
		switch e.Key() {
		case tcell.KeyUp:
			s.Scroll--
		case tcell.KeyDown:
			s.Scroll++
		case tcell.KeyPgUp:
			s.Scroll -= page
		case tcell.KeyPgDn:
			s.Scroll += page
		case tcell.KeyHome:
			s.Scroll = 0
			c.X = 0
		case tcell.KeyEnd:
			s.Scroll = len(lines) - body.H
		case tcell.KeyLeft:
			c.X = max(0, c.X-1)
		case tcell.KeyRight:
			if !flag(n, "soft_wrap") {
				c.X++
			}
		case tcell.KeyCtrlD:
			s.Scroll += max(1, page/2)
		case tcell.KeyCtrlU:
			s.Scroll -= max(1, page/2)
		case tcell.KeyRune:
			switch e.Rune() {
			case 'k':
				s.Scroll--
			case 'j':
				s.Scroll++
			case 'h':
				c.X = max(0, c.X-1)
			case 'l':
				if !flag(n, "soft_wrap") {
					c.X++
				}
			case 'g':
				s.Scroll = 0
				c.X = 0
			case 'G':
				s.Scroll = len(lines) - body.H
			case 'd':
				s.Scroll += max(1, page/2)
			case 'u':
				s.Scroll -= max(1, page/2)
			default:
				return false, nil
			}
		default:
			return false, nil
		}
		s.Scroll = max(0, min(s.Scroll, max(0, len(lines)-body.H)))
		return true, nil
	}
	collectionClamp(s, len(items))
	oldSelected := s.Selected
	switch e.Key() {
	case tcell.KeyUp:
		s.Selected--
	case tcell.KeyDown:
		s.Selected++
	case tcell.KeyHome:
		s.Selected = 0
	case tcell.KeyEnd:
		s.Selected = len(items) - 1
	case tcell.KeyPgUp:
		s.Selected -= page
	case tcell.KeyPgDn:
		s.Selected += page
	case tcell.KeyCtrlD:
		s.Selected += page
	case tcell.KeyCtrlU:
		s.Selected -= page
	case tcell.KeyRune:
		switch e.Rune() {
		case 'k':
			s.Selected--
		case 'j':
			s.Selected++
		case 'g':
			s.Selected = 0
		case 'G':
			s.Selected = len(items) - 1
		case 'h':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch && collectionExpanded(c, it) {
					c.Expanded[it.id] = false
					return true, nil
				}
			}
			return false, nil
		case 'l':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch && !collectionExpanded(c, it) {
					c.Expanded[it.id] = true
					return true, nil
				}
			}
			return false, nil
		case ' ':
			if n.Type == "tree" && len(items) > 0 {
				it := items[s.Selected]
				if it.branch {
					c.Expanded[it.id] = !collectionExpanded(c, it)
					return true, nil
				}
			}
			return false, nil
		default:
			return false, nil
		}
	case tcell.KeyEnter:
		if n.Type == "filepicker" && e.Modifiers()&tcell.ModCtrl != 0 && len(items) > 0 {
			it := items[s.Selected]
			if collectionFileAllowed(n, it) {
				return true, r.Action(str(n, "on_select", ""), map[string]any{"id": n.ID, "index": it.index, "value": it.value})
			}
			return true, nil
		}
		return true, r.collectionActivate(n, s, c, items)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if n.Type == "filepicker" {
			return true, r.Action(str(n, "on_navigate", ""), map[string]any{"id": n.ID, "path": filepath.Dir(str(n, "path", "."))})
		}
	case tcell.KeyLeft, tcell.KeyRight:
		if n.Type == "table" {
			if e.Key() == tcell.KeyRight {
				c.X++
			} else {
				c.X = max(0, c.X-1)
			}
		}
		if n.Type == "tree" && len(items) > 0 {
			it := items[s.Selected]
			expanded := collectionExpanded(c, it)
			if e.Key() == tcell.KeyRight {
				if it.branch {
					if expanded {
						if s.Selected+1 < len(items) {
							s.Selected++
						}
					} else {
						c.Expanded[it.id] = true
					}
				}
			} else {
				if it.branch && expanded {
					c.Expanded[it.id] = false
				} else {
					for i := s.Selected - 1; i >= 0; i-- {
						if items[i].depth < it.depth {
							s.Selected = i
							break
						}
					}
				}
			}
		}
	}
	items = r.collectionItems(n, c)
	collectionClamp(s, len(items))
	if s.Selected != oldSelected {
		if changeAction := str(n, "on_change", ""); changeAction != "" {
			if s.Selected >= 0 && s.Selected < len(items) {
				it := items[s.Selected]
				err := r.Action(changeAction, map[string]any{"id": n.ID, "index": it.index, "value": it.value})
				if err != nil {
					s.Selected = oldSelected
				}
				return true, err
			}
		}
	}
	return true, nil
}

func (r *Runtime) mouseCollection(n *Node, e *tcell.EventMouse) (bool, error) {
	if !isCollection(n) {
		return false, nil
	}
	if flag(n, "disabled") {
		return true, nil
	}
	s, c := r.collectionState(n)
	x, y := e.Position()
	body := collectionBody(n, contentRect(n))
	buttons := e.Buttons()
	if buttons&tcell.WheelUp != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyUp, 0, 0))
	}
	if buttons&tcell.WheelDown != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyDown, 0, 0))
	}
	if buttons&tcell.WheelLeft != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	}
	if buttons&tcell.WheelRight != 0 {
		return r.keyCollection(n, tcell.NewEventKey(tcell.KeyRight, 0, 0))
	}
	if buttons&tcell.Button1 == 0 {
		return true, nil
	}
	if x < body.X || x >= body.X+body.W || y < body.Y || y >= body.Y+body.H {
		return true, nil
	}
	if n.Type == "viewport" {
		return true, nil
	}
	items := r.collectionItems(n, c)
	index := s.Scroll + (y-body.Y)/collectionStride(n, items)
	if index >= len(items) {
		return true, nil
	}
	oldSelected := s.Selected
	s.Selected = index
	if n.Type == "tree" {
		it := items[index]
		if it.branch && x < body.X+it.depth*2+2 {
			c.Expanded[it.id] = !collectionExpanded(c, it)
			return true, nil
		}
	}
	if s.Selected != oldSelected {
		if changeAction := str(n, "on_change", ""); changeAction != "" {
			if s.Selected >= 0 && s.Selected < len(items) {
				it := items[s.Selected]
				if err := r.Action(changeAction, map[string]any{"id": n.ID, "index": it.index, "value": it.value}); err != nil {
					s.Selected = oldSelected
					return true, err
				}
			}
		}
	}
	return true, r.collectionActivate(n, s, c, items)
}
