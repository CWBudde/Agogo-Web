import { GradientEditorDialog } from "@/components/gradient-editor";
import { useColorState } from "@/state/color-state";
import { useDialogState } from "@/state/dialog-state";
import { useFillGradientState } from "@/state/fill-gradient-state";
import { useCreateAdjustmentLayer } from "./use-create-adjustment-layer";

/**
 * Gradient Map adjustment dialog. Reuses the shared gradient editor bound to the
 * document gradient state and creates a gradient-map adjustment layer on apply.
 */
export function GradientMapDialog() {
  const { gradientMapDialogOpen, setGradientMapDialogOpen } = useDialogState();
  const createAdjustmentLayer = useCreateAdjustmentLayer();
  const { gradientReverse, setGradientReverse, gradientStops, setGradientStops } =
    useFillGradientState();
  const {
    recentColors,
    pushRecentColor,
    colorChannelMode,
    setColorChannelMode,
    onlyWebColors,
    setOnlyWebColors,
  } = useColorState();

  return (
    <GradientEditorDialog
      open={gradientMapDialogOpen}
      title="Gradient Map"
      description="Create an adjustment layer that remaps luminance through the current gradient."
      stops={gradientStops}
      onStopsChange={setGradientStops}
      recentColors={recentColors}
      onRecentColorSelect={pushRecentColor}
      channelMode={colorChannelMode}
      onChannelModeChange={setColorChannelMode}
      onlyWebColors={onlyWebColors}
      onOnlyWebColorsChange={setOnlyWebColors}
      reverse={gradientReverse}
      onReverseChange={setGradientReverse}
      primaryActionLabel="Create Adjustment Layer"
      onPrimaryAction={() => {
        createAdjustmentLayer("Gradient Map", "gradient-map", {
          stops: gradientStops,
          reverse: gradientReverse,
        });
        setGradientMapDialogOpen(false);
      }}
      onClose={() => setGradientMapDialogOpen(false)}
    />
  );
}
