package assets_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/VintlGvard/LugoTUI/internal/assets"
)

func TestAssets(t *testing.T) {
	if len(assets.DemoSource) == 0 {
		t.Fatal("empty DemoSource")
	}
	if !strings.Contains(assets.DemoSource, "require('ui')") {
		t.Fatal("DemoSource missing expected content")
	}
	data, err := fs.ReadFile(assets.FS, "demo.lua")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != assets.DemoSource {
		t.Fatal("FS content differs from DemoSource")
	}
}
