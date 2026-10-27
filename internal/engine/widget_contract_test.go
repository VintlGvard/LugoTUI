package engine

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNativeWidgetActionContracts(t *testing.T) {
	for _, source := range []string{
		`local ui=require('ui'); return {view=function() return ui.timer{id='t',on_timeout='missing'} end}`,
		`local ui=require('ui'); return {view=function() return ui.filepicker{id='f',items={},on_navigate='missing'} end}`,
		`local ui=require('ui'); return {view=function() return ui.help{id='h',keys={ui.key{keys={'x'},action='missing'}}} end}`,
	} {
		r, err := Load(source)
		if r != nil {
			r.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "unknown action") {
			t.Fatalf("missing action accepted: %v", err)
		}
	}
}

func TestNativeHelpKeyModifiers(t *testing.T) {
	if !primitiveKeyMatches("enter", tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)) {
		t.Fatal("plain Enter did not match")
	}
	if primitiveKeyMatches("enter", tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModAlt)) {
		t.Fatal("plain Enter matched Alt+Enter")
	}
}

func TestNativeWidgetStableIDs(t *testing.T) {
	for _, kind := range []string{"spinner", "progress", "table", "viewport", "tree", "filepicker", "paginator", "timer", "stopwatch", "help"} {
		_, err := validateTree(&Node{Type: kind, Props: map[string]any{}})
		if err == nil || !strings.Contains(err.Error(), "requires id") {
			t.Errorf("%s: %v", kind, err)
		}
	}
}
