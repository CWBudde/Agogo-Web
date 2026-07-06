package engine

import (
	"fmt"
	"strings"
	"testing"
)

// TestDispatchCommand_UnsupportedIDsReturnError verifies that the
// DispatchCommand domain switch surfaces an error instead of silently
// falling through to a normal render when a dispatcher reports
// handled == false (PLAN.md Phase S.2 item 9 — unknown command IDs used to
// be swallowed, turning frontend typos or unwired command IDs into a
// "button does nothing" bug).
//
// Command IDs for the Core, Transform, Filter, Shape, Text, and UI domains
// are densely packed in internal/command/domain.go: every numeric ID that
// DomainOf() routes to those domains already has a matching switch case in
// the respective dispatcher, so there is currently no "real" unassigned ID
// that reaches the new fallback for those six domains — the fix there is
// defensive/future-proofing. Only DomainPath (which reserves 0x0608-0x060f
// and 0x0617-0x061f between its defined command blocks) and
// DomainSelectionPaint (which reserves 0x0403-0x040f between the stroke and
// color commands) currently leave gaps that DomainOf still routes to the
// domain but no dispatcher handles. Those two are exercised as negative
// cases here; see TestDispatchCommand_KnownCommandsStillSucceed for the
// positive control covering all eight domains.
func TestDispatchCommand_UnsupportedIDsReturnError(t *testing.T) {
	unassigned := []struct {
		name      string
		commandID int32
	}{
		{"path_reservedGap", 0x0608},
		{"selectionPaint_reservedGap", 0x0403},
	}

	for _, tc := range unassigned {
		t.Run(tc.name, func(t *testing.T) {
			h := initWithDefaultDoc(t)
			defer Free(h)

			_, err := DispatchCommand(h, tc.commandID, "")
			if err == nil {
				t.Fatalf("DispatchCommand(0x%04x) = nil error, want error mentioning the command id", tc.commandID)
			}
			want := fmt.Sprintf("0x%04x", tc.commandID)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("DispatchCommand(0x%04x) error = %q, want it to contain %q", tc.commandID, err.Error(), want)
			}
		})
	}
}

// TestDispatchCommand_KnownCommandsStillSucceed is the positive control for
// every domain touched by the DispatchCommand switch fix: dispatching a
// known, legitimately-handled command ID in each domain must still succeed,
// proving the new "unsupported ... command id" guards didn't break existing
// commands that fall through to the normal render path.
func TestDispatchCommand_KnownCommandsStillSucceed(t *testing.T) {
	type colorPayload struct {
		Color [4]uint8 `json:"color"`
	}

	cases := []struct {
		name      string
		commandID int32
		payload   string
	}{
		{"core_zoomSet", commandZoomSet, mustJSON(t, ZoomPayload{Zoom: 1})},
		{"transform_cancelCrop", commandCancelCrop, "{}"},
		{"filter_cancelFilterPreview", commandCancelFilterPreview, ""},
		{"path_setActiveTool", commandSetActiveTool, mustJSON(t, SetActiveToolPayload{Tool: "pen"})},
		{"shape_drawShapePath", commandDrawShape, mustJSON(t, DrawShapePayload{
			ShapeType: "rect",
			X:         0,
			Y:         0,
			W:         10,
			H:         10,
			Mode:      "path",
		})},
		{"text_addTextLayer", commandAddTextLayer, mustJSON(t, AddTextLayerPayload{
			X:        10,
			Y:        10,
			FontSize: 24,
			Color:    [4]uint8{0, 0, 0, 255},
		})},
		{"ui_getLayerThumbnails", commandGetLayerThumbnails, ""},
		{"selectionPaint_setForegroundColor", commandSetForegroundColor, mustJSON(t, colorPayload{
			Color: [4]uint8{10, 20, 30, 255},
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := initWithDefaultDoc(t)
			defer Free(h)

			if _, err := DispatchCommand(h, tc.commandID, tc.payload); err != nil {
				t.Fatalf("DispatchCommand(%s) = %v, want no error", tc.name, err)
			}
		})
	}
}
