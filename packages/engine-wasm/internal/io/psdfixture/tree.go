//go:build !js

package psdfixture

import (
	"strings"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

// maxDiffLines bounds the LCS table. Above it the quadratic diff stops being
// worth its cost and a plain both-sides dump is more readable anyway.
const maxDiffLines = 400

// indentUnit is one nesting level in a rendered tree.
const indentUnit = "  "

// RenderTree renders the children of root as canonical indented columns:
//
//	Background  pixel  0,0 64x48  normal  op=255 fill=255 vis
//
// The root group itself is not rendered: it is the synthetic document root and
// has no counterpart in a sidecar's "layers" array.
func RenderTree(root *model.GroupLayer) string {
	if root == nil {
		return ""
	}
	var rows [][]string
	var walk func(nodes []model.LayerNode, depth int)
	walk = func(nodes []model.LayerNode, depth int) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			rows = append(rows, []string{
				strings.Repeat(indentUnit, depth) + node.Name(),
				string(node.LayerType()),
				boundsCell(node),
				string(node.BlendMode()),
				"op=" + fmtByte(node.Opacity()),
				"fill=" + fmtByte(node.FillOpacity()),
				visibilityCell(node.Visible()),
			})
			walk(node.Children(), depth+1)
		}
	}
	walk(root.Children(), 0)
	return renderRows(rows)
}

// RenderExpectedTree renders a sidecar's layer expectations in exactly the
// format RenderTree produces, so the two can be diffed line by line. A column
// the sidecar left nil renders as "?" — it asserts nothing, and the diff must
// show that rather than inventing a default.
func RenderExpectedTree(layers []LayerExpect) string {
	var rows [][]string
	var walk func(nodes []LayerExpect, depth int)
	walk = func(nodes []LayerExpect, depth int) {
		for _, node := range nodes {
			rows = append(rows, []string{
				strings.Repeat(indentUnit, depth) + expectedName(node),
				optionalString(node.Type),
				expectedBoundsCell(node),
				optionalString(node.BlendMode),
				"op=" + optionalInt(node.Opacity255),
				"fill=" + optionalInt(node.FillOpacity255),
				expectedVisibilityCell(node.Visible),
			})
			if node.Children != nil {
				walk(*node.Children, depth+1)
			}
		}
	}
	walk(layers, 0)
	return renderRows(rows)
}

// UnifiedDiff renders a line diff of two trees, "-" for want and "+" for got.
// Above maxDiffLines it falls back to printing both sides whole: a quadratic
// LCS over a huge tree costs more than it explains.
func UnifiedDiff(want, got string) string {
	if want == got {
		return ""
	}
	a := splitLines(want)
	b := splitLines(got)
	if len(a)+len(b) > maxDiffLines {
		return "tree (want):\n" + want + "\ntree (got):\n" + got
	}

	lengths := lcsTable(a, b)
	var out strings.Builder
	out.WriteString("tree diff (-want +got):\n")
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out.WriteString("  " + a[i] + "\n")
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]:
			out.WriteString("- " + a[i] + "\n")
			i++
		default:
			out.WriteString("+ " + b[j] + "\n")
			j++
		}
	}
	for ; i < len(a); i++ {
		out.WriteString("- " + a[i] + "\n")
	}
	for ; j < len(b); j++ {
		out.WriteString("+ " + b[j] + "\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// lcsTable builds the suffix-LCS length table: lengths[i][j] is the length of
// the longest common subsequence of a[i:] and b[j:].
func lcsTable(a, b []string) [][]int {
	lengths := make([][]int, len(a)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
				continue
			}
			lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
		}
	}
	return lengths
}

func splitLines(text string) []string {
	trimmed := strings.TrimRight(text, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// renderRows pads every column to the widest cell so that the two trees line up
// character for character; otherwise a one-character name change would shift
// every later column and the diff would flag lines that did not change.
func renderRows(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var out strings.Builder
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(cell)
			if pad := widths[i] - len(cell); pad > 0 {
				line.WriteString(strings.Repeat(" ", pad))
			}
		}
		out.WriteString(strings.TrimRight(line.String(), " "))
		out.WriteString("\n")
	}
	return out.String()
}

// boundsCell renders a node's rectangle, or "-" for the node kinds that have
// none (groups and adjustment layers are unbounded in the engine model).
func boundsCell(node model.LayerNode) string {
	bounds, ok := nodeBounds(node)
	if !ok {
		return "-"
	}
	return fmtBounds(bounds)
}

func expectedBoundsCell(node LayerExpect) string {
	if node.Bounds != nil {
		return fmtBounds(node.Bounds.Bounds())
	}
	// A group or adjustment layer has no bounds to assert, so "?" would be
	// noise: render the same "-" the actual tree renders.
	if node.Type != nil {
		switch model.LayerType(*node.Type) {
		case model.LayerTypeGroup, model.LayerTypeAdjustment:
			return "-"
		}
	}
	return "?"
}

func visibilityCell(visible bool) string {
	if visible {
		return "vis"
	}
	return "hid"
}

func expectedVisibilityCell(visible *bool) string {
	if visible == nil {
		return "?"
	}
	return visibilityCell(*visible)
}

func expectedName(node LayerExpect) string {
	if node.Name != nil {
		return *node.Name
	}
	if node.Path != "" {
		segments := strings.Split(node.Path, "/")
		return segments[len(segments)-1]
	}
	return "?"
}

func optionalString(value *string) string {
	if value == nil {
		return "?"
	}
	return *value
}

func optionalInt(value *int) string {
	if value == nil {
		return "?"
	}
	return fmtInt(*value)
}
