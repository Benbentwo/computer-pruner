package scanner

import (
	"fmt"
	"testing"

	"github.com/Benbentwo/computer-pruner/internal/preferences"
)

// dirNode builds a directory node whose Size/FileCount/DirCount are derived from
// its children, exactly as scanDir derives them.
func dirNode(name string, children ...*TreeNode) *TreeNode {
	n := &TreeNode{Name: name, Path: "/" + name, IsDir: true, Children: children}
	for _, c := range children {
		n.Size += c.Size
		if c.IsDir {
			n.DirCount += 1 + c.DirCount
			n.FileCount += c.FileCount
		} else {
			n.FileCount++
		}
	}
	return n
}

func fileNode(name string, size int64) *TreeNode {
	return &TreeNode{Name: name, Path: "/" + name, Size: size, Children: []*TreeNode{}}
}

func otherNode(t *testing.T, pruned *TreeNode) *TreeNode {
	t.Helper()
	n := findChild(pruned, "(other small items)")
	if n == nil {
		t.Fatalf("expected an \"(other small items)\" node; children: %v", childNames(pruned))
	}
	return n
}

func childNames(n *TreeNode) []string {
	out := make([]string, 0, len(n.Children))
	for _, c := range n.Children {
		out = append(out, c.Name)
	}
	return out
}

// TestPruneTreeGroupedCountsAreExact pins bug #3. The grouping branch used to do
//
//	otherFiles += child.FileCount
//	otherDirs  += child.DirCount
//	otherCount++
//	if child.IsDir { otherDirs++ } else { otherFiles++ }
//
// which, for a directory child, added the subtree's own counts and then
// incremented again — inflating both figures on the summary node.
func TestPruneTreeGroupedCountsAreExact(t *testing.T) {
	// A directory small enough to be grouped, containing 3 files and 2
	// directories in total.
	//
	//   small/            <- 1 dir
	//     inner/          <- 1 dir
	//       i1 (1), i2 (1)
	//     s1 (1)
	//
	// Grouping "small" must contribute exactly 2 dirs and 3 files.
	small := dirNode("small",
		dirNode("inner", fileNode("i1", 1), fileNode("i2", 1)),
		fileNode("s1", 1),
	)
	if small.DirCount != 1 || small.FileCount != 3 {
		t.Fatalf("test fixture is wrong: small has DirCount=%d FileCount=%d", small.DirCount, small.FileCount)
	}

	root := dirNode("root",
		fileNode("huge", 1_000_000), // way above the threshold, kept
		small,                       // 3 bytes, grouped
		fileNode("tiny", 1),         // grouped
	)

	pruned := pruneTree(root, preferences.DefaultScanDepth)
	other := otherNode(t, pruned)

	if other.DirCount != 2 {
		t.Fatalf("grouped DirCount = %d, want 2 (small + small/inner counted once each)", other.DirCount)
	}
	if other.FileCount != 4 {
		t.Fatalf("grouped FileCount = %d, want 4 (small's 3 files + the grouped tiny file)", other.FileCount)
	}
	if other.Size != 4 {
		t.Fatalf("grouped Size = %d, want 4", other.Size)
	}
	if !other.IsDir {
		t.Fatal("the summary node should present as a directory")
	}

	// And the grouped counts must never exceed what the parent actually holds.
	if other.DirCount > root.DirCount || other.FileCount > root.FileCount {
		t.Fatalf("grouped counts (%d dirs, %d files) exceed the parent's own totals (%d dirs, %d files)",
			other.DirCount, other.FileCount, root.DirCount, root.FileCount)
	}
}

func TestPruneTreeGroupsByChildLimit(t *testing.T) {
	// Equal-sized children so the size threshold never fires: only the
	// maxChildrenPerNode cap does the grouping.
	children := make([]*TreeNode, 0, maxChildrenPerNode+10)
	for i := 0; i < maxChildrenPerNode+10; i++ {
		children = append(children, fileNode(fmt.Sprintf("f%03d", i), 1000))
	}
	root := dirNode("root", children...)

	pruned := pruneTree(root, preferences.DefaultScanDepth)

	// maxChildrenPerNode kept children plus one summary node.
	if len(pruned.Children) != maxChildrenPerNode+1 {
		t.Fatalf("pruned children = %d, want %d", len(pruned.Children), maxChildrenPerNode+1)
	}
	other := otherNode(t, pruned)
	if other.FileCount != 10 {
		t.Fatalf("grouped FileCount = %d, want 10", other.FileCount)
	}
	if other.DirCount != 0 {
		t.Fatalf("grouped DirCount = %d, want 0 (all grouped children are files)", other.DirCount)
	}
	if other.Size != 10_000 {
		t.Fatalf("grouped Size = %d, want 10000", other.Size)
	}
}

func TestPruneTreeAddsNoSummaryWhenNothingIsGrouped(t *testing.T) {
	root := dirNode("root", fileNode("a", 100), fileNode("b", 100))
	pruned := pruneTree(root, preferences.DefaultScanDepth)

	if len(pruned.Children) != 2 {
		t.Fatalf("pruned children = %v, want exactly the two originals", childNames(pruned))
	}
	if findChild(pruned, "(other small items)") != nil {
		t.Fatal("no summary node should be added when nothing was grouped")
	}
}

func TestPruneTreeDoesNotMutateTheSource(t *testing.T) {
	root := dirNode("root",
		fileNode("huge", 1_000_000),
		fileNode("tiny", 1),
	)
	before := len(root.Children)
	beforeSize := root.Size

	_ = pruneTree(root, 2)

	if len(root.Children) != before || root.Size != beforeSize {
		t.Fatal("pruneTree must not modify the tree it is given")
	}
	if findChild(root, "(other small items)") != nil {
		t.Fatal("pruneTree leaked a synthetic node into the source tree")
	}
}

func TestPruneTreeClampsItsDepthArgument(t *testing.T) {
	// A five-level chain.
	node := fileNode("leaf", 10)
	for i := 4; i >= 0; i-- {
		node = dirNode(fmt.Sprintf("d%d", i), node)
	}

	const fullDepth = 5 // d0 -> d1 -> d2 -> d3 -> d4 -> leaf

	tests := []struct {
		depth int
		want  int
	}{
		{0, fullDepth},       // unset -> default 8, which covers the whole chain
		{-1, fullDepth},      // nonsense -> default 8
		{1 << 20, fullDepth}, // clamped to 64, still covers the whole chain
		{2, 2},               // an in-range depth is honoured verbatim
		{preferences.MaxScanDepth, fullDepth},
	}

	for _, tt := range tests {
		if got := treeEdgeDepth(pruneTree(node, tt.depth)); got != tt.want {
			t.Errorf("pruneTree(depth=%d) produced depth %d, want %d", tt.depth, got, tt.want)
		}
	}
}

func TestPruneTreeHandlesNil(t *testing.T) {
	if pruneTree(nil, 4) != nil {
		t.Fatal("pruneTree(nil) must return nil")
	}
}
