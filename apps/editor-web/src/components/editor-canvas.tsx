import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useEngine } from "@/wasm/context";

type CursorPosition = {
  x: number;
  y: number;
} | null;

type EditorCanvasProps = {
  isPanMode: boolean;
  isZoomTool: boolean;
  onCursorChange(position: CursorPosition): void;
};

type ZoomDragState = {
  pointerId: number;
  startX: number;
  startY: number;
  startZoom: number;
  anchorX: number;
  anchorY: number;
  zoomOut: boolean;
  moved: boolean;
} | null;

function fitCanvasToElement(
  canvas: HTMLCanvasElement,
  element: HTMLElement,
  devicePixelRatio: number,
) {
  const rect = element.getBoundingClientRect();
  const width = Math.max(1, Math.floor(rect.width * devicePixelRatio));
  const height = Math.max(1, Math.floor(rect.height * devicePixelRatio));
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }
  return { width, height };
}

export function EditorCanvas({ isPanMode, isZoomTool, onCursorChange }: EditorCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const zoomDragRef = useRef<ZoomDragState>(null);
  const lastViewportRef = useRef<{
    width: number;
    height: number;
    devicePixelRatio: number;
  } | null>(null);
  const [size, setSize] = useState({ width: 1, height: 1 });
  const engine = useEngine();
  const render = engine.render;

  useLayoutEffect(() => {
    const canvas = canvasRef.current;
    const host = hostRef.current;
    if (!canvas || !host) {
      return;
    }

    const updateSize = () => {
      const devicePixelRatio = window.devicePixelRatio || 1;
      const next = fitCanvasToElement(canvas, host, devicePixelRatio);
      setSize((current) =>
        current.width === next.width && current.height === next.height ? current : next,
      );

      if (!engine.handle) {
        return;
      }

      const previousViewport = lastViewportRef.current;
      if (
        previousViewport?.width === next.width &&
        previousViewport.height === next.height &&
        previousViewport.devicePixelRatio === devicePixelRatio
      ) {
        return;
      }

      lastViewportRef.current = {
        width: next.width,
        height: next.height,
        devicePixelRatio,
      };
      engine.resizeViewport(next.width, next.height, devicePixelRatio);
    };

    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(host);

    return () => observer.disconnect();
  }, [engine.handle, engine.resizeViewport]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !engine.handle || !render || render.bufferLen === 0) {
      return;
    }

    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }

    const bytes = engine.handle.readPixels(render);
    const imageData = new ImageData(
      new Uint8ClampedArray(bytes),
      render.viewport.canvasW,
      render.viewport.canvasH,
    );
    context.putImageData(imageData, 0, 0);
    engine.handle.free(render.bufferPtr);
  }, [engine.handle, render]);

  const canvasPointFromClient = (clientX: number, clientY: number) => {
    const host = hostRef.current;
    if (!host) {
      return null;
    }

    const rect = host.getBoundingClientRect();
    const scaleX = size.width / Math.max(rect.width, 1);
    const scaleY = size.height / Math.max(rect.height, 1);
    return {
      x: (clientX - rect.left) * scaleX,
      y: (clientY - rect.top) * scaleY,
    };
  };

  const updateCursor = (clientX: number, clientY: number) => {
    const host = hostRef.current;
    if (!host || !render) {
      onCursorChange(null);
      return;
    }

    const point = canvasPointFromClient(clientX, clientY);
    if (!point) {
      onCursorChange(null);
      return;
    }
    const canvasX = point.x;
    const canvasY = point.y;

    const dx = canvasX - render.viewport.canvasW / 2;
    const dy = canvasY - render.viewport.canvasH / 2;
    const radians = (render.viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    const docX = render.viewport.centerX + (dx * cos + dy * sin) / render.viewport.zoom;
    const docY = render.viewport.centerY + (-dx * sin + dy * cos) / render.viewport.zoom;

    if (
      docX >= 0 &&
      docX < render.uiMeta.documentWidth &&
      docY >= 0 &&
      docY < render.uiMeta.documentHeight
    ) {
      onCursorChange({ x: Math.floor(docX), y: Math.floor(docY) });
      return;
    }

    onCursorChange(null);
  };

  const clientPointToDocument = (clientX: number, clientY: number) => {
    if (!render) {
      return null;
    }
    const point = canvasPointFromClient(clientX, clientY);
    if (!point) {
      return null;
    }
    const dx = point.x - render.viewport.canvasW / 2;
    const dy = point.y - render.viewport.canvasH / 2;
    const radians = (render.viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    return {
      x: render.viewport.centerX + (dx * cos + dy * sin) / render.viewport.zoom,
      y: render.viewport.centerY + (-dx * sin + dy * cos) / render.viewport.zoom,
      canvasX: point.x,
      canvasY: point.y,
    };
  };

  return (
    <div
      ref={hostRef}
      className="relative h-full min-h-[32rem] overflow-hidden rounded-[var(--ui-radius-md)] border border-white/8 bg-[#111419]"
      onPointerDown={(event) => {
        if (!render) {
          return;
        }
        const docPoint = clientPointToDocument(event.clientX, event.clientY);
        if (!docPoint) {
          return;
        }
        if (isZoomTool && !isPanMode) {
          engine.beginTransaction("Zoom viewport");
          zoomDragRef.current = {
            pointerId: event.pointerId,
            startX: event.clientX,
            startY: event.clientY,
            startZoom: render.viewport.zoom,
            anchorX: docPoint.x,
            anchorY: docPoint.y,
            zoomOut: event.altKey,
            moved: false,
          };
          event.currentTarget.setPointerCapture(event.pointerId);
          event.preventDefault();
          return;
        }
        event.currentTarget.setPointerCapture(event.pointerId);
        engine.dispatchPointerEvent({
          phase: "down",
          pointerId: event.pointerId,
          x: docPoint.canvasX,
          y: docPoint.canvasY,
          button: event.button,
          buttons: event.buttons,
          panMode: isPanMode,
        });
        event.preventDefault();
      }}
      onPointerMove={(event) => {
        updateCursor(event.clientX, event.clientY);
        const zoomDrag = zoomDragRef.current;
        if (zoomDrag && zoomDrag.pointerId === event.pointerId) {
          const deltaX = event.clientX - zoomDrag.startX;
          const deltaY = event.clientY - zoomDrag.startY;
          if (Math.abs(deltaX) > 2 || Math.abs(deltaY) > 2) {
            zoomDrag.moved = true;
          }
          const factor = 2 ** (deltaX / 180);
          const nextZoom = zoomDrag.zoomOut
            ? zoomDrag.startZoom / factor
            : zoomDrag.startZoom * factor;
          engine.setZoom(nextZoom, zoomDrag.anchorX, zoomDrag.anchorY);
          return;
        }
        const point = canvasPointFromClient(event.clientX, event.clientY);
        if (!point) {
          return;
        }
        engine.dispatchPointerEvent({
          phase: "move",
          pointerId: event.pointerId,
          x: point.x,
          y: point.y,
          button: event.button,
          buttons: event.buttons,
          panMode: isPanMode,
        });
      }}
      onPointerUp={(event) => {
        const zoomDrag = zoomDragRef.current;
        if (zoomDrag && zoomDrag.pointerId === event.pointerId) {
          if (!zoomDrag.moved) {
            const step = zoomDrag.zoomOut ? 1 / 1.25 : 1.25;
            engine.setZoom(zoomDrag.startZoom * step, zoomDrag.anchorX, zoomDrag.anchorY);
          }
          engine.endTransaction(true);
          zoomDragRef.current = null;
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        const point = canvasPointFromClient(event.clientX, event.clientY);
        if (point) {
          engine.dispatchPointerEvent({
            phase: "up",
            pointerId: event.pointerId,
            x: point.x,
            y: point.y,
            button: event.button,
            buttons: event.buttons,
            panMode: isPanMode,
          });
          event.currentTarget.releasePointerCapture(event.pointerId);
        }
      }}
      onPointerLeave={() => {
        onCursorChange(null);
      }}
      onWheel={(event) => {
        if (!render) {
          return;
        }
        event.preventDefault();
        const direction = event.deltaY > 0 ? 1 / 1.1 : 1.1;
        engine.setZoom(render.viewport.zoom * direction);
      }}
    >
      <canvas ref={canvasRef} className="absolute inset-0 h-full w-full bg-slate-950" />
      {engine.status !== "ready" ? (
        <div className="absolute inset-0 flex items-center justify-center bg-slate-950/70 p-6 backdrop-blur-sm">
          <div className="max-w-lg rounded-2xl border border-white/10 bg-slate-950/85 p-5 text-center shadow-2xl">
            <p className="text-xs uppercase tracking-[0.28em] text-slate-500">Wasm bridge</p>
            <h2 className="mt-2 text-lg font-semibold text-slate-100">
              {engine.status === "loading" ? "Loading engine" : "Engine not connected"}
            </h2>
            <p className="mt-3 text-sm leading-6 text-slate-300">
              {engine.status === "error"
                ? (engine.error?.message ?? "The Wasm engine failed to load.")
                : "The editor waits for the Go/Wasm runtime and will blit the engine output directly with putImageData."}
            </p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
