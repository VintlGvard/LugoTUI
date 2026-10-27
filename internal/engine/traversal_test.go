package engine

import (
	"slices"
	"testing"
)

func TestTraversal(t *testing.T) {
	a := &Node{ID: "a", Props: map[string]any{"z_index": float64(2)}}
	b := &Node{ID: "b"}
	c := &Node{ID: "c", Props: map[string]any{"hidden": true}, Children: []*Node{{ID: "hidden-child"}}}
	d := &Node{ID: "d"}
	root := &Node{ID: "root", Children: []*Node{a, b, c, d}}
	if got := slices.Collect(ordered(root)); !slices.Equal(got, []*Node{root, a, b, c, c.Children[0], d}) {
		t.Fatalf("tree order: %v", got)
	}
	if got := slices.Collect(visibleNodes(root)); !slices.Equal(got, []*Node{root, b, d, a}) {
		t.Fatalf("visible order: %v", got)
	}
	if !slices.Equal(root.Children, []*Node{a, b, c, d}) {
		t.Fatal("sorting changed retained tree")
	}
	for n := range visibleNodes(root) {
		if n != root {
			t.Fatal("iterator continued after break")
		}
		break
	}
	count := 0
	ordered(root)(func(*Node) bool { count++; return false })
	if count != 1 {
		t.Fatalf("early stop visited %d nodes", count)
	}
	if got := slices.Collect(visibleNodes(c)); len(got) != 0 {
		t.Fatal("hidden root was traversed")
	}
}

func TestSortedChildrenReuse(t *testing.T) {
	root := &Node{Children: []*Node{{ID: "a"}, {ID: "b"}}}
	if got := sortedChildren(root); &got[0] != &root.Children[0] {
		t.Fatal("already sorted children were copied")
	}
}

func BenchmarkVisibleNodes(b *testing.B) {
	root := &Node{}
	for range 1000 {
		root.Children = append(root.Children, &Node{})
	}
	b.ReportAllocs()
	for b.Loop() {
		for range visibleNodes(root) {
		}
	}
}
