import type { PropsWithChildren } from "react";
import { BrushStateProvider } from "./brush-state";
import { ColorStateProvider } from "./color-state";
import { DialogStateProvider } from "./dialog-state";
import { FillGradientStateProvider } from "./fill-gradient-state";
import { SelectionToolStateProvider } from "./selection-tool-state";
import { ShapeStateProvider } from "./shape-state";
import { ToolStateProvider } from "./tool-state";
import { ViewStateProvider } from "./view-state";

/**
 * Composes all domain state providers in dependency order.
 *
 * Must be mounted inside <EngineProvider>: several domain providers call
 * useEngine() for domain-local effects (e.g. foreground/background color
 * sync to the engine, crop-parameter mirroring, active-layer selection sync).
 *
 * FillGradientStateProvider reads useColorState() for its default gradient
 * stops, so it must stay nested inside ColorStateProvider. The remaining
 * providers are independent of each other.
 */
export function AppStateProvider({ children }: PropsWithChildren) {
  return (
    <ColorStateProvider>
      <BrushStateProvider>
        <FillGradientStateProvider>
          <ShapeStateProvider>
            <SelectionToolStateProvider>
              <ViewStateProvider>
                <ToolStateProvider>
                  <DialogStateProvider>{children}</DialogStateProvider>
                </ToolStateProvider>
              </ViewStateProvider>
            </SelectionToolStateProvider>
          </ShapeStateProvider>
        </FillGradientStateProvider>
      </BrushStateProvider>
    </ColorStateProvider>
  );
}
