import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Dialog } from "@/components/ui/dialog";

afterEach(() => {
  cleanup();
});

describe("Dialog", () => {
  it("calls onClose when Escape is pressed inside the dialog", () => {
    const onClose = vi.fn();
    render(
      <Dialog open title="Escape Test" onClose={onClose}>
        <button type="button">Ok</button>
      </Dialog>,
    );

    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("stops Escape from propagating past the dialog", () => {
    const onClose = vi.fn();
    const outerKeyDown = vi.fn();
    render(
      // biome-ignore lint/a11y/noStaticElementInteractions: test-only listener to observe propagation
      <div onKeyDown={outerKeyDown}>
        <Dialog open title="Propagation" onClose={onClose}>
          <button type="button">Ok</button>
        </Dialog>
      </div>,
    );

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(outerKeyDown).not.toHaveBeenCalled();
  });

  it("skips invisible elements for initial focus", () => {
    render(
      <Dialog open title="Hidden">
        <input type="text" style={{ display: "none" }} aria-label="hidden input" />
        <button type="button">Visible</button>
      </Dialog>,
    );

    expect(document.activeElement).toBe(screen.getByText("Visible"));
  });

  it("keeps role=dialog and aria-modal=true (keyboard-shortcut modal guard)", () => {
    render(
      <Dialog open title="Guard">
        <button type="button">Ok</button>
      </Dialog>,
    );

    expect(document.querySelector('[role="dialog"][aria-modal="true"]')).not.toBeNull();
  });

  it("focuses the first focusable element on open", () => {
    render(
      <Dialog open title="Focus Test">
        <button type="button">First</button>
        <button type="button">Second</button>
      </Dialog>,
    );

    expect(document.activeElement).toBe(screen.getByText("First"));
  });

  it("focuses the dialog itself when there is nothing focusable", () => {
    render(
      <Dialog open title="Empty">
        <p>Just text</p>
      </Dialog>,
    );

    expect(document.activeElement).toBe(screen.getByRole("dialog"));
  });

  it("wraps Tab from the last focusable element to the first", () => {
    render(
      <Dialog open title="Trap">
        <button type="button">First</button>
        <button type="button">Last</button>
      </Dialog>,
    );

    const last = screen.getByText("Last");
    last.focus();
    fireEvent.keyDown(last, { key: "Tab" });
    expect(document.activeElement).toBe(screen.getByText("First"));
  });

  it("wraps Shift+Tab from the first focusable element to the last", () => {
    render(
      <Dialog open title="Trap">
        <button type="button">First</button>
        <button type="button">Last</button>
      </Dialog>,
    );

    const first = screen.getByText("First");
    first.focus();
    fireEvent.keyDown(first, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(screen.getByText("Last"));
  });

  it("restores focus to the previously focused element on close", () => {
    const trigger = document.createElement("button");
    trigger.textContent = "Open dialog";
    document.body.appendChild(trigger);
    trigger.focus();

    const { rerender } = render(
      <Dialog open title="Restore">
        <button type="button">Inside</button>
      </Dialog>,
    );
    expect(document.activeElement).toBe(screen.getByText("Inside"));

    rerender(
      <Dialog open={false} title="Restore">
        <button type="button">Inside</button>
      </Dialog>,
    );
    expect(document.activeElement).toBe(trigger);
    trigger.remove();
  });

  it("wires aria-labelledby to the rendered title and aria-describedby to the description", () => {
    render(
      <Dialog open title="My Title" description="My description">
        <button type="button">Ok</button>
      </Dialog>,
    );

    const dialog = screen.getByRole("dialog");
    const labelledBy = dialog.getAttribute("aria-labelledby");
    const describedBy = dialog.getAttribute("aria-describedby");
    expect(labelledBy).toBeTruthy();
    expect(describedBy).toBeTruthy();
    expect(document.getElementById(labelledBy as string)?.textContent).toBe("My Title");
    expect(document.getElementById(describedBy as string)?.textContent).toBe("My description");
  });

  it("omits aria-labelledby/aria-describedby when title/description are absent", () => {
    render(
      <Dialog open>
        <button type="button">Ok</button>
      </Dialog>,
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-labelledby")).toBeNull();
    expect(dialog.getAttribute("aria-describedby")).toBeNull();
  });

  it("renders nothing when closed", () => {
    render(
      <Dialog open={false} title="Hidden">
        <button type="button">Ok</button>
      </Dialog>,
    );

    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
