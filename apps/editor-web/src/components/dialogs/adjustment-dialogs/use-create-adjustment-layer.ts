import {
  type AddLayerCommand,
  type AdjustmentKind,
  type AdjustmentLayerParams,
  CommandID,
} from "@agogo/proto";
import { findLayerPositionInTree } from "@/lib/layer-tree";
import { useEngine } from "@/wasm/context";

/**
 * Returns a helper that inserts an adjustment layer directly above the active
 * layer. Shared by the adjustment dialogs and the menu (Invert) so the ABI call
 * lives in exactly one place.
 */
export function useCreateAdjustmentLayer() {
  const engine = useEngine();
  const render = engine.render;

  return <K extends AdjustmentKind>(
    name: string,
    adjustmentKind: K,
    params: AdjustmentLayerParams<K> = {} as AdjustmentLayerParams<K>,
  ) => {
    if (!render?.uiMeta.activeLayerId) {
      return;
    }
    const position = findLayerPositionInTree(render.uiMeta.layers, render.uiMeta.activeLayerId);
    if (!position) {
      return;
    }
    engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "adjustment",
      name,
      adjustmentKind,
      params,
      parentLayerId: position.parentId,
      index: position.index + 1,
    } satisfies AddLayerCommand);
  };
}
