package core

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRectSidesAndBorderSides(t *testing.T) {
	a := Rect{X: 2, Y: 3, W: 10, H: 6}
	b := Rect{X: 5, Y: 4, W: 12, H: 3}
	if got, want := a.Intersect(b), (Rect{X: 5, Y: 4, W: 7, H: 3}); got != want {
		t.Fatalf("Intersect = %+v, want %+v", got, want)
	}
	if got := a.Intersect(Rect{X: 50, Y: 50, W: 2, H: 2}); got.W != 0 || got.H != 0 {
		t.Fatalf("disjoint Intersect should have zero size, got %+v", got)
	}

	s := Sides{Top: 1, Right: 2, Bottom: 3, Left: 4}
	if s.Horizontal() != 6 || s.Vertical() != 4 {
		t.Fatalf("Sides totals wrong: h=%d v=%d", s.Horizontal(), s.Vertical())
	}
	if got := s.Add(Sides{Top: 1, Right: 1, Bottom: 1, Left: 1}); got != (Sides{2, 3, 4, 5}) {
		t.Fatalf("Sides.Add = %+v", got)
	}

	var empty BorderSides
	if empty.Any() || empty.All() || empty.Insets() != (Sides{}) {
		t.Fatalf("empty BorderSides unexpected: %+v", empty)
	}
	partial := BorderSides{Top: true, Left: true}
	if !partial.Any() || partial.All() || partial.Insets() != (Sides{Top: 1, Left: 1}) {
		t.Fatalf("partial BorderSides unexpected: %+v", partial)
	}
	full := BorderSides{Top: true, Right: true, Bottom: true, Left: true}
	if !full.Any() || !full.All() || full.Insets() != (Sides{1, 1, 1, 1}) {
		t.Fatalf("full BorderSides unexpected: %+v", full)
	}
}

func TestWidgetCollectionAndPrimitiveState(t *testing.T) {
	if CloneCollectionState(nil) != nil {
		t.Fatal("CloneCollectionState(nil) should be nil")
	}
	cs := &CollectionState{Query: "abc", Editing: true, X: 3, Expanded: map[string]bool{"root": true}}
	csCopy := CloneCollectionState(cs)
	csCopy.Expanded["root"] = false
	if !cs.Expanded["root"] {
		t.Fatal("CloneCollectionState aliased Expanded map")
	}

	if ClonePrimitiveState(nil) != nil {
		t.Fatal("ClonePrimitiveState(nil) should be nil")
	}
	ps := &PrimitiveState{Initialized: true, Page: 2, Elapsed: time.Second}
	psCopy := ClonePrimitiveState(ps)
	psCopy.Page = 9
	if ps.Page != 2 {
		t.Fatal("ClonePrimitiveState mutated source")
	}

	ws := &WidgetState{Value: "ab\ncd"}
	g, pos := ws.EditorIndex(2, true)
	if !slices.Equal(g, []string{"a", "b", "\n", "c", "d"}) || len(pos) != 6 || pos[3] != [2]int{0, 1} {
		t.Fatalf("EditorIndex multiline mismatch: g=%q pos=%v", g, pos)
	}
	_, posFlat := ws.EditorIndex(2, false)
	if posFlat[3] != [2]int{2, 0} || posFlat[5] != [2]int{4, 0} {
		t.Fatalf("EditorIndex single-line mismatch: pos=%v", posFlat)
	}
}

func TestBorderGlyphsAndThemePresets(t *testing.T) {
	borders := []BorderGlyphs{SingleBorder, RoundedBorder, DoubleBorder, ThickBorder, ASCIIBorder}
	for i, bg := range borders {
		if bg.Top == 0 || bg.Bottom == 0 || bg.Left == 0 || bg.Right == 0 ||
			bg.TopLeft == 0 || bg.TopRight == 0 || bg.BottomLeft == 0 || bg.BottomRight == 0 {
			t.Fatalf("border preset %d has zero rune: %+v", i, bg)
		}
	}

	themes := []ThemePreset{
		DefaultTheme(),
		TokyoNightTheme(),
		CatppuccinTheme(),
		DraculaTheme(),
		NordTheme(),
	}
	for i, th := range themes {
		if th.Foreground == "" || th.Background == "" || th.FocusForeground == "" {
			t.Fatalf("theme preset %d has empty color: %+v", i, th)
		}
	}
}

func TestEstimateBytesValidateModelAndCloneCopyValue(t *testing.T) {
	samples := []any{
		nil,
		true,
		false,
		0,
		-42,
		int64(123456),
		3.5,
		"hello\n\"world\"\\",
		[]string{"a", "b"},
		[]any{1, "two", map[string]any{"ok": true}},
		map[string]any(nil),
	}
	for _, sample := range samples {
		got, err := EstimateBytes(sample)
		if err != nil {
			t.Fatalf("EstimateBytes(%#v) err = %v", sample, err)
		}
		raw, err := json.Marshal(sample)
		if err != nil {
			t.Fatalf("json.Marshal(%#v) err = %v", sample, err)
		}
		if got != len(raw) {
			t.Fatalf("EstimateBytes(%#v) = %d, want %d", sample, got, len(raw))
		}
	}

	if _, err := EstimateBytes(math.NaN()); err == nil {
		t.Fatal("expected NaN error from EstimateBytes")
	}
	if _, err := EstimateBytes(make(chan int)); err == nil {
		t.Fatal("expected unsupported type error from EstimateBytes")
	}
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if _, err := EstimateBytes(cyclic); err == nil {
		t.Fatal("expected cycle/depth error from EstimateBytes")
	}

	if err := ValidateModel(map[string]any{"k": "v"}); err != nil {
		t.Fatalf("ValidateModel valid failed: %v", err)
	}
	tooMany := make(map[string]any, MaxModelKeys+1)
	for i := 0; i <= MaxModelKeys; i++ {
		tooMany[string(rune('a'+i%26))+strings.Repeat("x", i/26)] = true
	}
	if err := ValidateModel(tooMany); err == nil || !strings.Contains(err.Error(), "1024") {
		t.Fatalf("expected 1024 keys error, got %v", err)
	}
	if err := ValidateModel(map[string]any{"big": strings.Repeat("a", MaxText)}); err == nil {
		t.Fatal("expected size limit error")
	}

	orig := map[string]any{
		"slice": []any{"x", map[string]any{"y": 1}},
		"strs":  []string{"a", "b"},
		"bg":    RoundedBorder,
	}
	copied := CopyValue(orig).(map[string]any)
	copied["strs"].([]string)[0] = "z"
	copied["slice"].([]any)[1].(map[string]any)["y"] = 99
	if orig["strs"].([]string)[0] != "a" || orig["slice"].([]any)[1].(map[string]any)["y"] != 1 {
		t.Fatal("CopyValue mutated original")
	}

	if _, err := CloneValue(make(chan int)); err == nil || !strings.Contains(err.Error(), "unsupported Go type") {
		t.Fatalf("expected unsupported Go type error, got %v", err)
	}
	if CopyValue(make(chan int)) != nil {
		t.Fatal("CopyValue should return nil on error")
	}
	deep := any("leaf")
	for i := 0; i < 34; i++ {
		deep = []any{deep}
	}
	if _, err := CloneValue(deep); err == nil || !strings.Contains(err.Error(), "32") {
		t.Fatalf("expected depth limit error, got %v", err)
	}
}

func TestPropSidesAndNodeBorderSides(t *testing.T) {
	if got := PropSides(nil, "padding"); got != (Sides{}) {
		t.Fatalf("nil PropSides = %+v", got)
	}
	if got := PropSides(&Node{Props: map[string]any{"padding": -5}}, "padding"); got != (Sides{}) {
		t.Fatalf("negative scalar PropSides = %+v", got)
	}
	if got := PropSides(&Node{Props: map[string]any{"padding": 2}}, "padding"); got != (Sides{2, 2, 2, 2}) {
		t.Fatalf("scalar PropSides = %+v", got)
	}
	if got := PropSides(&Node{Props: map[string]any{"padding": []int{1, 3}}}, "padding"); got != (Sides{1, 3, 1, 3}) {
		t.Fatalf("2-elem PropSides = %+v", got)
	}
	if got := PropSides(&Node{Props: map[string]any{"margin": []any{1, 2, 3, 4}}}, "margin"); got != (Sides{1, 2, 3, 4}) {
		t.Fatalf("4-elem PropSides = %+v", got)
	}
	n := &Node{Props: map[string]any{
		"padding":      map[string]any{"x": 4, "y": 2, "top": 1},
		"padding_x":    5,
		"padding_left": 9,
	}}
	if got := PropSides(n, "padding"); got != (Sides{Top: 1, Right: 5, Bottom: 2, Left: 9}) {
		t.Fatalf("override PropSides = %+v", got)
	}

	if got := NodeBorderSides(nil); got != (BorderSides{}) {
		t.Fatalf("nil NodeBorderSides = %+v", got)
	}
	if got := NodeBorderSides(&Node{Type: "panel"}); !got.All() {
		t.Fatalf("panel default borders = %+v", got)
	}
	if got := NodeBorderSides(&Node{Type: "panel", Props: map[string]any{"border": false}}); got.Any() {
		t.Fatalf("panel border=false = %+v", got)
	}
	if got := NodeBorderSides(&Node{Type: "box", Props: map[string]any{"border": "rounded", "border_bottom": false}}); got != (BorderSides{Top: true, Right: true, Bottom: false, Left: true}) {
		t.Fatalf("rounded with bottom=false = %+v", got)
	}
	if got := NodeBorderSides(&Node{Type: "box", Props: map[string]any{"border": "none"}}); got.Any() {
		t.Fatalf("border=none = %+v", got)
	}
	if got := NodeBorderSides(&Node{Type: "box", Props: map[string]any{"border_sides": map[string]any{"top": true, "left": true}}}); got != (BorderSides{Top: true, Left: true}) {
		t.Fatalf("border_sides map = %+v", got)
	}
}

func TestTraversalAutoAssignAndValidateTree(t *testing.T) {
	if got := slices.Collect(Ordered(nil)); len(got) != 0 {
		t.Fatalf("Ordered(nil) = %v", got)
	}
	if got := slices.Collect(VisibleNodes(nil)); len(got) != 0 {
		t.Fatalf("VisibleNodes(nil) = %v", got)
	}
	if got := SortedChildren(nil); got != nil {
		t.Fatalf("SortedChildren(nil) = %v", got)
	}

	a := (&Node{Type: "box"}).SetID("a").ZIndex(2)
	b := (&Node{Type: "box"}).SetID("b")
	c := (&Node{Type: "box", Children: []*Node{{Type: "text", ID: "hid"}}}).SetID("c").Hidden(true)
	d := (&Node{Type: "box"}).SetID("d")
	root := (&Node{Type: "column"}).SetID("root").Add(a, b, c, d)

	if got := slices.Collect(Ordered(root)); !slices.Equal(got, []*Node{root, a, b, c, c.Children[0], d}) {
		t.Fatalf("Ordered mismatch: %v", got)
	}
	if got := slices.Collect(VisibleNodes(root)); !slices.Equal(got, []*Node{root, b, d, a}) {
		t.Fatalf("VisibleNodes mismatch: %v", got)
	}
	if got := SortedChildren(root); &got[0] == &root.Children[0] {
		t.Fatal("unsorted children should be cloned")
	}

	sortedRoot := &Node{Children: []*Node{b, d}}
	if got := SortedChildren(sortedRoot); &got[0] != &sortedRoot.Children[0] {
		t.Fatal("already sorted children should reuse slice")
	}

	autoRoot := (&Node{Type: "column"}).Add(
		(&Node{Type: "input"}).Bind("name"),
		(&Node{Type: "button"}).OnClick("save"),
		&Node{Type: "text"},
		(&Node{Type: "button"}).OnClick("save"),
	)
	AutoAssignIDs(autoRoot)
	wantIDs := []string{"root.input_name", "root.button_save", "root.text_2", "root.button_save_2"}
	for i, want := range wantIDs {
		if autoRoot.Children[i].ID != want {
			t.Fatalf("child %d ID = %q, want %q", i, autoRoot.Children[i].ID, want)
		}
	}
	if FindNode(autoRoot, "root.button_save_2") != autoRoot.Children[3] {
		t.Fatal("FindNode failed")
	}
	cloned := CloneNode(autoRoot)
	if cloned == autoRoot || cloned.Children[0] == autoRoot.Children[0] {
		t.Fatal("CloneNode aliased nodes")
	}

	ids, err := ValidateTree(autoRoot)
	if err != nil || len(ids) != 5 {
		t.Fatalf("ValidateTree failed: ids=%d err=%v", len(ids), err)
	}
	if _, err := ValidateTree(&Node{Type: "nope"}); err == nil {
		t.Fatal("expected unknown component error")
	}
	if _, err := ValidateTree(&Node{Type: "input"}); err == nil {
		t.Fatal("expected missing id error for input")
	}
	dup := (&Node{Type: "column"}).SetID("dup").Add((&Node{Type: "text"}).SetID("dup"))
	if _, err := ValidateTree(dup); err == nil {
		t.Fatal("expected duplicate id error")
	}
	if _, err := ValidateTree(&Node{Type: "text", ID: strings.Repeat("x", MaxIDBytes+1)}); err == nil {
		t.Fatal("expected id length error")
	}
	hookErr := errors.New("custom check")
	_, err = ValidateTree(&Node{Type: "text", ID: "t"}, func(*Node) error { return hookErr })
	var ne *NodeError
	if !errors.As(err, &ne) || !errors.Is(err, hookErr) {
		t.Fatalf("expected wrapped NodeError, got %T %v", err, err)
	}
}

func TestValidateTreePropsAndNodeBuilders(t *testing.T) {
	valid := (&Node{Type: "column"}).SetID("root").
		Prop("direction", "column").
		Align("center").
		Justify("space-between").
		Padding(1).
		Margin(1).
		Gap(1).
		Border(BorderSides{Top: true, Bottom: true}).
		Prop("border_sides", BorderSides{Top: true, Bottom: true}).
		Add(
			(&Node{Type: "text"}).SetID("t1").
				Text("hi").
				FG("#ffffff").
				BG("#000000").
				Bold(true).
				Wrap(true).
				Prop("align_self", "stretch").
				ZIndex(1).
				Prop("width", "50%").
				Prop("height", "auto").
				Prop("padding", Sides{Top: 1, Left: 2}).
				Prop("margin", []int{1, 2}).
				Prop("border_glyphs", ThickBorder).
				Prop("border_top", true).
				Prop("spans", []any{map[string]any{"text": "hi", "fg": "red", "bg": "black", "bold": true}}).
				Nav("u", "d", "l", "r").
				Keymap(map[string]string{"enter": "go"}),
			(&Node{Type: "button"}).SetID("b1").
				Plain().
				Prefix("[").
				Suffix("]").
				Gradient("red", "blue").
				OnClickDo(func() {}).
				KeyDo("x", func() {}),
			(&Node{Type: "list"}).SetID("l1").
				Items("a", "b").
				SelectedFG("white").
				SelectedBG("blue").
				SelectedBold(true).
				Filterable(true),
			(&Node{Type: "paginator"}).SetID("p1").Page(1, 0, 10),
		)
	if _, err := ValidateTree(valid); err != nil {
		t.Fatalf("ValidateTree(valid) failed: %v", err)
	}

	invalidCases := []*Node{
		nil,
		{Type: "text", ID: "bad_pct", Props: map[string]any{"width": "20000%"}},
		{Type: "text", ID: "bad_dir", Props: map[string]any{"direction": "diag"}},
		{Type: "text", ID: "bad_align", Props: map[string]any{"align": "middle"}},
		{Type: "text", ID: "bad_just", Props: map[string]any{"justify": "around"}},
		{Type: "text", ID: "bad_pos", Props: map[string]any{"position": "fixed"}},
		{Type: "text", ID: "bad_border", Props: map[string]any{"border": "wavy"}},
		{Type: "text", ID: "bad_bsides", Props: map[string]any{"border_sides": "all"}},
		{Type: "text", ID: "bad_bglyph", Props: map[string]any{"border_glyphs": "single"}},
		{Type: "text", ID: "bad_bool", Props: map[string]any{"bold": "yes"}},
		{Type: "text", ID: "bad_nav", Props: map[string]any{"nav": map[string]any{"north": "x"}}},
		{Type: "text", ID: "bad_km", Props: map[string]any{"keymap": map[string]any{"a": 1}}},
		{Type: "text", ID: "bad_z", Props: map[string]any{"z_index": math.NaN()}},
		{Type: "text", ID: "bad_spans", Props: map[string]any{"spans": []any{map[string]any{"unknown": 1}}}},
		{Type: "text", ID: "bad_items", Props: map[string]any{"items": "not-slice"}},
		{Type: "text", ID: "bad_pad", Props: map[string]any{"padding": Sides{Top: -1}}},
		{Type: "text", ID: "bad_prop", Props: map[string]any{"unknown_prop": 1}},
	}
	for i, n := range invalidCases {
		if _, err := ValidateTree(n); err == nil {
			t.Fatalf("case %d unexpectedly succeeded: %+v", i, n)
		}
	}
}

func TestPropSidesExtendedAndNil(t *testing.T) {
	if got := PropSides(nil, "padding"); got != (Sides{}) {
		t.Fatalf("nil node sides = %+v", got)
	}
	n := &Node{
		Type: "box",
		Props: map[string]any{
			"p_f64":   []float64{1, 2, 3},
			"p_empty": []int{},
		},
	}
	if got := PropSides(n, "p_f64"); got != (Sides{Top: 1, Right: 2, Bottom: 3, Left: 2}) {
		t.Fatalf("[]float64 3-elem sides = %+v", got)
	}
	if got := PropSides(n, "p_empty"); got != (Sides{}) {
		t.Fatalf("empty sides = %+v", got)
	}
}

func TestEstimateBytesAndValidateModelExtended(t *testing.T) {
	n, err := EstimateBytes(map[string]any{
		"f32":    float32(1.5),
		"glyphs": SingleBorder,
		"empty":  []string(nil),
	})
	if err != nil || n <= 0 {
		t.Fatalf("EstimateBytes failed: n=%d err=%v", n, err)
	}
	if _, err := EstimateBytes(float32(math.NaN())); err == nil {
		t.Fatal("expected NaN float32 error")
	}

	hugeSlice := make([]string, 2048)
	for i := range hugeSlice {
		hugeSlice[i] = strings.Repeat("a", 1024)
	}
	if err := ValidateModel(map[string]any{"big": hugeSlice}); err == nil {
		t.Fatal("expected ValidateModel error for >1MiB []string")
	}

	manyKeys := make(map[string]any, MaxModelKeys+1)
	for i := range MaxModelKeys + 1 {
		manyKeys[string(rune('a'+i%26))+strings.Repeat("x", 4)+json.Number(string(rune('0'+i%10))).String()+string(rune(i))] = i
	}
	if len(manyKeys) > MaxModelKeys {
		if err := ValidateModel(manyKeys); err == nil {
			t.Fatal("expected ValidateModel error for >1024 keys")
		}
	}
}

func TestNodeBuildersExtended(t *testing.T) {
	n := (&Node{Type: "box"}).
		Bind("m.val").
		Source("m.items").
		Textf("hi %d", 1).
		Title("hdr").
		Placeholder("ph").
		Width(20).
		Height(10).
		Size(20, 10).
		MinWidth(2).
		MaxWidth(20).
		MinHeight(1).
		MaxHeight(10).
		Grow(1).
		Shrink(1).
		Flex(2).
		Padding(1).
		Margin(1).
		Gap(1).
		Border("rounded").
		BorderStyle("double").
		BorderFG("#fff").
		BorderBG("#000").
		BorderColor("#fff", "#000").
		BorderGlyphs(RoundedBorder).
		TitleFG("#aaa").
		ButtonStyle("filled").
		Plain().
		Filled().
		Chevrons().
		Parens().
		CursorPrefix("> ").
		Focusable(true).
		FG("#111").
		BG("#222").
		Color("#111", "#222").
		Bold(true).
		Align("center").
		Justify("between").
		Hidden(false).
		Disabled(true).
		Modal(true).
		Backdrop("dim").
		CloseOnEsc(true).
		ZIndex(3).
		Top(1).
		Right(2).
		Bottom(3).
		Left(4).
		Wrap(true).
		WrapMode("word").
		Absolute(1, 2).
		Inset(1, 2, 3, 4).
		Echo("password").
		Masked(true).
		CharLimit(32).
		MaxLength(64).
		ValidationPattern("^[a-z]+$").
		MultiSelect(true).
		ItemHeight(2).
		ShowIcons(true).
		ShowHidden(false).
		CurrentDir("/tmp").
		NavUp("u").
		NavDown("d").
		NavLeft("l").
		NavRight("r").
		Nav("u", "d", "l", "r").
		OnClick("act").
		OnChange("chg").
		OnSubmit("sub").
		OnSelect("sel").
		OnNavigate("nav").
		OnTimeout("to").
		OnKey("k").
		OnChangeText(func(string) {}).
		OnSubmitText(func(string) {}).
		OnSelectItem(func(int, any) {}).
		OnTimeoutDo(func() {}).
		Key("q", "quit").
		Keymap(map[string]string{"x": "y"}).
		Selected(1).
		Running(true).
		Percent(0.5).
		ShowPercentage(true).
		ShowHeader(true).
		ShowLineNumbers(true).
		SoftWrap(true).
		Mode("spinner").
		Preset("dots").
		Duration(250).
		Interval(100)

	if len(n.Props) == 0 {
		t.Fatal("expected populated props")
	}
	if _, ok := n.Props["timer_id"]; !ok {
		_ = n
	}
	_ = time.Millisecond
}

func BenchmarkEstimateBytes(b *testing.B) {
	data := map[string]any{
		"title": "Dashboard",
		"items": []string{"alpha", "beta", "gamma", "delta"},
		"meta":  map[string]any{"page": 1, "ok": true, "ratio": 0.75},
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := EstimateBytes(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateModelLargeReject(b *testing.B) {
	big := make([]string, 4096)
	for i := range big {
		big[i] = strings.Repeat("x", 1024)
	}
	data := map[string]any{"big": big}
	b.ReportAllocs()
	for b.Loop() {
		if err := ValidateModel(data); err == nil {
			b.Fatal("expected size limit error")
		}
	}
}

func BenchmarkPropSides(b *testing.B) {
	n := &Node{
		Type: "box",
		Props: map[string]any{
			"padding": []int{1, 2, 3, 4},
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = PropSides(n, "padding")
	}
}

func TestBuilderPropsValidationDeepCloneAndBoundedIDs(t *testing.T) {
	root := (&Node{Type: "column"}).SetID("root").Add(
		(&Node{Type: "panel"}).SetID("dlg").Modal(true).Backdrop("dim").CloseOnEsc(false),
		(&Node{Type: "list"}).SetID("lst").Items("a", "b").MultiSelect(true).ItemHeight(2).Selected(1),
		(&Node{Type: "filepicker"}).SetID("fp").ShowIcons(true).CurrentDir("/work").Prop("items", []any{
			map[string]any{"text": "a.go", "path": "/work/a.go"},
		}),
		(&Node{Type: "box"}).SetID("padbox").Prop("padding", []float64{1, 2}).Prop("margin", []int{1, 2, 3, 4}),
	)
	if _, err := ValidateTree(root); err != nil {
		t.Fatalf("ValidateTree failed on builder props: %v", err)
	}

	clonedPad := CloneNode(root.Children[3])
	clonedPad.Props["padding"].([]float64)[0] = 99
	rootFp := root.Children[2]
	clonedFp := CloneNode(rootFp)
	clonedFp.Props["items"].([]any)[0].(map[string]any)["text"] = "changed"
	if root.Children[3].Props["padding"].([]float64)[0] != 1 {
		t.Fatal("CloneNode aliased []float64 padding slice")
	}
	if rootFp.Props["items"].([]any)[0].(map[string]any)["text"] != "a.go" {
		t.Fatal("CloneNode aliased nested items slice/map")
	}

	v32, err := CloneValue(float32(2.5))
	if err != nil || v32 != float32(2.5) {
		t.Fatalf("CloneValue float32 = %v, %v", v32, err)
	}

	deep := &Node{Type: "box"}
	cur := deep
	for i := 0; i < 25; i++ {
		child := &Node{Type: "box"}
		cur.Children = []*Node{child}
		cur = child
	}
	AutoAssignIDs(deep)
	ids, err := ValidateTree(deep)
	if err != nil {
		t.Fatalf("ValidateTree failed after AutoAssignIDs on deep tree: %v", err)
	}
	for id := range ids {
		if len(id) > MaxIDBytes {
			t.Fatalf("generated id exceeded MaxIDBytes: %d", len(id))
		}
	}

	for _, invalid := range []*Node{
		{Type: "input", Props: map[string]any{"disabled": true}},
		{Type: "button", Props: map[string]any{"hidden": true}},
		{Type: "timer", Props: map[string]any{"hidden": true}},
		{Type: "list", Props: map[string]any{"disabled": true}},
	} {
		if _, err := ValidateTree(invalid); err == nil {
			t.Fatalf("expected missing id error for %s", invalid.Type)
		}
	}
}
