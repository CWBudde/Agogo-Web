import type { PropsWithChildren } from "react";
import { BrushStateProvider } from "./brush-state";
import { ColorStateProvider } from "./color-state";

/**
 * Composes all domain state providers in dependency order.
 *
 * Must be mounted inside <EngineProvider>: the domain providers call
 * useEngine() for domain-local effects (e.g. foreground/background color
 * sync to the engine).
 */
export function AppStateProvider({ children }: PropsWithChildren) {
  return (
    <ColorStateProvider>
      <BrushStateProvider>{children}</BrushStateProvider>
    </ColorStateProvider>
  );
}
