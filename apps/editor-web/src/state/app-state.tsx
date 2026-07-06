import type { PropsWithChildren } from "react";
import { BrushStateProvider } from "./brush-state";
import { ColorStateProvider } from "./color-state";
import { FillGradientStateProvider } from "./fill-gradient-state";
import { SelectionToolStateProvider } from "./selection-tool-state";
import { ShapeStateProvider } from "./shape-state";
import { ViewStateProvider } from "./view-state";

/**
 * Composes all domain state providers in dependency order.
 *
 * Must be mounted inside <EngineProvider>: the domain providers call
 * useEngine() for domain-local effects (e.g. foreground/background color
 * sync to the engine).
 *
 * FillGradientStateProvider reads useColorState() for its default gradient
 * stops, so it must stay nested inside ColorStateProvider.
 */
export function AppStateProvider({ children }: PropsWithChildren) {
  return (
    <ColorStateProvider>
      <BrushStateProvider>
        <FillGradientStateProvider>
          <ShapeStateProvider>
            <SelectionToolStateProvider>
              <ViewStateProvider>{children}</ViewStateProvider>
            </SelectionToolStateProvider>
          </ShapeStateProvider>
        </FillGradientStateProvider>
      </BrushStateProvider>
    </ColorStateProvider>
  );
}
