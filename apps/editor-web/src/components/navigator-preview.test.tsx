import { CommandID, type NavigatorThumbnail, type ViewportMeta } from "@agogo/proto";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NavigatorPreview } from "./navigator-preview";

const dispatchCommand = vi.fn();
let engineHandle: object | null = {};
let documentMeta: { id: string; version: number; width: number; height: number } | null;
let viewport: ViewportMeta | null;
let resizeCallback: ResizeObserverCallback | null;
const putImageData = vi.fn();

vi.mock("@/wasm/context", () => ({
  useEngine: () => ({ handle: engineHandle, dispatchCommand }),
}));

vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (
    selector: (meta: {
      activeDocumentId: string;
      contentVersion: number;
      documentWidth: number;
      documentHeight: number;
    }) => unknown,
  ) =>
    selector({
      activeDocumentId: documentMeta?.id ?? "",
      contentVersion: documentMeta?.version ?? 0,
      documentWidth: documentMeta?.width ?? 0,
      documentHeight: documentMeta?.height ?? 0,
    }),
  useViewport: () => viewport,
}));

function thumbnail(overrides: Partial<NavigatorThumbnail> = {}): NavigatorThumbnail {
  return {
    documentId: "doc-1",
    contentVersion: 7,
    requestedWidth: 240,
    requestedHeight: 180,
    width: 2,
    height: 1,
    background: "transparent",
    rgba: btoa(String.fromCharCode(1, 2, 3, 4, 5, 6, 7, 8)),
    ...overrides,
  };
}

beforeEach(() => {
  dispatchCommand.mockReset();
  engineHandle = {};
  documentMeta = { id: "doc-1", version: 7, width: 100, height: 50 };
  viewport = {
    centerX: 50,
    centerY: 25,
    zoom: 2,
    rotation: 0,
    canvasW: 40,
    canvasH: 20,
    devicePixelRatio: 1,
  };
  resizeCallback = null;
  putImageData.mockReset();

  globalThis.ResizeObserver = class {
    constructor(callback: ResizeObserverCallback) {
      resizeCallback = callback;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
  globalThis.ImageData = class {
    readonly data: Uint8ClampedArray;
    readonly width: number;
    readonly height: number;
    readonly colorSpace = "srgb";
    constructor(data: Uint8ClampedArray, width: number, height: number) {
      this.data = data;
      this.width = width;
      this.height = height;
    }
  } as unknown as typeof ImageData;
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
    putImageData,
  } as unknown as CanvasRenderingContext2D);
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.hasPointerCapture = vi.fn(() => true);
});

describe("NavigatorPreview", () => {
  it("requests once in Strict Mode and blits the engine-owned RGBA thumbnail", async () => {
    dispatchCommand.mockReturnValue({ navigatorThumbnail: thumbnail() });
    render(
      <StrictMode>
        <NavigatorPreview />
      </StrictMode>,
    );

    expect(dispatchCommand).toHaveBeenCalledTimes(1);
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.GetNavigatorThumbnail, {
      width: 240,
      height: 180,
      background: "transparent",
    });
    await waitFor(() => expect(putImageData).toHaveBeenCalledTimes(1));
    const image = putImageData.mock.calls[0][0] as ImageData;
    expect([...image.data]).toEqual([1, 2, 3, 4, 5, 6, 7, 8]);
    expect([image.width, image.height]).toEqual([2, 1]);
  });

  it("ignores a thumbnail response for an older document version", () => {
    dispatchCommand.mockReturnValue({
      navigatorThumbnail: thumbnail({ contentVersion: 6 }),
    });
    render(<NavigatorPreview />);
    expect(screen.queryByLabelText("Document preview")).toBeNull();
    expect(putImageData).not.toHaveBeenCalled();
  });

  it("refreshes for content changes and panel resizes but deduplicates unchanged keys", async () => {
    dispatchCommand.mockReturnValue({ navigatorThumbnail: thumbnail() });
    const { rerender } = render(<NavigatorPreview />);
    expect(dispatchCommand).toHaveBeenCalledTimes(1);

    rerender(<NavigatorPreview />);
    expect(dispatchCommand).toHaveBeenCalledTimes(1);

    documentMeta = { id: "doc-1", version: 8, width: 100, height: 50 };
    dispatchCommand.mockReturnValue({
      navigatorThumbnail: thumbnail({ contentVersion: 8 }),
    });
    rerender(<NavigatorPreview />);
    expect(dispatchCommand).toHaveBeenCalledTimes(2);

    act(() => {
      resizeCallback?.(
        [{ contentRect: { width: 320, height: 120 } } as ResizeObserverEntry],
        {} as ResizeObserver,
      );
    });
    await waitFor(() =>
      expect(dispatchCommand).toHaveBeenLastCalledWith(CommandID.GetNavigatorThumbnail, {
        width: 320,
        height: 120,
        background: "transparent",
      }),
    );
  });

  it("projects the visible canvas rectangle into thumbnail coordinates", () => {
    dispatchCommand.mockReturnValue({
      navigatorThumbnail: thumbnail({ width: 50, height: 25 }),
    });
    const { container } = render(<NavigatorPreview />);
    expect(container.querySelector("polygon")?.getAttribute("points")).toBe(
      "20,10 30,10 30,15 20,15",
    );
  });

  it("maps pointer position back to document coordinates for click and drag panning", () => {
    dispatchCommand.mockReturnValue({ navigatorThumbnail: thumbnail() });
    render(<NavigatorPreview />);
    const canvas = screen.getByLabelText("Document preview");
    vi.spyOn(canvas, "getBoundingClientRect").mockReturnValue({
      left: 10,
      top: 20,
      width: 200,
      height: 100,
      right: 210,
      bottom: 120,
      x: 10,
      y: 20,
      toJSON: () => ({}),
    });
    const host = canvas.parentElement?.parentElement?.parentElement;
    if (!host) {
      throw new Error("Navigator host was not rendered.");
    }

    fireEvent.pointerDown(host, { pointerId: 4, clientX: 110, clientY: 45 });
    fireEvent.pointerMove(host, { pointerId: 4, clientX: 60, clientY: 95 });

    expect(dispatchCommand).toHaveBeenNthCalledWith(2, CommandID.PanSet, {
      centerX: 50,
      centerY: 12.5,
    });
    expect(dispatchCommand).toHaveBeenNthCalledWith(3, CommandID.PanSet, {
      centerX: 25,
      centerY: 37.5,
    });
  });

  it("does not request a thumbnail without an active document or engine handle", () => {
    documentMeta = null;
    const { rerender } = render(<NavigatorPreview />);
    expect(dispatchCommand).not.toHaveBeenCalled();
    documentMeta = { id: "doc-1", version: 7, width: 100, height: 50 };
    engineHandle = null;
    rerender(<NavigatorPreview />);
    expect(dispatchCommand).not.toHaveBeenCalled();
  });
});
