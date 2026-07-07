import type { EditorTool } from "@/components/tool-rail-model";
import { useSelectionToolState } from "@/state/selection-tool-state";
import { ToolChoiceButton, ToolNumberField, ToolOptionGroup } from "./controls";

export function SelectionOptions({ activeTool }: { activeTool: EditorTool }) {
  const {
    marqueeShape,
    setMarqueeShape,
    marqueeStyle,
    setMarqueeStyle,
    marqueeRatioW,
    setMarqueeRatioW,
    marqueeRatioH,
    setMarqueeRatioH,
    marqueeSizeW,
    setMarqueeSizeW,
    marqueeSizeH,
    setMarqueeSizeH,
    lassoMode,
    setLassoMode,
    selectionAntiAlias,
    setSelectionAntiAlias,
    selectionFeatherRadius,
    setSelectionFeatherRadius,
    wandMode,
    setWandMode,
    wandTolerance,
    setWandTolerance,
    wandContiguous,
    setWandContiguous,
    wandSampleMerged,
    setWandSampleMerged,
  } = useSelectionToolState();

  if (activeTool === "marquee") {
    return (
      <>
        <ToolOptionGroup label="Shape">
          <ToolChoiceButton
            active={marqueeShape === "rect"}
            onClick={() => setMarqueeShape("rect")}
          >
            Rect
          </ToolChoiceButton>
          <ToolChoiceButton
            active={marqueeShape === "ellipse"}
            onClick={() => setMarqueeShape("ellipse")}
          >
            Ellipse
          </ToolChoiceButton>
          <ToolChoiceButton active={marqueeShape === "row"} onClick={() => setMarqueeShape("row")}>
            Row
          </ToolChoiceButton>
          <ToolChoiceButton active={marqueeShape === "col"} onClick={() => setMarqueeShape("col")}>
            Col
          </ToolChoiceButton>
        </ToolOptionGroup>
        {marqueeShape === "rect" || marqueeShape === "ellipse" ? (
          <ToolOptionGroup label="Style">
            <ToolChoiceButton
              active={marqueeStyle === "normal"}
              onClick={() => setMarqueeStyle("normal")}
            >
              Normal
            </ToolChoiceButton>
            <ToolChoiceButton
              active={marqueeStyle === "fixed-ratio"}
              onClick={() => setMarqueeStyle("fixed-ratio")}
            >
              Fixed Ratio
            </ToolChoiceButton>
            <ToolChoiceButton
              active={marqueeStyle === "fixed-size"}
              onClick={() => setMarqueeStyle("fixed-size")}
            >
              Fixed Size
            </ToolChoiceButton>
          </ToolOptionGroup>
        ) : null}
        {marqueeStyle === "fixed-ratio" &&
        (marqueeShape === "rect" || marqueeShape === "ellipse") ? (
          <>
            <ToolNumberField
              label="W"
              min={0.01}
              max={9999}
              step={1}
              value={marqueeRatioW}
              onChange={setMarqueeRatioW}
            />
            <ToolNumberField
              label="H"
              min={0.01}
              max={9999}
              step={1}
              value={marqueeRatioH}
              onChange={setMarqueeRatioH}
            />
          </>
        ) : null}
        {marqueeStyle === "fixed-size" &&
        (marqueeShape === "rect" || marqueeShape === "ellipse") ? (
          <>
            <ToolNumberField
              label="W px"
              min={1}
              max={99999}
              step={1}
              value={marqueeSizeW}
              onChange={setMarqueeSizeW}
            />
            <ToolNumberField
              label="H px"
              min={1}
              max={99999}
              step={1}
              value={marqueeSizeH}
              onChange={setMarqueeSizeH}
            />
          </>
        ) : null}
        <ToolNumberField
          label="Feather"
          min={0}
          max={128}
          step={1}
          value={selectionFeatherRadius}
          onChange={setSelectionFeatherRadius}
        />
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
      </>
    );
  }

  if (activeTool === "lasso") {
    return (
      <>
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton
            active={lassoMode === "freehand"}
            onClick={() => setLassoMode("freehand")}
          >
            Freehand
          </ToolChoiceButton>
          <ToolChoiceButton
            active={lassoMode === "polygon"}
            onClick={() => setLassoMode("polygon")}
          >
            Polygon
          </ToolChoiceButton>
          <ToolChoiceButton
            active={lassoMode === "magnetic"}
            onClick={() => setLassoMode("magnetic")}
          >
            Magnetic
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Feather"
          min={0}
          max={128}
          step={1}
          value={selectionFeatherRadius}
          onChange={setSelectionFeatherRadius}
        />
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
      </>
    );
  }

  if (activeTool === "wand") {
    return (
      <>
        <ToolOptionGroup label="Mode">
          <ToolChoiceButton active={wandMode === "magic"} onClick={() => setWandMode("magic")}>
            Magic
          </ToolChoiceButton>
          <ToolChoiceButton active={wandMode === "quick"} onClick={() => setWandMode("quick")}>
            Quick
          </ToolChoiceButton>
        </ToolOptionGroup>
        <ToolNumberField
          label="Tolerance"
          min={0}
          max={255}
          step={1}
          value={wandTolerance}
          onChange={setWandTolerance}
        />
        {wandMode === "magic" ? (
          <ToolChoiceButton
            active={wandContiguous}
            onClick={() => setWandContiguous((current) => !current)}
          >
            Contiguous
          </ToolChoiceButton>
        ) : null}
        <ToolChoiceButton
          active={selectionAntiAlias}
          onClick={() => setSelectionAntiAlias((current) => !current)}
        >
          Anti-alias
        </ToolChoiceButton>
        <ToolChoiceButton
          active={wandSampleMerged}
          onClick={() => setWandSampleMerged((current) => !current)}
        >
          Sample all layers
        </ToolChoiceButton>
      </>
    );
  }

  return null;
}
