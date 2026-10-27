package engine

import (
	"math"
	"strconv"
	"strings"
)

type Rect struct{ X, Y, W, H int }

func intersect(a, b Rect) Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return Rect{x, y, max(0, min(a.X+a.W, b.X+b.W)-x), max(0, min(a.Y+a.H, b.Y+b.H)-y)}
}

func size(n *Node, key string, parent int, fallback float64) float64 {
	switch v := n.Props[key].(type) {
	case float64:
		return v
	case string:
		if strings.HasSuffix(v, "%") {
			f, _ := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
			return float64(parent) * f / 100
		}
	}
	return fallback
}

func bounded(n *Node, key string, v float64, parent int) float64 {
	return max(size(n, "min_"+key, parent, 0), min(size(n, "max_"+key, parent, 1e6), v))
}

func intrinsic(n *Node) (float64, float64) {
	if n.intrinsicValid {
		return n.intrinsicCache[0], n.intrinsicCache[1]
	}
	if flag(n, "hidden") {
		return 0, 0
	}
	pad := num(n, "padding", 0)
	if flag(n, "border") || n.Type == "panel" {
		pad++
	}
	text := measureText(n)
	if primitive := primitiveText(n); primitive != "" {
		text = primitive
	}
	w, h := 0, 0
	for line := range strings.SplitSeq(text, "\n") {
		w = max(w, textWidth(line))
		h++
	}
	switch n.Type {
	case "input", "button":
		h = 1
	case "textarea", "list", "scroll", "table", "viewport", "tree", "filepicker":
		h = 3
	}
	if len(n.Children) > 0 {
		w, h = 0, 0
		row := str(n, "direction", n.Type) == "row"
		count := 0
		for _, c := range n.Children {
			if flag(c, "hidden") || str(c, "position", "") == "absolute" {
				continue
			}
			cw, ch := intrinsic(c)
			cw = size(c, "width", 0, cw)
			ch = size(c, "height", 0, ch)
			m := num(c, "margin", 0) * 2
			if row {
				w += int(cw + m)
				h = max(h, int(ch+m))
			} else {
				h += int(ch + m)
				w = max(w, int(cw+m))
			}
			count++
		}
		if row {
			w += max(0, count-1) * int(num(n, "gap", 0))
		} else {
			h += max(0, count-1) * int(num(n, "gap", 0))
		}
	}
	n.intrinsicCache = [2]float64{float64(w) + 2*pad, float64(h) + 2*pad}
	n.intrinsicValid = true
	return n.intrinsicCache[0], n.intrinsicCache[1]
}

func Layout(root *Node, w, h int) error {
	invalidateMeasure(root)
	place(root, Rect{0, 0, max(0, w), max(0, h)}, Rect{0, 0, max(0, w), max(0, h)})
	return nil
}

func place(n *Node, r, clip Rect) {
	n.Rect = r
	n.Clip = intersect(r, clip)
	if flag(n, "hidden") {
		n.Rect.W = 0
		n.Rect.H = 0
		n.Clip = Rect{}
		return
	}
	if len(n.Children) == 0 {
		return
	}
	pad := int(num(n, "padding", 0))
	if flag(n, "border") || n.Type == "panel" {
		pad++
	}
	inner := Rect{r.X + pad, r.Y + pad, max(0, r.W-2*pad), max(0, r.H-2*pad)}
	childClip := intersect(inner, n.Clip)
	row := str(n, "direction", n.Type) == "row"
	main, cross := inner.H, inner.W
	if row {
		main, cross = inner.W, inner.H
	}
	axis, caxis := "height", "width"
	if row {
		axis, caxis = "width", "height"
	}
	children := []*Node{}
	for _, c := range n.Children {
		if flag(c, "hidden") {
			place(c, Rect{}, childClip)
			continue
		}
		if str(c, "position", "") != "absolute" {
			children = append(children, c)
		}
	}
	bases := make([]float64, len(children))
	vals := make([]float64, len(children))
	margins := make([]int, len(children))
	gap := num(n, "gap", 0)
	used := gap * float64(max(0, len(children)-1))
	for i, c := range children {
		iw, ih := intrinsic(c)
		_, explicitHeight := c.Props["height"]
		if !row && !explicitHeight && num(c, "grow", 0) == 0 {
			available := cross - int(num(c, "margin", 0))*2
			available = int(size(c, "width", cross, float64(available)))
			ih = measuredHeight(c, max(0, available))
		}
		b := ih
		if row {
			b = iw
		}
		if _, ok := c.Props[axis]; !ok && num(c, "grow", 0) > 0 {
			b = 0
		}
		b = size(c, axis, main, b)
		b = size(c, "basis", main, b)
		bases[i] = b
		vals[i] = bounded(c, axis, b, main)
		margins[i] = int(num(c, "margin", 0))
		used += vals[i] + float64(margins[i]*2)
	}
	remaining := float64(main) - used
	weights := make([]float64, len(children))
	for pass := 0; pass <= len(children) && math.Abs(remaining) > 0.001; pass++ {
		weight := 0.0
		clear(weights)
		for i, c := range children {
			if remaining > 0 && vals[i] < size(c, "max_"+axis, main, 1e6) {
				weights[i] = num(c, "grow", 0)
			} else if remaining < 0 && vals[i] > size(c, "min_"+axis, main, 0) {
				weights[i] = num(c, "shrink", 1) * bases[i]
			}
			weight += weights[i]
		}
		if weight == 0 {
			break
		}
		delta := 0.0
		for i, c := range children {
			nv := bounded(c, axis, vals[i]+remaining*weights[i]/weight, main)
			delta += nv - vals[i]
			vals[i] = nv
		}
		remaining -= delta
		if math.Abs(delta) < 0.001 {
			break
		}
	}
	used = gap * float64(max(0, len(children)-1))
	for i, v := range vals {
		used += v + float64(margins[i]*2)
	}
	free := max(0, float64(main)-used)
	offset := 0.0
	switch str(n, "justify", "start") {
	case "center":
		offset = free / 2
	case "end":
		offset = free
	case "space-between":
		if len(children) > 1 {
			gap += free / float64(len(children)-1)
		}
	case "space-around":
		if len(children) > 0 {
			gap += free / float64(len(children))
			offset = free / float64(len(children)) / 2
		}
	}
	cursor := offset
	for i, c := range children {
		m := margins[i]
		cursor += float64(m)
		start := int(math.Round(cursor))
		end := int(math.Round(cursor + vals[i]))
		iw, ih := intrinsic(c)
		cv := iw
		if row {
			cv = ih
		}
		align := str(c, "align_self", str(n, "align", "stretch"))
		if _, ok := c.Props[caxis]; !ok && align == "stretch" {
			cv = float64(max(0, cross-2*m))
		}
		cv = bounded(c, caxis, size(c, caxis, cross, cv), cross)
		cs := int(math.Round(cv))
		off := m
		switch align {
		case "center":
			off = (cross - cs) / 2
		case "end":
			off = cross - cs - m
		}
		cr := Rect{inner.X + off, inner.Y + start, cs, max(0, end-start)}
		if row {
			cr = Rect{inner.X + start, inner.Y + off, max(0, end-start), cs}
		}
		place(c, cr, childClip)
		cursor += vals[i] + float64(m) + gap
	}
	for _, c := range n.Children {
		if flag(c, "hidden") || str(c, "position", "") != "absolute" {
			continue
		}
		iw, ih := intrinsic(c)
		cw, ch := size(c, "width", inner.W, iw), size(c, "height", inner.H, ih)
		left, right := size(c, "left", inner.W, 0), size(c, "right", inner.W, 0)
		top, bottom := size(c, "top", inner.H, 0), size(c, "bottom", inner.H, 0)
		_, l := c.Props["left"]
		_, rr := c.Props["right"]
		_, t := c.Props["top"]
		_, b := c.Props["bottom"]
		if _, ok := c.Props["width"]; !ok && l && rr {
			cw = max(0, float64(inner.W)-left-right)
		}
		if _, ok := c.Props["height"]; !ok && t && b {
			ch = max(0, float64(inner.H)-top-bottom)
		}
		cw = bounded(c, "width", cw, inner.W)
		ch = bounded(c, "height", ch, inner.H)
		if !l && rr {
			left = float64(inner.W) - right - cw
		}
		if !t && b {
			top = float64(inner.H) - bottom - ch
		}
		place(c, Rect{inner.X + int(math.Round(left)), inner.Y + int(math.Round(top)), int(math.Round(cw)), int(math.Round(ch))}, childClip)
	}
}

func measureText(n *Node) string {
	if n.layoutBound {
		if _, spans := n.Props["spans"].([]any); !spans {
			return n.layoutText
		}
	}
	return plainText(n)
}

func invalidateMeasure(n *Node) {
	n.intrinsicValid = false
	n.measuredValid = false
	for _, c := range n.Children {
		invalidateMeasure(c)
	}
}

func measuredHeight(n *Node, width int) (heightResult float64) {
	if n.measuredValid && n.measuredWidth == width {
		return n.measuredHeightCache
	}
	defer func() { n.measuredValid = true; n.measuredWidth = width; n.measuredHeightCache = heightResult }()
	_, h := intrinsic(n)
	if flag(n, "hidden") {
		return 0
	}
	pad := int(num(n, "padding", 0))
	if flag(n, "border") || n.Type == "panel" {
		pad++
	}
	inner := max(0, width-pad*2)
	if n.Type == "text" && str(n, "wrap", "") != "none" && inner > 0 {
		return float64(len(Lines(measureText(n), inner, true)) + pad*2)
	}
	if len(n.Children) == 0 {
		return h
	}
	row := str(n, "direction", n.Type) == "row"
	height := 0.0
	count := 0
	for _, c := range n.Children {
		if flag(c, "hidden") || str(c, "position", "") == "absolute" {
			continue
		}
		m := num(c, "margin", 0)
		cw := size(c, "width", inner, float64(inner)-2*m)
		ch := size(c, "height", 0, measuredHeight(c, max(0, int(cw))))
		if row {
			height = max(height, ch+2*m)
		} else {
			height += ch + 2*m
		}
		count++
	}
	if !row {
		height += float64(max(0, count-1)) * num(n, "gap", 0)
	}
	return height + float64(pad*2)
}
