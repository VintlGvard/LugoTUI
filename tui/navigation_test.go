package tui_test

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestExplicitNavigation(t *testing.T) {
	source := `return {
		view = {
			type = 'column',
			{ type = 'input', id = 'search', nav_down = 'table1' },
			{ type = 'table', id = 'table1', nav_up = 'search', nav = { right = 'btn_save' }, columns = {{ title = 'Col', width = 10 }}, rows = {{ 'Row1' }} },
			{ type = 'button', id = 'btn_save', text = 'Save', nav_left = 'table1' },
			{ type = 'button', id = 'btn_blocked', text = 'Blocked', nav_down = 'none' },
		},
		actions = {}
	}`
	r := mouseFixture(t, source)
	if r.Focus() != "search" {
		t.Fatalf("expected search focused, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if r.Focus() != "table1" {
		t.Fatalf("expected table1 focused on down, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if r.Focus() != "btn_save" {
		t.Fatalf("expected btn_save focused on right, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if r.Focus() != "table1" {
		t.Fatalf("expected table1 focused on left, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyUp, 0, 0))
	if r.Focus() != "search" {
		t.Fatalf("expected search focused on up, got %s", r.Focus())
	}
	if err := r.SetFocus("btn_blocked"); err != nil {
		t.Fatal(err)
	}
	r.Key(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if r.Focus() != "btn_blocked" {
		t.Fatalf("expected btn_blocked to stay focused, got %s", r.Focus())
	}
}

func TestSpatialNavigationButtons(t *testing.T) {
	source := `return {
		view = {
			type = 'column',
			{
				type = 'row',
				height = 3,
				{ type = 'button', id = 'btn1', text = 'One' },
				{ type = 'button', id = 'btn2', text = 'Two' },
				{ type = 'button', id = 'btn3', text = 'Three' },
			},
			{
				type = 'row',
				height = 3,
				{ type = 'button', id = 'btn4', text = 'Four' },
				{ type = 'button', id = 'btn5', text = 'Five' },
			}
		},
		actions = {}
	}`
	r := mouseFixture(t, source)
	if r.Focus() != "btn1" {
		t.Fatalf("expected btn1 focused, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if r.Focus() != "btn2" {
		t.Fatalf("expected btn2 on right, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if r.Focus() != "btn3" {
		t.Fatalf("expected btn3 on right, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if r.Focus() != "btn2" {
		t.Fatalf("expected btn2 on left, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if r.Focus() != "btn5" && r.Focus() != "btn4" {
		t.Fatalf("expected btn4 or btn5 on down, got %s", r.Focus())
	}
}

func TestSpatialNavigationGlobalOptIn(t *testing.T) {
	source := `return {
		spatial_nav = true,
		view = {
			type = 'row',
			{ type = 'table', id = 'tbl', width = 30, columns = {{ title = 'Col', width = 10 }}, rows = {{ 'val' }} },
			{ type = 'button', id = 'side_btn', width = 20, text = 'Action' },
		},
		actions = {}
	}`
	r := mouseFixture(t, source)
	if r.Focus() != "tbl" {
		t.Fatalf("expected tbl focused, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if r.Focus() != "side_btn" {
		t.Fatalf("expected side_btn on right with spatial_nav, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyLeft, 0, 0))
	if r.Focus() != "tbl" {
		t.Fatalf("expected tbl on left with spatial_nav, got %s", r.Focus())
	}
}

func TestWidgetKeymapAndOnKey(t *testing.T) {
	source := `return {
		view = {
			type = 'column',
			{
				type = 'table',
				id = 'custom_table',
				columns = {{ title = 'Name', width = 10 }},
				rows = {{ 'Pkg1' }, { 'Pkg2' }},
				keymap = {
					['Space'] = 'toggle_select',
					['i'] = 'install_pkg',
				}
			},
			{
				type = 'input',
				id = 'custom_input',
				on_key = 'any_key',
			}
		},
		actions = {
			toggle_select = function(e)
				host.dispatch('toggled', { id = e.id, key = e.key })
			end,
			install_pkg = function(e)
				host.dispatch('installed', { id = e.id, key = e.key })
			end,
			any_key = function(e)
				host.dispatch('keyed', { id = e.id, key = e.key, rune = e.rune })
			end,
		}
	}`
	r := mouseFixture(t, source)
	if r.Focus() != "custom_table" {
		t.Fatalf("expected custom_table focused, got %s", r.Focus())
	}
	r.Key(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
	events := r.DrainEvents()
	if len(events) != 1 || events[0].Action != "toggled" {
		t.Fatalf("expected toggled event, got %+v", events)
	}
	r.Key(tcell.NewEventKey(tcell.KeyRune, 'i', 0))
	events = r.DrainEvents()
	if len(events) != 1 || events[0].Action != "installed" {
		t.Fatalf("expected installed event, got %+v", events)
	}
	if err := r.SetFocus("custom_input"); err != nil {
		t.Fatal(err)
	}
	r.Key(tcell.NewEventKey(tcell.KeyRune, 'z', 0))
	events = r.DrainEvents()
	if len(events) != 1 || events[0].Action != "keyed" {
		t.Fatalf("expected keyed event from on_key, got %+v", events)
	}
}

func TestLuaFocusAPIs(t *testing.T) {
	source := `return {
		view = {
			type = 'column',
			{ type = 'button', id = 'top', text = 'Top' },
			{ type = 'button', id = 'mid', text = 'Mid' },
			{ type = 'button', id = 'bot', text = 'Bot' },
		},
		actions = {
			test_nav = function()
				local initial = ui.focused()
				local target = ui.focus_direction('down')
				local current = ui.focused()
				local next_f = ui.focus_next()
				local prev_f = ui.focus_prev()
				host.dispatch('report', {
					initial = initial,
					target = target,
					current = current,
					next = next_f,
					prev = prev_f,
				})
			end,
		}
	}`
	r := mouseFixture(t, source)
	if err := r.Action("test_nav", nil); err != nil {
		t.Fatal(err)
	}
	events := r.DrainEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	payload, ok := events[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("bad payload: %+v", events[0].Payload)
	}
	if payload["initial"] != "top" {
		t.Errorf("expected initial top, got %v", payload["initial"])
	}
	if payload["target"] != "mid" {
		t.Errorf("expected target mid, got %v", payload["target"])
	}
	if payload["current"] != "mid" {
		t.Errorf("expected current mid, got %v", payload["current"])
	}
	if payload["next"] != "bot" {
		t.Errorf("expected next bot, got %v", payload["next"])
	}
	if payload["prev"] != "mid" {
		t.Errorf("expected prev mid, got %v", payload["prev"])
	}
}

func TestProgrammaticGoFocusDirection(t *testing.T) {
	source := `return {
		view = {
			type = 'column',
			{ type = 'button', id = 'b1', text = '1' },
			{ type = 'button', id = 'b2', text = '2' },
		},
		actions = {}
	}`
	r := mouseFixture(t, source)
	if r.Focus() != "b1" {
		t.Fatalf("expected b1, got %s", r.Focus())
	}
	if !r.FocusDirection("down") {
		t.Fatal("expected FocusDirection down to succeed")
	}
	if r.Focus() != "b2" {
		t.Fatalf("expected b2, got %s", r.Focus())
	}
	if !r.FocusDirection("up") {
		t.Fatal("expected FocusDirection up to succeed")
	}
	if r.Focus() != "b1" {
		t.Fatalf("expected b1, got %s", r.Focus())
	}
	if next := r.FocusNext(); next != "b2" {
		t.Fatalf("expected FocusNext b2, got %s", next)
	}
	if prev := r.FocusPrev(); prev != "b1" {
		t.Fatalf("expected FocusPrev b1, got %s", prev)
	}
}
