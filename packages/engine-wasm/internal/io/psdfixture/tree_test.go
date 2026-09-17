package psdfixture

import (
	"strconv"
	"strings"
	"testing"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func TestRenderTreeColumns(t *testing.T) {
	got := RenderTree(sampleTree())
	want := strings.Join([]string{
		"Background  pixel  0,0 64x48  normal    op=255  fill=255  vis",
		"Group A     group  -          normal    op=128  fill=255  vis",
		"  Fill      pixel  8,8 32x32  multiply  op=255  fill=255  vis",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("RenderTree mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderTreeNilRoot(t *testing.T) {
	if got := RenderTree(nil); got != "" {
		t.Fatalf("RenderTree(nil) = %q", got)
	}
}

func TestRenderTreeShowsHiddenLayers(t *testing.T) {
	root := model.NewGroupLayer("Root")
	hidden := model.NewPixelLayer("Hidden", model.LayerBounds{W: 2, H: 2}, make([]byte, 16))
	hidden.SetVisible(false)
	root.SetChildren([]model.LayerNode{hidden})

	if got := RenderTree(root); !strings.HasSuffix(strings.TrimRight(got, "\n"), "hid") {
		t.Fatalf("RenderTree = %q, want a trailing \"hid\" column", got)
	}
}

// TestRenderExpectedTreeMatchesRenderTree is the property the diff depends on:
// a sidecar that pins every column must render byte-identically to the tree it
// describes, or the diff would flag lines that did not actually differ.
func TestRenderExpectedTreeMatchesRenderTree(t *testing.T) {
	exp := sampleExpectation()
	want := RenderTree(sampleTree())
	if got := RenderExpectedTree(*exp.Layers); got != want {
		t.Fatalf("expected tree differs:\n--- expected ---\n%s\n--- actual ---\n%s", got, want)
	}
}

func TestRenderExpectedTreeMarksNilColumns(t *testing.T) {
	got := RenderExpectedTree([]LayerExpect{{Path: "Only A Path"}})
	want := "Only A Path  ?  ?  ?  op=?  fill=?  ?\n"
	if got != want {
		t.Fatalf("RenderExpectedTree = %q, want %q", got, want)
	}
}

func TestRenderExpectedTreeRendersBoundlessKindsAsDash(t *testing.T) {
	got := RenderExpectedTree([]LayerExpect{
		{Path: "Group", Name: strPtr("Group"), Type: strPtr("group")},
		{Path: "Curves", Name: strPtr("Curves"), Type: strPtr("adjustment")},
		{Path: "Pixels", Name: strPtr("Pixels"), Type: strPtr("pixel")},
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	// A group or adjustment layer has no bounds to assert, so the bounds column
	// renders "-" just as the actual tree does; only a kind that DOES have
	// bounds renders "?" when the sidecar left them out.
	boundsColumn := func(line string) string { return strings.Fields(line)[2] }
	for i, kind := range []string{"group", "adjustment"} {
		if got := boundsColumn(lines[i]); got != "-" {
			t.Errorf("line %d (%q): %s bounds column = %q, want \"-\"", i, lines[i], kind, got)
		}
	}
	if got := boundsColumn(lines[2]); got != "?" {
		t.Errorf("line 2 (%q): pixel bounds column = %q, want \"?\"", lines[2], got)
	}
}

func TestRenderExpectedTreeEmpty(t *testing.T) {
	if got := RenderExpectedTree(nil); got != "" {
		t.Fatalf("RenderExpectedTree(nil) = %q", got)
	}
}

func TestUnifiedDiffIdenticalIsEmpty(t *testing.T) {
	if got := UnifiedDiff("a\nb\n", "a\nb\n"); got != "" {
		t.Fatalf("UnifiedDiff = %q, want empty", got)
	}
}

func TestUnifiedDiffMarksBothSides(t *testing.T) {
	got := UnifiedDiff("keep\nold\ntail\n", "keep\nnew\ntail\n")
	want := strings.Join([]string{
		"tree diff (-want +got):",
		"  keep",
		"- old",
		"+ new",
		"  tail",
	}, "\n")
	if got != want {
		t.Fatalf("UnifiedDiff:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestUnifiedDiffHandlesInsertionsAndDeletions(t *testing.T) {
	got := UnifiedDiff("a\n", "a\nb\nc\n")
	if !strings.Contains(got, "+ b") || !strings.Contains(got, "+ c") {
		t.Fatalf("UnifiedDiff = %q", got)
	}
	got = UnifiedDiff("a\nb\nc\n", "a\n")
	if !strings.Contains(got, "- b") || !strings.Contains(got, "- c") {
		t.Fatalf("UnifiedDiff = %q", got)
	}
}

func TestUnifiedDiffEmptySides(t *testing.T) {
	if got := UnifiedDiff("", "only\n"); !strings.Contains(got, "+ only") {
		t.Fatalf("UnifiedDiff = %q", got)
	}
	if got := UnifiedDiff("only\n", ""); !strings.Contains(got, "- only") {
		t.Fatalf("UnifiedDiff = %q", got)
	}
}

// TestUnifiedDiffFallsBackOnLargeTrees pins the bound: above maxDiffLines the
// quadratic LCS is skipped in favour of a plain both-sides dump.
func TestUnifiedDiffFallsBackOnLargeTrees(t *testing.T) {
	var want, got strings.Builder
	for i := 0; i < maxDiffLines; i++ {
		want.WriteString("line " + strconv.Itoa(i) + "\n")
		got.WriteString("line " + strconv.Itoa(i+1) + "\n")
	}
	diff := UnifiedDiff(want.String(), got.String())
	if !strings.HasPrefix(diff, "tree (want):") {
		t.Fatalf("want the both-sides fallback, got %q", diff[:min(120, len(diff))])
	}
	if strings.Contains(diff, "tree diff (-want +got)") {
		t.Fatal("the LCS diff should not run above the line bound")
	}
}

func TestUnifiedDiffStaysUnderTheBound(t *testing.T) {
	var want, got strings.Builder
	for i := 0; i < 10; i++ {
		want.WriteString("line " + strconv.Itoa(i) + "\n")
		got.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	got.WriteString("extra\n")
	diff := UnifiedDiff(want.String(), got.String())
	if !strings.HasPrefix(diff, "tree diff (-want +got):") {
		t.Fatalf("want the LCS diff, got %q", diff)
	}
}
