import { CommandID } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FadeDialog } from "./fade-dialog";

function createEngine() {
  return { dispatchCommand: vi.fn(() => null) };
}

describe("FadeDialog", () => {
  it("fades the last filter at full opacity in Normal mode by default", () => {
    const engine = createEngine();
    const onFaded = vi.fn();
    const onClose = vi.fn();
    render(
      <FadeDialog engine={engine} filterName="Gaussian Blur" onClose={onClose} onFaded={onFaded} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.FadeFilter, {
      opacity: 100,
      blendMode: "normal",
    });
    expect(onFaded).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("fades with the chosen opacity and blend mode", () => {
    const engine = createEngine();
    render(<FadeDialog engine={engine} filterName="Twirl" onClose={vi.fn()} onFaded={vi.fn()} />);

    fireEvent.change(screen.getByRole("spinbutton"), { target: { value: "60" } });
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "multiply" } });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.FadeFilter, {
      opacity: 60,
      blendMode: "multiply",
    });
  });

  it("does not fade when cancelled", () => {
    const engine = createEngine();
    const onClose = vi.fn();
    render(<FadeDialog engine={engine} filterName="Twirl" onClose={onClose} onFaded={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(engine.dispatchCommand).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
