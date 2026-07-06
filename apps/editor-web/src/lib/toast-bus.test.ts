import { describe, expect, it, vi } from "vitest";
import { emitToast, subscribeToasts, type ToastOptions } from "@/lib/toast-bus";

describe("toast-bus", () => {
  it("delivers emitted toasts to a subscriber", () => {
    const received: ToastOptions[] = [];
    const unsubscribe = subscribeToasts((toast) => received.push(toast));

    emitToast({ kind: "info", message: "Saved." });
    emitToast({ kind: "error", title: "Oops", message: "Something broke.", durationMs: 1000 });

    expect(received).toEqual([
      { kind: "info", message: "Saved." },
      { kind: "error", title: "Oops", message: "Something broke.", durationMs: 1000 },
    ]);
    unsubscribe();
  });

  it("stops delivering after unsubscribe", () => {
    const received: ToastOptions[] = [];
    const unsubscribe = subscribeToasts((toast) => received.push(toast));
    unsubscribe();

    emitToast({ kind: "info", message: "Never seen." });

    expect(received).toEqual([]);
  });

  it("delivers to multiple subscribers independently", () => {
    const first: ToastOptions[] = [];
    const second: ToastOptions[] = [];
    const unsubscribeFirst = subscribeToasts((toast) => first.push(toast));
    const unsubscribeSecond = subscribeToasts((toast) => second.push(toast));

    emitToast({ kind: "warning", message: "Both see this." });
    unsubscribeFirst();
    emitToast({ kind: "warning", message: "Only second sees this." });

    expect(first).toHaveLength(1);
    expect(second).toHaveLength(2);
    unsubscribeSecond();
  });

  it("does not throw when emitting without subscribers", () => {
    expect(() => emitToast({ kind: "info", message: "Into the void." })).not.toThrow();
  });

  it("isolates a throwing subscriber from emitToast and other subscribers", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    const received: ToastOptions[] = [];
    const unsubscribeThrowing = subscribeToasts(() => {
      throw new Error("broken listener");
    });
    const unsubscribeHealthy = subscribeToasts((toast) => received.push(toast));

    expect(() => emitToast({ kind: "error", message: "Still delivered." })).not.toThrow();

    expect(received).toEqual([{ kind: "error", message: "Still delivered." }]);
    expect(consoleError).toHaveBeenCalledWith("Toast listener failed:", expect.any(Error));
    unsubscribeThrowing();
    unsubscribeHealthy();
    consoleError.mockRestore();
  });
});
