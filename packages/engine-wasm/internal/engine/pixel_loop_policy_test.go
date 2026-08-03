package engine

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// manualPixelLoopAllowlist is the executable index for PIXEL_LOOP_POLICY.md.
// Keep the rationale here short; the policy owns the full reason of record.
var manualPixelLoopAllowlist = map[string]string{
	"adjustments.go:applyAdjustmentLayerRectToSurface":      "ticket S9-MASK-COMPOSITE: the adjustment callback stays pixel-domain; move its mask blend to CompositeImage",
	"agg_composite.go:applyBlendIfChannelsClipped":          "keep: BlendIf channel gating is a conditional channel operation, not rasterization or a comp-op",
	"brush.go:cloneStampDabResource":                        "ticket S9-BRUSH-SAMPLING: replace private bilinear source sampling while retaining dab-mask generation",
	"brush.go:eraseBackgroundDabResource":                   "keep: tolerance erase decides from each destination pixel's colour before changing alpha",
	"brush.go:paintDabClippedToSelection":                   "ticket S9-MASK-COMPOSITE: replace the post-dab premultiplied mask lerp with CompositeImage",
	"brush.go:paintSampledBrushTip":                         "keep: expands a one-channel imported tip into an AGG source image; AGG performs the transformed draw",
	"brush_resources.go:renderBrushResourceThumbnail":       "keep: expands a one-channel tip into an AGG source image; AGG performs thumbnail resampling",
	"clipboard.go:clearPixelLayerSelection":                 "keep: builds a DstOut mask and restores hidden straight-alpha RGB; AGG performs the erase",
	"crop.go:RenderCropOverlay":                             "keep: pixel-crisp interactive UI overlay, under the Phase X.7 overlay policy",
	"crop.go:buildContentAwareCropFillLayer":                "keep: classifies/clears unknown expansion cells after DrawImageAffine; no resampling remains here",
	"crop.go:diffuseCropExpansion":                          "keep: neighbourhood diffusion solver, not rasterization, compositing, or resampling",
	"crop.go:trimPixelLayerToBounds":                        "keep: exact destructive bounds clear with no interpolation",
	"fill_gradient.go:applySelectionMaskToDocBuffer":        "ticket S9-MASK-COMPOSITE: express the post-render selection alpha through an AGG mask",
	"fill_gradient.go:fillRasterWithMask":                   "keep: builds procedural source/mask buffers; CompositeImage performs the actual blend",
	"filters_builtin_blur.go:filterBoxBlur":                 "keep: separable box-filter algorithm has distinct semantics from AGG stack blur",
	"filters_builtin_blur.go:filterGaussianBlur":            "ticket S9-MASK-COMPOSITE: AGG already blurs; replace only the selection restore/lerp loop",
	"filters_builtin_blur.go:filterSurfaceBlur":             "keep: edge-aware neighbourhood filter has no public AGG equivalent",
	"filters_builtin_distort.go:filterLensCorrection":       "ticket S9-FILTER-DISTORT: needs a public nonlinear AGG span interpolator, not an affine draw",
	"filters_builtin_helpers.go:applyFilteredRGBAWithMask":  "ticket S9-MASK-COMPOSITE: composite callback output through AlphaMask/CompositeImage",
	"filters_builtin_helpers.go:applyFilteredWithMask":      "ticket S9-MASK-COMPOSITE: composite callback output through AlphaMask/CompositeImage",
	"filters_builtin_helpers.go:premultiplyRGBA":            "ticket S9-ALPHA-CONVERSION: use agg_go Image.Premultiply once filter buffers attach safely",
	"filters_builtin_helpers.go:unpremultiplyRGBA":          "ticket S9-ALPHA-CONVERSION: use agg_go Image.Demultiply once filter buffers attach safely",
	"filters_builtin_noise.go:applyMedian":                  "keep: order-statistic neighbourhood filter has no public AGG equivalent",
	"filters_builtin_noise.go:filterMaximum":                "keep: morphological maximum filter has no public AGG equivalent",
	"filters_builtin_noise.go:filterMinimum":                "keep: morphological minimum filter has no public AGG equivalent",
	"filters_builtin_noise.go:filterReduceNoise":            "keep: edge-aware denoise/chroma algorithm; its blur substep already uses AGG",
	"layer_ops.go:scaleGrayToRGBA":                          "ticket S9-IMAGE-SCALE: replace nearest resize after defining a grayscale-to-RGBA AGG adapter",
	"layer_ops.go:scaleRGBA":                                "ticket S9-IMAGE-SCALE: replace nearest resize with DrawImageAffine",
	"layer_styles_effects.go:applyGradientDitherMasked":     "ticket S9-STYLE-SPANS: fold style-gradient dithering into RenderGradient",
	"layer_styles_effects.go:renderMaskedLinearGradientLUT": "ticket S9-STYLE-SPANS: migrate the remaining style gradient to RenderGradient/GradientLUT",
	"layer_styles_effects.go:renderMaskedPatternSurface":    "ticket S9-STYLE-SPANS: add/use an AGG repeating-pattern span and mask",
	"magnetic_lasso.go:suggestMagneticPath":                 "keep: exact subimage extraction feeds AGG SobelGradient; path search is not rendering",
	"render.go:premultipliedDocumentSurface":                "keep: copies dirty rows into the cached AGG Image; Image.Premultiply performs channel conversion",
	"selection.go:cropSurfaceBounds":                        "keep: exact rectangular byte remap with no interpolation",
	"selection.go:extractSelectionFromSurface":              "keep: selection-aware sparse extraction and alpha scaling, not a rendered composite",
	"selection_overlay.go:RenderSelectionOverlay":           "keep: pixel-crisp marching-ants/view-mode UI overlay",
	"transform.go:RenderTransformHandlesOverlay":            "keep: Phase X.7 measured/visual policy keeps pixel-crisp transform UI manual",
	"transform.go:clearSelectionContent":                    "keep: exact selection-alpha mutation preserving the layer's hidden-RGB contract",
	"transform.go:extractSelectionContent":                  "keep: exact selection lift/bounds discovery, not resampling",
	"transform.go:flipPixelsH":                              "keep: exact byte remap; Phase X.7 found AGG setup adds cost without interpolation benefit",
	"transform.go:flipPixelsV":                              "keep: exact byte remap; Phase X.7 found AGG setup adds cost without interpolation benefit",
	"transform.go:maskRegionToRGBA":                         "keep: one-channel mask adapter for the shared AGG transform pipeline",
	"transform.go:mergePixelLayerOnto":                      "keep: loop only places the old surface in a union buffer; CompositeImage performs source-over",
	"transform.go:rotatePixels180":                          "keep: exact byte remap; Phase X.7 found AGG setup adds cost without interpolation benefit",
	"transform.go:rotatePixels90CCW":                        "keep: exact dimension-swapping byte remap; filtering would weaken bit parity",
	"transform.go:rotatePixels90CW":                         "keep: exact dimension-swapping byte remap; filtering would weaken bit parity",
	"transform.go:writeMaskRegionFromRGBA":                  "keep: alpha-channel adapter returning AGG-transformed mask data to its one-channel model",
	"viewport_composite.go:compositeViewportIdentity":       "keep: measured opaque 1:1 row-copy fast path; translucent source-over still delegates to CompositeImage",
}

// TestManualPixelLoopAllowlist is deliberately a lightweight syntax guard. It
// catches production functions that contain a loop and directly write three or
// four RGBA channels (including writes in a closure used by that loop). It is
// not a type-aware pixel-data-flow analysis, so reviewers must still apply the
// checklist in PIXEL_LOOP_POLICY.md to helpers and unusual indexing schemes.
func TestManualPixelLoopAllowlist(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read engine package: %v", err)
	}

	found := make(map[string]struct{})
	productionFunctions := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			key := filepath.ToSlash(name) + ":" + function.Name.Name
			productionFunctions[key] = struct{}{}
			if functionContainsDirectRGBAWrite(fset, function) {
				found[key] = struct{}{}
			}
		}
	}

	var unreviewed []string
	for key := range found {
		if _, ok := manualPixelLoopAllowlist[key]; !ok {
			unreviewed = append(unreviewed, key)
		}
	}
	sort.Strings(unreviewed)
	if len(unreviewed) != 0 {
		t.Fatalf("unreviewed direct RGBA writes; migrate them or add a reason of record to PIXEL_LOOP_POLICY.md and manualPixelLoopAllowlist:\n  %s", strings.Join(unreviewed, "\n  "))
	}

	var stale []string
	var unclassified []string
	for key, rationale := range manualPixelLoopAllowlist {
		if _, ok := productionFunctions[key]; !ok {
			stale = append(stale, key)
		}
		if !strings.HasPrefix(rationale, "keep:") && !strings.HasPrefix(rationale, "ticket ") {
			unclassified = append(unclassified, key)
		}
	}
	sort.Strings(stale)
	if len(stale) != 0 {
		t.Fatalf("stale manual-pixel-loop allowlist entries; the named production functions no longer exist:\n  %s", strings.Join(stale, "\n  "))
	}
	sort.Strings(unclassified)
	if len(unclassified) != 0 {
		t.Fatalf("manual-pixel-loop entries must be classified as keep or ticket:\n  %s", strings.Join(unclassified, "\n  "))
	}
}

func functionContainsDirectRGBAWrite(fset *token.FileSet, function *ast.FuncDecl) bool {
	hasLoop := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.ForStmt:
			hasLoop = true
		case *ast.RangeStmt:
			hasLoop = true
		}
		return !hasLoop
	})
	return hasLoop && nodeContainsDirectRGBAWrite(fset, function.Body)
}

func nodeContainsDirectRGBAWrite(fset *token.FileSet, body ast.Node) bool {
	channels := make(map[string]map[int]struct{})
	directSpanWrite := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range statement.Lhs {
				index, ok := lhs.(*ast.IndexExpr)
				if !ok {
					continue
				}
				root, channel := rgbaIndexRoot(fset, index.Index)
				key := expressionString(fset, index.X) + "|" + root
				if channels[key] == nil {
					channels[key] = make(map[int]struct{})
				}
				channels[key][channel] = struct{}{}
			}
		case *ast.CallExpr:
			identifier, ok := statement.Fun.(*ast.Ident)
			if !ok || (identifier.Name != "copy" && identifier.Name != "clear") || len(statement.Args) == 0 {
				break
			}
			slice, ok := statement.Args[0].(*ast.SliceExpr)
			if ok && rgbaSliceWidth(fset, slice) {
				directSpanWrite = true
			}
		}
		return !directSpanWrite
	})
	if directSpanWrite {
		return true
	}
	for _, offsets := range channels {
		if len(offsets) >= 3 {
			return true
		}
	}
	return false
}

func rgbaIndexRoot(fset *token.FileSet, expression ast.Expr) (string, int) {
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || (binary.Op != token.ADD && binary.Op != token.SUB) {
		return expressionString(fset, expression), 0
	}
	offset, ok := integerLiteral(binary.Y)
	if !ok || offset < 0 || offset > 3 {
		return expressionString(fset, expression), 0
	}
	if binary.Op == token.SUB {
		offset = -offset
	}
	return expressionString(fset, binary.X), offset
}

func rgbaSliceWidth(fset *token.FileSet, expression *ast.SliceExpr) bool {
	if expression.Low == nil || expression.High == nil {
		return false
	}
	high, ok := expression.High.(*ast.BinaryExpr)
	if !ok || high.Op != token.ADD || expressionString(fset, expression.Low) != expressionString(fset, high.X) {
		return false
	}
	width, ok := integerLiteral(high.Y)
	return ok && (width == 3 || width == 4)
}

func integerLiteral(expression ast.Expr) (int, bool) {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.INT {
		return 0, false
	}
	value, err := strconv.Atoi(literal.Value)
	return value, err == nil
}

func expressionString(fset *token.FileSet, expression ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fset, expression); err != nil {
		return fmt.Sprintf("%T", expression)
	}
	return buffer.String()
}
