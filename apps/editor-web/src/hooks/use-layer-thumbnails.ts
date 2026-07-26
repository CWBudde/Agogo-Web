import { CommandID, type ThumbnailEntry } from "@agogo/proto";
import { useEffect, useState } from "react";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

/**
 * Fetches per-layer thumbnails from the engine whenever the document content
 * version changes. Local to the right dock's Layers section.
 */
export function useLayerThumbnails(): Record<string, ThumbnailEntry> {
  const engine = useEngine();
  const contentVersion = useUiMeta((meta) => meta?.contentVersion);
  const [layerThumbnails, setLayerThumbnails] = useState<Record<string, ThumbnailEntry>>({});

  useEffect(() => {
    if (contentVersion === undefined || !engine.handle) {
      return;
    }
    const result = engine.dispatchCommand(CommandID.GetLayerThumbnails);
    if (result?.thumbnails) {
      setLayerThumbnails(result.thumbnails);
    }
  }, [contentVersion, engine.dispatchCommand, engine.handle]);

  return layerThumbnails;
}
