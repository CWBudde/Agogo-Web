import { CommandID } from "@agogo/proto";
import { hexToRgba, rgbaToHex } from "@/lib/color";
import { findLayerMetaInTree } from "@/lib/layer-tree";
import { type ArtboardPreset, useShapeState } from "@/state/shape-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";
import { ToolChoiceButton, ToolOptionGroup } from "./controls";

export const artboardPresetMap: Record<
  Exclude<ArtboardPreset, "custom">,
  { width: number; height: number; label: string }
> = {
  hd: { width: 1920, height: 1080, label: "HD" },
  iphone: { width: 1179, height: 2556, label: "iPhone" },
  ipad: { width: 1668, height: 2388, label: "iPad" },
  a4: { width: 2480, height: 3508, label: "A4" },
};

export function ArtboardOptions() {
  const engine = useEngine();
  const layers = useUiMeta((meta) => meta?.layers);
  const activeLayerId = useUiMeta((meta) => meta?.activeLayerId);
  const { artboardPreset, setArtboardPreset, artboardBackground, setArtboardBackground } =
    useShapeState();

  const activeArtboard =
    activeLayerId && layers ? findLayerMetaInTree(layers, activeLayerId) : null;
  const artboardPresetSize = artboardPreset === "custom" ? null : artboardPresetMap[artboardPreset];

  return (
    <>
      <ToolOptionGroup label="Preset">
        <ToolChoiceButton
          active={artboardPreset === "custom"}
          onClick={() => setArtboardPreset("custom")}
        >
          Custom
        </ToolChoiceButton>
        {(
          Object.entries(artboardPresetMap) as Array<
            [Exclude<ArtboardPreset, "custom">, { width: number; height: number; label: string }]
          >
        ).map(([presetId, preset]) => (
          <ToolChoiceButton
            key={presetId}
            active={artboardPreset === presetId}
            onClick={() => setArtboardPreset(presetId)}
          >
            {preset.label}
          </ToolChoiceButton>
        ))}
      </ToolOptionGroup>
      {artboardPresetSize ? (
        <span className="text-[11px] text-slate-400">
          {artboardPresetSize.width} × {artboardPresetSize.height}
        </span>
      ) : (
        <span className="text-[11px] text-slate-400">Drag freely to size the artboard.</span>
      )}
      <div className="flex items-center gap-2 text-[11px] text-slate-400">
        <span className="shrink-0 uppercase tracking-[0.18em] text-slate-500">BG</span>
        <input
          type="color"
          className="h-6 w-8 rounded border border-white/10 bg-transparent"
          value={rgbaToHex(artboardBackground)}
          onChange={(event) => {
            const next = hexToRgba(event.target.value);
            if (!next) {
              return;
            }
            setArtboardBackground([...next]);
            if (activeArtboard?.isArtboard && activeArtboard.artboardBounds) {
              engine.dispatchCommand(CommandID.SetArtboard, {
                layerId: activeArtboard.id,
                bounds: activeArtboard.artboardBounds,
                background: next,
              });
            }
          }}
        />
        <span>{rgbaToHex(artboardBackground).toUpperCase()}</span>
      </div>
    </>
  );
}
