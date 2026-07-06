import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EngineLoadErrorScreen } from "@/components/engine-load-error";

describe("EngineLoadErrorScreen", () => {
  it("shows the title, the error message, and a reload button", () => {
    render(<EngineLoadErrorScreen message="Failed to fetch engine.wasm (404)." />);

    expect(screen.getByText("Engine failed to load")).toBeTruthy();
    expect(screen.getByText("Failed to fetch engine.wasm (404).")).toBeTruthy();
    // jsdom's window.location.reload is non-configurable, so the click itself
    // is not exercised here — presence and labeling are what we assert.
    expect(screen.getByRole("button", { name: "Reload" })).toBeTruthy();
  });
});
