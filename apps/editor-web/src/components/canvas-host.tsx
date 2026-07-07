import { CommandID } from "@agogo/proto";
import { memo, useCallback, useMemo } from "react";
import { EditorCanvas } from "@/components/editor-canvas";
import { artboardPresetMap } from "@/components/tool-options/artboard-options";
import { useBrushState } from "@/state/brush-state";
import { useColorState } from "@/state/color-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { useShapeState } from "@/state/shape-state";
import { useToolState } from "@/state/tool-state";
import { useCursorState, useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";

const MemoEditorCanvas = memo(EditorCanvas);

/**
 * Wraps the display-only EditorCanvas with a memoized boundary. All ~70 props
 * are sourced from the domain state hooks here and stabilized (useMemo for the
 * option bundles + sampler mapping, useCallback for inline closures) so that
 * unrelated App-level state changes (dialogs, doc-io, panels) do not re-render
 * the canvas.
 */
export function CanvasHost() {
  const engine = useEngine();
  const render = engine.render;
  const { activeTool } = useToolState();
  const { isPanMode, selectedLayerIds } = useViewState();
  const { setCursor } = useCursorState();
  const {
    foregroundColor,
    setForegroundColor,
    setBackgroundColor,
    eyedropperSampleSize,
    eyedropperSampleMerged,
    eyedropperSampleAllLayersNoAdj,
    colorSamplerPoints,
    addColorSamplerPoint,
  } = useColorState();
  const {
    brushSize,
    brushHardness,
    brushFlow,
    brushOpacity,
    brushBlendMode,
    brushAirbrush,
    brushSmoothing,
    pressureAffectsSize,
    pressureAffectsOpacity,
    pressureAffectsFlow,
    mixerBrushWetness,
    mixerBrushLoad,
    mixerBrushSampleMerged,
    cloneStampOpacity,
    cloneStampLoad,
    cloneStampAligned,
    cloneStampAlignedOffset,
    cloneStampSampleMerged,
    cloneStampSource,
    setCloneStampSource,
    setCloneStampAlignedOffset,
    cloneStampUseHistorySource,
    cloneStampHistorySourceIndex,
    historyBrushOpacity,
    historyBrushLoad,
    historyBrushSourceIndex,
    historyBrushSampleMerged,
    pencilAutoErase,
    eraserMode,
    eraserTolerance,
  } = useBrushState();
  const {
    fillSource,
    fillPatternId,
    fillTolerance,
    fillContiguous,
    fillSampleMerged,
    fillCreateLayer,
    gradientType,
    gradientReverse,
    gradientDither,
    gradientCreateLayer,
    gradientStops,
  } = useFillGradientState();
  const {
    shapeSubTool,
    shapeMode,
    shapeCornerRadius,
    shapePolygonSides,
    shapePolygonInnerRadiusPct,
    shapeStarMode,
    selectedShapePreset,
    shapeFillColor,
    shapeStrokeColor,
    shapeStrokeWidth,
    artboardPreset,
    artboardBackground,
  } = useShapeState();
  const {
    marqueeShape,
    marqueeStyle,
    marqueeRatioW,
    marqueeRatioH,
    marqueeSizeW,
    marqueeSizeH,
    lassoMode,
    selectionAntiAlias,
    selectionFeatherRadius,
    wandMode,
    wandTolerance,
    wandContiguous,
    wandSampleMerged,
    moveAutoSelectGroup,
    cropDeletePixels,
    cropContentAwareFill,
    cropResolution,
    cropOverlayType,
    cropStraightenActive,
    setCropStraightenActive,
    transformSelectionActive,
    setTransformSelectionActive,
  } = useSelectionToolState();

  const historyEntries = render?.uiMeta.history ?? [];
  const selectedHistoryBrushEntry =
    historyBrushSourceIndex === null
      ? null
      : (historyEntries.find((entry) => entry.id === historyBrushSourceIndex) ?? null);
  const historyBrushSourceLabel = selectedHistoryBrushEntry?.description ?? null;

  const selectionOptions = useMemo(
    () => ({
      marqueeShape,
      marqueeStyle,
      marqueeRatioW,
      marqueeRatioH,
      marqueeSizeW,
      marqueeSizeH,
      lassoMode,
      antiAlias: selectionAntiAlias,
      featherRadius: selectionFeatherRadius,
      wandMode,
      wandTolerance,
      wandContiguous,
      wandSampleMerged,
    }),
    [
      marqueeShape,
      marqueeStyle,
      marqueeRatioW,
      marqueeRatioH,
      marqueeSizeW,
      marqueeSizeH,
      lassoMode,
      selectionAntiAlias,
      selectionFeatherRadius,
      wandMode,
      wandTolerance,
      wandContiguous,
      wandSampleMerged,
    ],
  );

  const shapeOptions = useMemo(
    () => ({
      subTool: shapeSubTool,
      mode: shapeMode,
      cornerRadius: shapeCornerRadius,
      polygonSides: shapePolygonSides,
      polygonInnerRadiusPct: shapePolygonInnerRadiusPct,
      starMode: shapeStarMode,
      customPreset: selectedShapePreset,
      fillColor: shapeFillColor,
      strokeColor: shapeStrokeColor,
      strokeWidth: shapeStrokeWidth,
    }),
    [
      shapeSubTool,
      shapeMode,
      shapeCornerRadius,
      shapePolygonSides,
      shapePolygonInnerRadiusPct,
      shapeStarMode,
      selectedShapePreset,
      shapeFillColor,
      shapeStrokeColor,
      shapeStrokeWidth,
    ],
  );

  const artboardOptions = useMemo(
    () => ({
      presetSize: artboardPreset === "custom" ? null : artboardPresetMap[artboardPreset],
      background: artboardBackground,
    }),
    [artboardPreset, artboardBackground],
  );

  const colorSamplerPointsMapped = useMemo(
    () =>
      colorSamplerPoints.map((point, index) => ({
        id: point.id,
        label: String(index + 1),
        x: point.x,
        y: point.y,
        color: point.color,
      })),
    [colorSamplerPoints],
  );

  const onCloneStampSourceChange = useCallback(
    (source: { x: number; y: number } | null) => {
      setCloneStampSource(source);
      setCloneStampAlignedOffset(null);
    },
    [setCloneStampSource, setCloneStampAlignedOffset],
  );

  const onTransformSelectionCommit = useCallback(
    (a: number, b: number, c: number, d: number, tx: number, ty: number) => {
      engine.dispatchCommand(CommandID.TransformSelection, { a, b, c, d, tx, ty });
      setTransformSelectionActive(false);
    },
    [engine.dispatchCommand, setTransformSelectionActive],
  );

  const onTransformSelectionCancel = useCallback(
    () => setTransformSelectionActive(false),
    [setTransformSelectionActive],
  );

  return (
    <MemoEditorCanvas
      activeTool={activeTool}
      isPanMode={isPanMode || activeTool === "hand"}
      isZoomTool={activeTool === "zoom"}
      selectionOptions={selectionOptions}
      moveAutoSelectGroup={moveAutoSelectGroup}
      selectedLayerIds={selectedLayerIds}
      onCursorChange={setCursor}
      brushSize={brushSize}
      brushHardness={brushHardness}
      brushFlow={brushFlow}
      brushOpacity={brushOpacity}
      brushBlendMode={brushBlendMode}
      brushAirbrush={brushAirbrush}
      brushSmoothing={brushSmoothing}
      pressureAffectsSize={pressureAffectsSize}
      pressureAffectsOpacity={pressureAffectsOpacity}
      pressureAffectsFlow={pressureAffectsFlow}
      mixerBrushWetness={mixerBrushWetness}
      mixerBrushLoad={mixerBrushLoad}
      mixerBrushSampleMerged={mixerBrushSampleMerged}
      cloneStampOpacity={cloneStampOpacity}
      cloneStampLoad={cloneStampLoad}
      cloneStampAligned={cloneStampAligned}
      cloneStampAlignedOffset={cloneStampAlignedOffset}
      cloneStampSampleMerged={cloneStampSampleMerged}
      cloneStampSource={cloneStampSource}
      onCloneStampSourceChange={onCloneStampSourceChange}
      onCloneStampAlignedOffsetChange={setCloneStampAlignedOffset}
      cloneStampUseHistorySource={cloneStampUseHistorySource}
      cloneStampHistorySourceIndex={cloneStampHistorySourceIndex}
      historyBrushOpacity={historyBrushOpacity}
      historyBrushLoad={historyBrushLoad}
      historyBrushSourceIndex={historyBrushSourceIndex}
      historyBrushSourceLabel={historyBrushSourceLabel}
      historyBrushSampleMerged={historyBrushSampleMerged}
      pencilAutoErase={pencilAutoErase}
      eraserMode={eraserMode}
      eraserTolerance={eraserTolerance}
      foregroundColor={foregroundColor}
      onForegroundColorChange={setForegroundColor}
      onBackgroundColorChange={setBackgroundColor}
      fillSource={fillSource}
      fillPatternId={fillPatternId}
      fillTolerance={fillTolerance}
      fillContiguous={fillContiguous}
      fillSampleMerged={fillSampleMerged}
      fillCreateLayer={fillCreateLayer}
      gradientType={gradientType}
      gradientReverse={gradientReverse}
      gradientDither={gradientDither}
      gradientCreateLayer={gradientCreateLayer}
      gradientStops={gradientStops}
      eyedropperSampleSize={eyedropperSampleSize}
      eyedropperSampleMerged={eyedropperSampleMerged}
      eyedropperSampleAllLayersNoAdj={eyedropperSampleAllLayersNoAdj}
      colorSamplerPoints={colorSamplerPointsMapped}
      onColorSamplerAdd={addColorSamplerPoint}
      shapeOptions={shapeOptions}
      artboardOptions={artboardOptions}
      cropDeletePixels={cropDeletePixels}
      cropContentAwareFill={cropContentAwareFill}
      cropResolution={cropResolution}
      cropOverlayType={cropOverlayType}
      cropStraightenActive={cropStraightenActive}
      onCropStraightenActiveChange={setCropStraightenActive}
      transformSelectionActive={transformSelectionActive}
      onTransformSelectionCommit={onTransformSelectionCommit}
      onTransformSelectionCancel={onTransformSelectionCancel}
    />
  );
}
