import type { LayerNodeMeta } from "@agogo/proto";

export function findLayerMetaInTree(
  layers: LayerNodeMeta[],
  targetID: string,
): LayerNodeMeta | null {
  for (const layer of layers) {
    if (layer.id === targetID) {
      return layer;
    }
    const child = findLayerMetaInTree(layer.children ?? [], targetID);
    if (child) {
      return child;
    }
  }
  return null;
}

export function findLayerPositionInTree(
  layers: LayerNodeMeta[],
  targetID: string,
  parentId?: string,
): { parentId?: string; index: number } | null {
  for (let index = 0; index < layers.length; index++) {
    const layer = layers[index];
    if (layer.id === targetID) {
      return { parentId, index };
    }
    const child = findLayerPositionInTree(layer.children ?? [], targetID, layer.id);
    if (child) {
      return child;
    }
  }
  return null;
}
