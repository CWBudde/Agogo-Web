import { CommandID } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FilterDialog } from "./filter-dialog";

function createEngine() {
  return { dispatchCommand: vi.fn((_id: number, _payload?: unknown) => null) };
}

function lastCallWith(engine: ReturnType<typeof createEngine>, commandId: number) {
  const calls = engine.dispatchCommand.mock.calls.filter((c) => c[0] === commandId);
  return calls.at(-1)?.[1] as Record<string, unknown> | undefined;
}

describe("FilterDialog", () => {
  it("renders a live preview with default params when opened", () => {
    const engine = createEngine();
    render(
      <FilterDialog
        filterId="gaussian-blur"
        engine={engine}
        onClose={vi.fn()}
        onApplied={vi.fn()}
      />,
    );

    expect(lastCallWith(engine, CommandID.PreviewFilter)).toEqual({
      filterId: "gaussian-blur",
      params: { radius: 5 },
      scale: 1,
    });
  });

  it("re-previews when a parameter changes", () => {
    const engine = createEngine();
    render(
      <FilterDialog
        filterId="gaussian-blur"
        engine={engine}
        onClose={vi.fn()}
        onApplied={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByRole("spinbutton"), { target: { value: "20" } });

    expect(lastCallWith(engine, CommandID.PreviewFilter)).toEqual({
      filterId: "gaussian-blur",
      params: { radius: 20 },
      scale: 1,
    });
  });

  it("commits the preview and reports the applied filter on Apply", () => {
    const engine = createEngine();
    const onApplied = vi.fn();
    const onClose = vi.fn();
    render(
      <FilterDialog
        filterId="gaussian-blur"
        engine={engine}
        onClose={onClose}
        onApplied={onApplied}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.CommitFilterPreview, {});
    expect(onApplied).toHaveBeenCalledWith("gaussian-blur", "Gaussian Blur");
    expect(onClose).toHaveBeenCalled();
  });

  it("cancels the preview on Cancel without committing", () => {
    const engine = createEngine();
    const onClose = vi.fn();
    render(
      <FilterDialog
        filterId="gaussian-blur"
        engine={engine}
        onClose={onClose}
        onApplied={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.CancelFilterPreview, {});
    expect(engine.dispatchCommand).not.toHaveBeenCalledWith(CommandID.CommitFilterPreview, {});
    expect(onClose).toHaveBeenCalled();
  });

  it("restores the original when preview is toggled off, and applies destructively on Apply", () => {
    const engine = createEngine();
    const onApplied = vi.fn();
    render(
      <FilterDialog
        filterId="gaussian-blur"
        engine={engine}
        onClose={vi.fn()}
        onApplied={onApplied}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "Preview" }));
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.CancelFilterPreview, {});

    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.ApplyFilter, {
      filterId: "gaussian-blur",
      params: { radius: 5 },
    });
    expect(engine.dispatchCommand).not.toHaveBeenCalledWith(CommandID.CommitFilterPreview, {});
    expect(onApplied).toHaveBeenCalledWith("gaussian-blur", "Gaussian Blur");
  });
});
