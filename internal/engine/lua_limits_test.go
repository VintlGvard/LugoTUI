package engine

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func limitsLuaValue(t *testing.T, source string) lua.LValue {
	t.Helper()
	L := lua.NewState()
	t.Cleanup(L.Close)
	if err := L.DoString(source); err != nil {
		t.Fatal(err)
	}
	return L.Get(-1)
}

func TestLuaExpansionBudgets(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		tree               bool
	}{
		{"tree aliases", `local n={type="box"}; for i=1,30 do n={type="box",children={n,n}} end; return n`, "4096 nodes", true},
		{"inline tree aliases", `local n={type="box"}; for i=1,30 do n={type="box",n,n} end; return n`, "4096 nodes", true},
		{"array aliases", `local v={}; for i=1,29 do v={v,v} end; return v`, "expanded values", false},
		{"object aliases", `local v={}; for i=1,29 do v={a=v,b=v} end; return v`, "expanded values", false},
		{"string aliases", `local v={string.rep("x",600000)}; return {v,v}`, "text exceeds", false},
		{"key aliases", `local v={[string.rep("x",600000)]=true}; return {v,v}`, "text exceeds", false},
		{"properties share budget", `local v=string.rep("x",600000); return {type="box",a=v,b=v}`, "text exceeds", true},
		{"nodes share text budget", `local n={type="text",text=string.rep("x",600000)}; return {type="box",children={n,n}}`, "text exceeds", true},
		{"node key budget", `return {type="box",[string.rep("x",1048576)]=true}`, "text exceeds", true},
		{"properties share value budget", `local v={}; for i=1,12 do v={v,v} end; local n={type="box"}; for i=1,40 do n["p"..i]=v end; return n`, "expanded values", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := limitsLuaValue(t, tc.source)
			budget := &luaBudget{}
			var err error
			if tc.tree {
				_, err = parseNodeBudget(v.(*lua.LTable), 0, budget)
			} else {
				_, err = luaPlainBudget(v, 0, budget)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			if budget.nodes > maxNodes || budget.values > maxLuaValues || budget.text > maxText {
				t.Fatalf("budget exceeded before rejection: %+v", budget)
			}
		})
	}
}

func TestLuaExpansionBoundaries(t *testing.T) {
	t.Run("4096 nodes", func(t *testing.T) {
		v := limitsLuaValue(t, `local n={type="box"}; local c={}; for i=1,4095 do c[i]=n end; return {type="box",children=c}`)
		if _, err := parseNode(v.(*lua.LTable), 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tree depth 64", func(t *testing.T) {
		v := limitsLuaValue(t, `local n={type="box"}; for i=1,64 do n={type="box",n} end; return n`)
		if _, err := parseNode(v.(*lua.LTable), 0); err != nil {
			t.Fatal(err)
		}
		if _, err := parseNode(v.(*lua.LTable), 1); err == nil || !strings.Contains(err.Error(), "depth exceeds 64") {
			t.Fatalf("expected depth rejection, got %v", err)
		}
	})
	t.Run("1 MiB string", func(t *testing.T) {
		v := limitsLuaValue(t, `return string.rep("x",1048576)`)
		if _, err := luaPlain(v, 0); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLuaRuntimeRejectsExpansion(t *testing.T) {
	for _, source := range []string{
		`local n={type="box"}; for i=1,30 do n={type="box",n,n} end; return {view=n}`,
		`local v={}; for i=1,29 do v={v,v} end; ui.set("v",v); return {view={type="box"}}`,
		`local s = string.rep("x", 1048577); return {view={type="box"}}`,
		`local s = ("x"):rep(500000, "--"); return {view={type="box"}}`,
	} {
		r, err := loadRuntime(source, "", nil)
		if r != nil {
			r.L.Close()
		}
		if err == nil {
			t.Fatalf("expanded Lua source accepted: %s", source)
		}
	}
}

func TestActionRollsBackLuaGlobalsAndDeepData(t *testing.T) {
	r, err := Load(`counter = 0
return {
  view = {type = "box", id = "root"},
  actions = {
    fail = function()
      counter = counter + 1
      error("boom")
    end,
    read = function()
      ui.set("c", counter)
    end
  }
}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	nested := map[string]any{"k": "v1"}
	r.Data["nested"] = nested
	if err := r.Action("fail", nil); err == nil {
		t.Fatal("expected fail action to return error")
	}
	if err := r.Action("read", nil); err != nil {
		t.Fatal(err)
	}
	if got := r.Data["c"]; got != float64(0) {
		t.Fatalf("Lua global counter was not rolled back after failed action: got %v, want 0", got)
	}
	r2, err := r.Reload(`return {view = {type = "box", id = "root"}}`)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	r2.Data["nested"].(map[string]any)["k"] = "mutated"
	if nested["k"] != "v1" {
		t.Fatalf("Reload shared nested Data map with previous runtime: got %v", nested["k"])
	}
}
