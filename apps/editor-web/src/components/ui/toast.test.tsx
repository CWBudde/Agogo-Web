import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastViewport } from "@/components/ui/toast";
import { emitToast } from "@/lib/toast-bus";

describe("ToastViewport", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders an emitted toast with title and message", () => {
    render(<ToastViewport />);

    act(() => {
      emitToast({ kind: "info", title: "Saved", message: "Project stored locally." });
    });

    expect(screen.getByText("Saved")).toBeTruthy();
    expect(screen.getByText("Project stored locally.")).toBeTruthy();
  });

  it("auto-dismisses after the default 6000ms", () => {
    render(<ToastViewport />);

    act(() => {
      emitToast({ kind: "info", message: "Short-lived." });
    });
    expect(screen.getByText("Short-lived.")).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(5999);
    });
    expect(screen.getByText("Short-lived.")).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.queryByText("Short-lived.")).toBeNull();
  });

  it("honors a custom durationMs", () => {
    render(<ToastViewport />);

    act(() => {
      emitToast({ kind: "warning", message: "Quick.", durationMs: 500 });
    });

    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(screen.queryByText("Quick.")).toBeNull();
  });

  it("caps the stack at four toasts, dropping the oldest", () => {
    render(<ToastViewport />);

    act(() => {
      for (let i = 1; i <= 5; i += 1) {
        emitToast({ kind: "info", message: `Toast ${i}` });
      }
    });

    expect(screen.queryByText("Toast 1")).toBeNull();
    expect(screen.getByText("Toast 2")).toBeTruthy();
    expect(screen.getByText("Toast 5")).toBeTruthy();
    expect(screen.getAllByRole("status")).toHaveLength(4);
  });

  it("uses aria-live assertive for errors and polite otherwise", () => {
    render(<ToastViewport />);

    act(() => {
      emitToast({ kind: "error", message: "Engine exploded." });
      emitToast({ kind: "info", message: "All fine." });
    });

    const statuses = screen.getAllByRole("status");
    expect(statuses).toHaveLength(2);
    const [errorToast, infoToast] = statuses;
    expect(errorToast.getAttribute("aria-live")).toBe("assertive");
    expect(infoToast.getAttribute("aria-live")).toBe("polite");
  });

  it("dismisses a toast via its dismiss button", () => {
    render(<ToastViewport />);

    act(() => {
      emitToast({ kind: "info", message: "Dismiss me." });
    });

    const dismiss = screen.getByRole("button", { name: /dismiss/iu });
    act(() => {
      dismiss.click();
    });
    expect(screen.queryByText("Dismiss me.")).toBeNull();

    // The auto-dismiss timer of the removed toast must not fire on a stale
    // entry (nothing to assert beyond "no crash").
    act(() => {
      vi.advanceTimersByTime(10000);
    });
  });
});
