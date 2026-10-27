package engine

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTextareaAltEnterRespectsLimit(t *testing.T) {
	r, err := Load(`return {view=ui.textarea{id='edit',text='x',char_limit=1}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt)
	if err := r.Key(event); err != nil {
		t.Fatal(err)
	}
	if got := r.State["edit"].Value; got != "x" {
		t.Fatalf("Alt+Enter bypassed char_limit: %q", got)
	}
	if err := r.Patch("edit", map[string]any{"char_limit": 0.0}); err != nil {
		t.Fatal(err)
	}
	if err := r.Key(event); err != nil {
		t.Fatal(err)
	}
	if got := r.State["edit"].Value; !strings.Contains(got, "\n") {
		t.Fatalf("Alt+Enter did not insert newline: %q", got)
	}
}

func TestEditorConfiguration(t *testing.T) {
	for _, props := range []map[string]any{
		{"echo": "secret"}, {"echo_character": ""}, {"echo_character": "ab"}, {"echo_character": "\n"},
		{"char_limit": -1.0}, {"char_limit": 1.5}, {"char_limit": "3"}, {"validation_pattern": "["},
		{"suggestions": []any{"ok", false}}, {"suggestions": []string{"two\nlines"}}, {"line_numbers": "yes"},
	} {
		if err := validateEditor(&Node{Type: "input", Props: props}); err == nil {
			t.Errorf("accepted invalid props: %v", props)
		}
	}
	for _, props := range []map[string]any{
		{"suggestions": make([]string, maxNodes+1)},
		{"suggestions": []string{strings.Repeat("x", maxText+1)}},
	} {
		if err := validateEditor(&Node{Type: "input", Props: props}); err == nil {
			t.Errorf("unbounded suggestions accepted")
		}
	}
	if err := validateEditor(&Node{Type: "text", Props: map[string]any{"char_limit": "x"}}); err == nil {
		t.Fatal("editor property accepted on text node")
	}
	if err := validateEditor(&Node{Type: "textarea", Props: map[string]any{"echo": "password", "echo_character": "🔒", "char_limit": 3.0, "suggestions": []string{"valid"}, "validation_pattern": "^[a-z]+$", "line_numbers": true}}); err != nil {
		t.Fatal(err)
	}
}

func TestEditorInsertGraphemeLimitsAndUndo(t *testing.T) {
	n := &Node{Type: "input", Props: map[string]any{"char_limit": 3.0}}
	s := &WidgetState{Value: "a👩‍💻b", Cursor: 2, Anchor: 1}
	changed, err := editorInsert(n, s, "éxy")
	if err != nil || !changed || s.Value != "aéb" || s.Cursor != 2 || s.Anchor != -1 || len(s.Undo) != 1 || s.Undo[0] != "a👩‍💻b" {
		t.Fatalf("replacement: %+v, %v", s, err)
	}
	changed, err = editorInsert(n, s, "z")
	if err != nil || changed || len(s.Undo) != 1 {
		t.Fatalf("full limit changes undo: %+v %v", s, err)
	}
	n.Props["char_limit"] = 0.0
	s.Value = strings.Repeat("x", maxText)
	s.Cursor = maxText
	s.Anchor = maxText - 1
	changed, err = editorInsert(n, s, "y")
	if err != nil || !changed || len(s.Value) != maxText {
		t.Fatalf("exact byte replacement: changed %v, %v", changed, err)
	}
	before := s.Value
	changed, err = editorInsert(n, s, "é")
	if err != errInputLimit || changed || s.Value != before {
		t.Fatalf("byte limit not atomic: %v %v", changed, err)
	}
}

func TestEditorCombiningMarkAtLimit(t *testing.T) {
	n := &Node{Type: "input", Props: map[string]any{"char_limit": 1.0}}
	s := &WidgetState{Value: "e", Cursor: 1, Anchor: -1}
	changed, err := editorInsert(n, s, "\u0301")
	if err != nil || !changed || s.Value != "e\u0301" || s.Cursor != 1 {
		t.Fatalf("combining mark: %+v, %v", s, err)
	}
}

func TestEditorEchoAndSuggestions(t *testing.T) {
	n := &Node{Type: "input", Props: map[string]any{"suggestions": []any{"hello", "help", "helm"}}}
	s := &WidgetState{Value: "hel", Cursor: 3, Anchor: -1}
	if got := editorSuggestion(n, s); got != "lo" {
		t.Fatalf("suggestion %q", got)
	}
	s.Selected = 1
	if got := editorSuggestion(n, s); got != "p" {
		t.Fatalf("cycled suggestion %q", got)
	}
	s.Cursor = 1
	if got := editorSuggestion(n, s); got != "" {
		t.Fatalf("suggestion away from end %q", got)
	}
	s.Value = "👩‍💻é"
	n.Props["echo"] = "password"
	if got := editorDisplay(n, s); got != "**" {
		t.Fatalf("mask %q", got)
	}
	if got := editorSuggestion(n, s); got != "" {
		t.Fatalf("secret completion leaked %q", got)
	}
	n.Props["echo"] = "none"
	if got := editorDisplay(n, s); got != "" {
		t.Fatalf("hidden echo %q", got)
	}
	if got := len(editorGraphemes(n, s)); got != 2 {
		t.Fatalf("hidden editor lost indices: %d", got)
	}
	s.Value = ""
	n.Props["placeholder"] = "secret"
	if got := editorDisplay(n, s); got != "secret" {
		t.Fatalf("placeholder %q", got)
	}
}

func TestEditorValidationRetainsInvalidValue(t *testing.T) {
	n := &Node{Type: "input", Props: map[string]any{"validation_pattern": "^[0-9]+$", "validation_error": "digits required"}}
	s := &WidgetState{Anchor: -1}
	if changed, err := editorInsert(n, s, "bad"); !changed || err != nil {
		t.Fatalf("invalid user input rejected: %v %v", changed, err)
	}
	if s.Value != "bad" || editorValidation(n, s.Value) != "digits required" {
		t.Fatalf("validation: %+v", s)
	}
	if got := editorValidation(n, "123"); got != "" {
		t.Fatalf("valid input: %q", got)
	}
}

func TestEditorWordMovementAndSelection(t *testing.T) {
	n := &Node{Type: "textarea", ID: "edit", Props: map[string]any{}}
	s := &WidgetState{Value: "one  👩‍💻two\nthree", Cursor: 15, Anchor: -1}
	s.Cursor = len(graphemes(s.Value))
	r := &Runtime{State: map[string]*WidgetState{"edit": s}}
	for _, tc := range []struct {
		key    tcell.Key
		mod    tcell.ModMask
		cursor int
		anchor int
	}{
		{tcell.KeyLeft, tcell.ModCtrl, 10, -1},
		{tcell.KeyLeft, tcell.ModCtrl | tcell.ModShift, 5, 10},
		{tcell.KeyRight, tcell.ModAlt, 9, -1},
		{tcell.KeyHome, tcell.ModCtrl, 0, -1},
	} {
		handled, err := r.editorKey(n, tcell.NewEventKey(tc.key, 0, tc.mod))
		if err != nil || !handled || s.Cursor != tc.cursor || s.Anchor != tc.anchor {
			t.Fatalf("%v: cursor=%d anchor=%d handled=%v err=%v", tc.key, s.Cursor, s.Anchor, handled, err)
		}
	}
}

func TestEditorKeyKillsCompletionAndUndo(t *testing.T) {
	r, err := Load(`local ui=require('ui'); return {view=ui.input{id='edit'},actions={}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	n := r.Nodes["edit"]
	s := r.State["edit"]
	s.Value = "one two three"
	s.Cursor = 7
	s.Anchor = -1
	for _, tc := range []struct {
		key    tcell.Key
		rune   rune
		mod    tcell.ModMask
		want   string
		cursor int
	}{
		{tcell.KeyCtrlW, 0, 0, "one  three", 4},
		{tcell.KeyCtrlK, 0, 0, "one ", 4},
		{tcell.KeyCtrlU, 0, 0, "", 0},
	} {
		handled, err := r.editorKey(n, tcell.NewEventKey(tc.key, tc.rune, tc.mod))
		if err != nil || !handled || s.Value != tc.want || s.Cursor != tc.cursor {
			t.Fatalf("%v: value=%q cursor=%d err=%v", tc.key, s.Value, s.Cursor, err)
		}
	}
	if len(s.Undo) != 3 {
		t.Fatalf("kills did not save undo: %+v", s.Undo)
	}
	s.Value = "hel"
	s.Cursor = 3
	s.Anchor = -1
	n.Props["suggestions"] = []string{"hello", "help"}
	if handled, err := r.editorKey(n, tcell.NewEventKey(tcell.KeyCtrlN, 0, 0)); !handled || err != nil || editorSuggestion(n, s) != "p" {
		t.Fatalf("cycle: %v %v", handled, err)
	}
	if handled, err := r.editorKey(n, tcell.NewEventKey(tcell.KeyTab, 0, 0)); !handled || err != nil || s.Value != "help" {
		t.Fatalf("accept: %q %v %v", s.Value, handled, err)
	}
	if handled, err := r.editorKey(n, tcell.NewEventKey(tcell.KeyTab, 0, 0)); handled || err != nil {
		t.Fatalf("tab should focus after completion: %v %v", handled, err)
	}
}

func TestEditorRuntimeCallbacksPasteAndUndo(t *testing.T) {
	r, err := Load(`local ui=require('ui'); return {view=ui.input{id='edit',char_limit=3,validation_pattern='^[0-9]+$',on_change='changed',on_submit='submitted'},actions={changed=function(e) ui.set('error',e.error); ui.set('value',e.value) end,submitted=function(e) ui.set('submit_error',e.error) end}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Paste("abxyz"); err != nil {
		t.Fatal(err)
	}
	s := r.State["edit"]
	if s.Value != "abx" || r.Data["error"] != "value does not match validation_pattern" || r.Data["value"] != "abx" {
		t.Fatalf("paste callback: %+v %+v", s, r.Data)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyEnter, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if r.Data["submit_error"] != "value does not match validation_pattern" {
		t.Fatalf("submit validation missing: %+v", r.Data)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlZ, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if s.Value != "" {
		t.Fatalf("undo paste: %q", s.Value)
	}
	if err := r.Key(tcell.NewEventKey(tcell.KeyCtrlY, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if s.Value != "abx" {
		t.Fatalf("redo paste: %q", s.Value)
	}
}

func TestEditorRuntimeMaskedRendering(t *testing.T) {
	r, err := Load(`local ui=require('ui'); return {view=ui.input{id='edit',width=20,height=1,prompt='> ',echo='password',text='secret'}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(20, 2)
	r.Draw(screen, "")
	var visible strings.Builder
	for x := 0; x < 20; x++ {
		c, _, _, _ := screen.GetContent(x, 0)
		visible.WriteRune(c)
	}
	if !strings.HasPrefix(visible.String(), "> ******") || strings.Contains(visible.String(), "secret") {
		t.Fatalf("mask render: %q", visible.String())
	}
}

func TestEditorPrefixAndMaskedPositions(t *testing.T) {
	n := &Node{Type: "textarea", Rect: Rect{W: 20, H: 3}, Props: map[string]any{"prompt": "> ", "line_numbers": true, "echo": "password"}}
	s := &WidgetState{Value: "界\nsecond"}
	if got := editorLinePrefix(n, s.Value, 2); got != "> 2 " {
		t.Fatalf("line prefix %q", got)
	}
	if got := editorLinePrefix(n, s.Value, 0); got != ">   " {
		t.Fatalf("wrap prefix %q", got)
	}
	cr := editorRect(n, s)
	if cr.X != 4 || cr.W != 16 {
		t.Fatalf("editor rect %+v", cr)
	}
	g, p := editorIndex(n, s, 16)
	if len(g) != 8 || g[1] != "\n" || p[1] != [2]int{1, 0} || p[2] != [2]int{0, 1} {
		t.Fatalf("mask positions %q %v", g, p)
	}
}
