import { CommandID, type NavigatorThumbnail } from "@agogo/proto";
import { useEffect, useMemo, useRef, useState } from "react";
import { useEngine } from "@/wasm/context";
import { useUiMeta, useViewport } from "@/wasm/use-engine-render";

function decodeRGBA(value: string): Uint8ClampedArray<ArrayBuffer> {
  const binary = atob(value);
  const result = new Uint8ClampedArray(binary.length);
  for (let index = 0; index < binary.length; index++) {
    result[index] = binary.charCodeAt(index);
  }
  return result;
}

export function NavigatorPreview() {
  const engine = useEngine();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const requestKeyRef = useRef("");
  const [size, setSize] = useState({ width: 240, height: 180 });
  const [thumbnail, setThumbnail] = useState<NavigatorThumbnail | null>(null);
  const document = useUiMeta((meta) =>
    meta
      ? {
          id: meta.activeDocumentId,
          version: meta.contentVersion,
          width: meta.documentWidth,
          height: meta.documentHeight,
        }
      : null,
  );
  const viewport = useViewport();

  useEffect(() => {
    const host = hostRef.current;
    if (!host) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) {
        return;
      }
      setSize({
        width: Math.max(1, Math.round(entry.contentRect.width)),
        height: Math.max(1, Math.round(entry.contentRect.height)),
      });
    });
    observer.observe(host);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!document?.id || !engine.handle) {
      setThumbnail(null);
      return;
    }
    const key = `${document.id}:${document.version}:${size.width}:${size.height}`;
    if (requestKeyRef.current === key) {
      return;
    }
    requestKeyRef.current = key;
    const result = engine.dispatchCommand(CommandID.GetNavigatorThumbnail, {
      width: size.width,
      height: size.height,
      background: "transparent",
    });
    const next = result?.navigatorThumbnail;
    if (next && next.documentId === document.id && next.contentVersion === document.version) {
      setThumbnail(next);
    }
  }, [document, engine, size.height, size.width]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !thumbnail) {
      return;
    }
    canvas.width = thumbnail.width;
    canvas.height = thumbnail.height;
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }
    context.putImageData(
      new ImageData(decodeRGBA(thumbnail.rgba), thumbnail.width, thumbnail.height),
      0,
      0,
    );
  }, [thumbnail]);

  const viewportPoints = useMemo(() => {
    if (!document || !viewport || !thumbnail || viewport.zoom <= 0) {
      return "";
    }
    const halfW = viewport.canvasW / 2;
    const halfH = viewport.canvasH / 2;
    const radians = (-viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    return [
      [-halfW, -halfH],
      [halfW, -halfH],
      [halfW, halfH],
      [-halfW, halfH],
    ]
      .map(([screenX, screenY]) => {
        const scaledX = screenX / viewport.zoom;
        const scaledY = screenY / viewport.zoom;
        const docX = viewport.centerX + scaledX * cos - scaledY * sin;
        const docY = viewport.centerY + scaledX * sin + scaledY * cos;
        return `${(docX / document.width) * thumbnail.width},${
          (docY / document.height) * thumbnail.height
        }`;
      })
      .join(" ");
  }, [document, thumbnail, viewport]);

  const panFromPointer = (clientX: number, clientY: number) => {
    const canvas = canvasRef.current;
    if (!canvas || !document) {
      return;
    }
    const rect = canvas.getBoundingClientRect();
    const centerX = ((clientX - rect.left) / Math.max(rect.width, 1)) * document.width;
    const centerY = ((clientY - rect.top) / Math.max(rect.height, 1)) * document.height;
    engine.dispatchCommand(CommandID.PanSet, { centerX, centerY });
  };

  return (
    <div
      ref={hostRef}
      className="relative aspect-[4/3] overflow-hidden border border-white/8 bg-background"
      onPointerDown={(event) => {
        event.currentTarget.setPointerCapture(event.pointerId);
        panFromPointer(event.clientX, event.clientY);
      }}
      onPointerMove={(event) => {
        if (event.currentTarget.hasPointerCapture(event.pointerId)) {
          panFromPointer(event.clientX, event.clientY);
        }
      }}
    >
      {thumbnail ? (
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="relative" style={{ width: thumbnail.width, height: thumbnail.height }}>
            <canvas ref={canvasRef} className="block size-full" aria-label="Document preview" />
            <svg
              className="pointer-events-none absolute inset-0 size-full overflow-visible"
              viewBox={`0 0 ${thumbnail.width} ${thumbnail.height}`}
              aria-hidden="true"
            >
              <polygon
                points={viewportPoints}
                fill="rgb(56 189 248 / 0.12)"
                stroke="rgb(56 189 248)"
                strokeWidth="1"
                vectorEffect="non-scaling-stroke"
              />
            </svg>
          </div>
        </div>
      ) : null}
    </div>
  );
}
