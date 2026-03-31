import { CommandID, type BeginPaintStrokeCommand, type ContinuePaintStrokeCommand, type FreeTransformMeta, type InterpolMode, type MagicEraseCommand } from "@agogo/proto";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { useEngine } from "@/wasm/context";

type CursorPosition = {
  x: number;
  y: number;
} | null;

type EditorCanvasProps = {
  activeTool:
    | "move"
    | "marquee"
    | "lasso"
    | "wand"
    | "brush"
    | "pencil"
    | "eraser"
    | "type"
    | "shape"
    | "hand"
    | "zoom"
    | "transform"
    | "crop";
  isPanMode: boolean;
  isZoomTool: boolean;
  selectionOptions: {
    marqueeShape: "rect" | "ellipse" | "row" | "col";
    marqueeStyle: "normal" | "fixed-ratio" | "fixed-size";
    marqueeRatioW: number;
    marqueeRatioH: number;
    marqueeSizeW: number;
    marqueeSizeH: number;
    lassoMode: "freehand" | "polygon" | "magnetic";
    antiAlias: boolean;
    featherRadius: number;
    wandMode: "magic" | "quick";
    wandTolerance: number;
    wandContiguous: boolean;
    wandSampleMerged: boolean;
  };
  moveAutoSelectGroup: boolean;
  selectedLayerIds: string[];
  onCursorChange(position: CursorPosition): void;
  brushSize: number;
  brushHardness: number;
  brushFlow: number;
  pencilAutoErase: boolean;
  eraserMode: "normal" | "background" | "magic";
  eraserTolerance: number;
  foregroundColor: [number, number, number, number];
  cropDeletePixels: boolean;
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

type PendingZoom = {
  zoom: number;
  anchorX: number | undefined;
  anchorY: number | undefined;
};

type DocumentPoint = {
  x: number;
  y: number;
};

type MarqueeDraft = {
  pointerId: number;
  start: DocumentPoint;
  current: DocumentPoint;
  mode: "replace" | "add" | "subtract" | "intersect";
  constrain: boolean;
};

type FreehandDraft = {
  pointerId: number;
  points: DocumentPoint[];
  mode: "replace" | "add" | "subtract" | "intersect";
};

type PolygonDraft = {
  points: DocumentPoint[];
  hoverPoint: DocumentPoint | null;
  mode: "replace" | "add" | "subtract" | "intersect";
};

type MagneticLassoDraft = {
  /** Anchor document-coordinate point (last confirmed fastening point). */
  anchorPoint: DocumentPoint;
  /** All confirmed path points in document coordinates (start → ... → last anchor). */
  confirmedPoints: DocumentPoint[];
  /** Engine-suggested path from anchor to cursor in document coordinates. */
  suggestedPath: DocumentPoint[];
  /** Start point used to detect closing proximity. */
  startPoint: DocumentPoint;
  /** Initial selection combine mode. */
  mode: "replace" | "add" | "subtract" | "intersect";
  /** Last cursor position used to throttle engine requests. */
  lastSuggestX: number;
  lastSuggestY: number;
};

type MoveDraft = {
  pointerId: number;
  layerIds: string[];
  start: DocumentPoint;
  appliedDX: number;
  appliedDY: number;
  moved: boolean;
};

type QuickSelectDraft = {
  pointerId: number;
  lastX: number;
  lastY: number;
  /** Mode to use for each drag step after the initial click. */
  dragMode: "add" | "subtract";
};

type TransformDragKind =
  | "move"
  | "scale-tl"
  | "scale-tr"
  | "scale-br"
  | "scale-bl"
  | "scale-t"
  | "scale-r"
  | "scale-b"
  | "scale-l"
  | "skew-t"
  | "skew-r"
  | "skew-b"
  | "skew-l"
  | "distort-tl"
  | "distort-tr"
  | "distort-br"
  | "distort-bl"
  | "perspective-tl"
  | "perspective-tr"
  | "perspective-br"
  | "perspective-bl"
  | `warp-${0 | 1 | 2 | 3}-${0 | 1 | 2 | 3}`
  | "rotate";

type TransformDraft = {
  pointerId: number;
  kind: TransformDragKind;
  // Snapshot of the affine matrix at drag start
  startA: number;
  startB: number;
  startC: number;
  startD: number;
  startTX: number;
  startTY: number;
  startPivotX: number;
  startPivotY: number;
  // For scale: the fixed corner in doc space (corner that stays put)
  fixedX: number;
  fixedY: number;
  // For rotation: angle from pivot to pointer at drag start (radians)
  startAngle: number;
  /** Corners at drag start (TL, TR, BR, BL in doc space). Used for distort. */
  startCorners: [[number, number], [number, number], [number, number], [number, number]];
  /** Warp grid at drag start. Present only in warp mode. */
  startWarpGrid?: [[number, number], [number, number], [number, number], [number, number]][];
};

type CropDragKind =
  | "move"
  | "rotate"
  | "scale-tl"
  | "scale-tr"
  | "scale-br"
  | "scale-bl"
  | "scale-t"
  | "scale-r"
  | "scale-b"
  | "scale-l";

type CropDraft = {
  pointerId: number;
  kind: CropDragKind;
  startDoc: DocumentPoint;
  startBox: { x: number; y: number; w: number; h: number };
  startRotation: number;      // crop rotation (degrees) when drag started
  startAngle: number;         // atan2 angle from crop center to startDoc (radians)
  cropCenterX: number;        // crop center X in doc space at drag start
  cropCenterY: number;        // crop center Y in doc space at drag start
};

function selectionModeFromModifiers(shiftKey: boolean, altKey: boolean) {
  if (shiftKey && altKey) {
    return "intersect" as const;
  }
  if (shiftKey) {
    return "add" as const;
  }
  if (altKey) {
    return "subtract" as const;
  }
  return "replace" as const;
}

function distanceSquared(a: DocumentPoint, b: DocumentPoint) {
  const dx = a.x - b.x;
  const dy = a.y - b.y;
  return dx * dx + dy * dy;
}

function buildOverlayPath(points: DocumentPoint[]) {
  if (points.length === 0) {
    return "";
  }
  const [first, ...rest] = points;
  return `M ${first.x} ${first.y} ${rest.map((point) => `L ${point.x} ${point.y}`).join(" ")} Z`;
}

function constrainedMarqueeEnd(
  start: DocumentPoint,
  current: DocumentPoint,
  constrain: boolean,
  marqueeStyle: "normal" | "fixed-ratio" | "fixed-size",
  marqueeRatioW: number,
  marqueeRatioH: number,
): DocumentPoint {
  const rawW = current.x - start.x;
  const rawH = current.y - start.y;
  if (constrain) {
    const side = Math.min(Math.abs(rawW), Math.abs(rawH));
    return {
      x: start.x + (rawW >= 0 ? side : -side),
      y: start.y + (rawH >= 0 ? side : -side),
    };
  }
  if (marqueeStyle === "fixed-ratio") {
    const ratio = marqueeRatioW / Math.max(marqueeRatioH, 0.001);
    const absW = Math.abs(rawW);
    const absH = Math.abs(rawH);
    if (absW / ratio > absH) {
      const side = absH * ratio;
      return {
        x: start.x + (rawW >= 0 ? side : -side),
        y: current.y,
      };
    }
    const side = absW / ratio;
    return {
      x: current.x,
      y: start.y + (rawH >= 0 ? side : -side),
    };
  }
  return current;
}

type LayerMetaSlim = {
  id: string;
  lockMode: string;
  layerType?: string;
  parentId?: string;
  children?: unknown[];
};

// --------------------------------------------------------------------------
// Transform helpers
// --------------------------------------------------------------------------

const TRANSFORM_HANDLE_HIT_RADIUS = 12; // canvas pixels

function transformHitTest(
  ft: FreeTransformMeta,
  docToCanvas: (d: DocumentPoint) => { x: number; y: number } | null,
  canvasX: number,
  canvasY: number,
): TransformDragKind | null {
  // Warp mode: hit-test against 4×4 control-point grid.
  if (ft.warpGrid) {
    for (let row = 0; row < 4; row++) {
      for (let col = 0; col < 4; col++) {
        const pt = ft.warpGrid[row][col];
        const cp = docToCanvas({ x: pt[0], y: pt[1] });
        if (!cp) continue;
        const dx = canvasX - cp.x;
        const dy = canvasY - cp.y;
        if (dx * dx + dy * dy <= TRANSFORM_HANDLE_HIT_RADIUS ** 2) {
          return `warp-${row as 0 | 1 | 2 | 3}-${col as 0 | 1 | 2 | 3}`;
        }
      }
    }
    // Inside warp bbox → move.
    const outerPts = [ft.warpGrid[0][0], ft.warpGrid[0][3], ft.warpGrid[3][3], ft.warpGrid[3][0]].map(
      (p) => docToCanvas({ x: p[0], y: p[1] }),
    );
    if (outerPts.every((p): p is { x: number; y: number } => p !== null)) {
      if (pointInQuad(outerPts, canvasX, canvasY)) return "move";
    }
    return null;
  }

  const corners = ft.corners;
  // 8 handles: TL, top-mid, TR, right-mid, BR, bottom-mid, BL, left-mid
  const handles: [TransformDragKind, [number, number]][] = [
    ["scale-tl", corners[0]],
    [
      "scale-t",
      [
        (corners[0][0] + corners[1][0]) * 0.5,
        (corners[0][1] + corners[1][1]) * 0.5,
      ],
    ],
    ["scale-tr", corners[1]],
    [
      "scale-r",
      [
        (corners[1][0] + corners[2][0]) * 0.5,
        (corners[1][1] + corners[2][1]) * 0.5,
      ],
    ],
    ["scale-br", corners[2]],
    [
      "scale-b",
      [
        (corners[2][0] + corners[3][0]) * 0.5,
        (corners[2][1] + corners[3][1]) * 0.5,
      ],
    ],
    ["scale-bl", corners[3]],
    [
      "scale-l",
      [
        (corners[3][0] + corners[0][0]) * 0.5,
        (corners[3][1] + corners[0][1]) * 0.5,
      ],
    ],
  ];

  // Check rotation handle first (above top-mid).
  const topMidDoc = handles[1][1];
  const topEdgeDX = corners[1][0] - corners[0][0];
  const topEdgeDY = corners[1][1] - corners[0][1];
  const topEdgeLen = Math.hypot(topEdgeDX, topEdgeDY);
  if (topEdgeLen > 0.1) {
    const perpX = -topEdgeDY / topEdgeLen;
    const perpY = topEdgeDX / topEdgeLen;
    const rot = docToCanvas({
      x: topMidDoc[0] + perpX * 24,
      y: topMidDoc[1] + perpY * 24,
    });
    if (rot) {
      const dx = canvasX - rot.x;
      const dy = canvasY - rot.y;
      if (dx * dx + dy * dy <= TRANSFORM_HANDLE_HIT_RADIUS ** 2) {
        return "rotate";
      }
    }
  }

  // Check scale handles.
  for (const [kind, docPos] of handles) {
    const cp = docToCanvas({ x: docPos[0], y: docPos[1] });
    if (!cp) continue;
    const dx = canvasX - cp.x;
    const dy = canvasY - cp.y;
    if (dx * dx + dy * dy <= TRANSFORM_HANDLE_HIT_RADIUS ** 2) {
      return kind;
    }
  }

  // Check if inside bounding box → move.
  // Use point-in-quadrilateral test (cross-product winding).
  const pts = corners.map((c) => docToCanvas({ x: c[0], y: c[1] }));
  if (pts.every((p): p is { x: number; y: number } => p !== null)) {
    if (pointInQuad(pts, canvasX, canvasY)) {
      return "move";
    }
  }

  return null;
}

function pointInQuad(
  pts: { x: number; y: number }[],
  px: number,
  py: number,
): boolean {
  let inside = false;
  const n = pts.length;
  for (let i = 0, j = n - 1; i < n; j = i++) {
    const xi = pts[i].x;
    const yi = pts[i].y;
    const xj = pts[j].x;
    const yj = pts[j].y;
    if (yi > py !== yj > py && px < ((xj - xi) * (py - yi)) / (yj - yi) + xi) {
      inside = !inside;
    }
  }
  return inside;
}

/** Returns the opposite corner (fixed point) in doc space for a scale drag. */
function oppositeCorner(
  ft: FreeTransformMeta,
  kind: TransformDragKind,
): [number, number] {
  const c = ft.corners;
  switch (kind) {
    case "scale-tl":
      return c[2]; // BR
    case "scale-tr":
      return c[3]; // BL
    case "scale-br":
      return c[0]; // TL
    case "scale-bl":
      return c[1]; // TR
    // For edge handles, return the midpoint of the opposite edge.
    case "scale-t":
      return [(c[2][0] + c[3][0]) * 0.5, (c[2][1] + c[3][1]) * 0.5];
    case "scale-b":
      return [(c[0][0] + c[1][0]) * 0.5, (c[0][1] + c[1][1]) * 0.5];
    case "scale-l":
      return [(c[1][0] + c[2][0]) * 0.5, (c[1][1] + c[2][1]) * 0.5];
    case "scale-r":
      return [(c[3][0] + c[0][0]) * 0.5, (c[3][1] + c[0][1]) * 0.5];
    default:
      return [ft.origX + ft.origW / 2, ft.origY + ft.origH / 2];
  }
}

function cropHitTest(
  crop: { x: number; y: number; w: number; h: number; rotation: number },
  docToCanvas: (d: DocumentPoint) => { x: number; y: number } | null,
  canvasX: number,
  canvasY: number,
): CropDragKind | null {
  const rotRad = (crop.rotation * Math.PI) / 180;
  const cosR = Math.cos(rotRad);
  const sinR = Math.sin(rotRad);
  const cx = crop.x + crop.w / 2;
  const cy = crop.y + crop.h / 2;

  // Rotate a crop-local offset (lx, ly) to doc space
  const rotateToDoc = (lx: number, ly: number): [number, number] => [
    cx + lx * cosR - ly * sinR,
    cy + lx * sinR + ly * cosR,
  ];

  const handles: [CropDragKind, [number, number]][] = [
    ["scale-tl", rotateToDoc(-crop.w / 2, -crop.h / 2)],
    ["scale-t",  rotateToDoc(0, -crop.h / 2)],
    ["scale-tr", rotateToDoc(crop.w / 2, -crop.h / 2)],
    ["scale-r",  rotateToDoc(crop.w / 2, 0)],
    ["scale-br", rotateToDoc(crop.w / 2, crop.h / 2)],
    ["scale-b",  rotateToDoc(0, crop.h / 2)],
    ["scale-bl", rotateToDoc(-crop.w / 2, crop.h / 2)],
    ["scale-l",  rotateToDoc(-crop.w / 2, 0)],
  ];

  for (const [kind, docPos] of handles) {
    const cp = docToCanvas({ x: docPos[0], y: docPos[1] });
    if (!cp) continue;
    const dx = canvasX - cp.x;
    const dy = canvasY - cp.y;
    if (dx * dx + dy * dy <= TRANSFORM_HANDLE_HIT_RADIUS ** 2) {
      return kind;
    }
  }

  // Inside crop box → move.
  const tlDoc = rotateToDoc(-crop.w / 2, -crop.h / 2);
  const trDoc = rotateToDoc(crop.w / 2, -crop.h / 2);
  const brDoc = rotateToDoc(crop.w / 2, crop.h / 2);
  const blDoc = rotateToDoc(-crop.w / 2, crop.h / 2);
  const tl = docToCanvas({ x: tlDoc[0], y: tlDoc[1] });
  const tr = docToCanvas({ x: trDoc[0], y: trDoc[1] });
  const br = docToCanvas({ x: brDoc[0], y: brDoc[1] });
  const bl = docToCanvas({ x: blDoc[0], y: blDoc[1] });
  if (tl && tr && br && bl) {
    if (pointInQuad([tl, tr, br, bl], canvasX, canvasY)) {
      return "move";
    }
  }

  return null;
}

function findLayerMetaByID(
  layers: Array<LayerMetaSlim>,
  targetID: string,
): LayerMetaSlim | null {
  for (const layer of layers) {
    if (layer.id === targetID) {
      return layer;
    }
    if (Array.isArray(layer.children)) {
      const child = findLayerMetaByID(
        layer.children as Array<LayerMetaSlim>,
        targetID,
      );
      if (child) {
        return child;
      }
    }
  }
  return null;
}

export function EditorCanvas({
  activeTool,
  isPanMode,
  isZoomTool,
  selectionOptions,
  moveAutoSelectGroup,
  selectedLayerIds,
  onCursorChange,
  brushSize,
  brushHardness,
  brushFlow,
  pencilAutoErase,
  eraserMode,
  eraserTolerance,
  foregroundColor,
  cropDeletePixels,
}: EditorCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const zoomDragRef = useRef<ZoomDragState>(null);
  const pendingZoomRef = useRef<PendingZoom | null>(null);
  const zoomRafRef = useRef<number | null>(null);
  const brushActiveRef = useRef(false);
  const lastViewportRef = useRef<{
    width: number;
    height: number;
    devicePixelRatio: number;
  } | null>(null);
  const [size, setSize] = useState({ width: 1, height: 1 });
  const [moveDraft, setMoveDraft] = useState<MoveDraft | null>(null);
  const [quickSelectDraft, setQuickSelectDraft] =
    useState<QuickSelectDraft | null>(null);
  const [transformDraft, setTransformDraft] =
    useState<TransformDraft | null>(null);
  const [cropDraft, setCropDraft] = useState<CropDraft | null>(null);
  const [marqueeDraft, setMarqueeDraft] = useState<MarqueeDraft | null>(null);
  const [freehandDraft, setFreehandDraft] = useState<FreehandDraft | null>(
    null,
  );
  const [polygonDraft, setPolygonDraft] = useState<PolygonDraft | null>(null);
  const [magneticLassoDraft, setMagneticLassoDraft] =
    useState<MagneticLassoDraft | null>(null);
  const engine = useEngine();
  const render = engine.render;

  // Keep a stable ref so the resize effect doesn't re-run whenever
  // engine.resizeViewport gets a new identity (it changes on every render
  // because the context useMemo depends on state.render).
  const resizeViewportRef = useRef(engine.resizeViewport);
  resizeViewportRef.current = engine.resizeViewport;

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
        current.width === next.width && current.height === next.height
          ? current
          : next,
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
      resizeViewportRef.current(next.width, next.height, devicePixelRatio);
    };

    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(host);

    return () => observer.disconnect();
  }, [engine.handle]);

  useEffect(() => {
    return () => {
      if (zoomRafRef.current !== null) {
        cancelAnimationFrame(zoomRafRef.current);
        zoomRafRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    if (activeTool !== "lasso" || selectionOptions.lassoMode !== "polygon") {
      setPolygonDraft(null);
    }
    if (activeTool !== "lasso" || selectionOptions.lassoMode !== "freehand") {
      setFreehandDraft(null);
    }
    if (activeTool !== "lasso" || selectionOptions.lassoMode !== "magnetic") {
      setMagneticLassoDraft(null);
    }
    if (activeTool !== "marquee") {
      setMarqueeDraft(null);
    }
    if (activeTool !== "wand" || selectionOptions.wandMode !== "quick") {
      setQuickSelectDraft(null);
    }
    if (activeTool !== "move") {
      setMoveDraft(null);
    }
    if (activeTool !== "transform") {
      setTransformDraft(null);
    }
    if (activeTool !== "crop") {
      setCropDraft(null);
    }
  }, [activeTool, selectionOptions.lassoMode, selectionOptions.wandMode]);

  // Once React commits a new render, if no rAF is pending the pending zoom has
  // been fully processed and render.viewport.zoom is fresh — safe to clear.
  // render is used as a change signal (not a value), so Biome's exhaustive-deps
  // rule doesn't apply here.
  // biome-ignore lint/correctness/useExhaustiveDependencies: render is an intentional change trigger
  useEffect(() => {
    if (zoomRafRef.current === null) {
      pendingZoomRef.current = null;
    }
  }, [render]);

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
    const pixelCopy = new Uint8ClampedArray(bytes.length);
    pixelCopy.set(bytes);
    const imageData = new ImageData(
      pixelCopy,
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

    const dx = canvasX - render.viewport.canvasW * 0.5;
    const dy = canvasY - render.viewport.canvasH * 0.5;
    const radians = (render.viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    const docX =
      render.viewport.centerX + (dx * cos + dy * sin) / render.viewport.zoom;
    const docY =
      render.viewport.centerY + (-dx * sin + dy * cos) / render.viewport.zoom;

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
    const dx = point.x - render.viewport.canvasW * 0.5;
    const dy = point.y - render.viewport.canvasH * 0.5;
    const radians = (render.viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    return {
      x: render.viewport.centerX + (dx * cos + dy * sin) / render.viewport.zoom,
      y:
        render.viewport.centerY + (-dx * sin + dy * cos) / render.viewport.zoom,
      canvasX: point.x,
      canvasY: point.y,
    };
  };

  const documentPointToCanvas = (docPoint: DocumentPoint) => {
    if (!render) {
      return null;
    }
    const radians = (render.viewport.rotation * Math.PI) / 180;
    const cos = Math.cos(radians);
    const sin = Math.sin(radians);
    const dx = docPoint.x - render.viewport.centerX;
    const dy = docPoint.y - render.viewport.centerY;
    return {
      x:
        render.viewport.canvasW * 0.5 +
        (dx * cos - dy * sin) * render.viewport.zoom,
      y:
        render.viewport.canvasH * 0.5 +
        (dx * sin + dy * cos) * render.viewport.zoom,
    };
  };

  const applySelectionFeather = useCallback(() => {
    if (selectionOptions.featherRadius > 0) {
      engine.dispatchCommand(CommandID.FeatherSelection, {
        radius: selectionOptions.featherRadius,
      });
    }
  }, [engine, selectionOptions.featherRadius]);

  const commitSelection = useCallback(
    (
      description: string,
      applyCommand: () => void,
      options?: { feather?: boolean },
    ) => {
      engine.beginTransaction(description);
      let committed = false;
      try {
        applyCommand();
        if (options?.feather !== false) {
          applySelectionFeather();
        }
        committed = true;
      } finally {
        engine.endTransaction(committed);
      }
    },
    [applySelectionFeather, engine],
  );

  const finalizePolygonDraft = useCallback(
    (draft: PolygonDraft) => {
      if (draft.points.length < 3) {
        return;
      }
      commitSelection("Create polygon selection", () => {
        engine.createSelection({
          shape: "polygon",
          mode: draft.mode,
          polygon: draft.points,
          antiAlias: selectionOptions.antiAlias,
        });
      });
      setPolygonDraft(null);
    },
    [commitSelection, engine, selectionOptions.antiAlias],
  );

  const finalizePolygonSelection = useCallback(() => {
    if (!polygonDraft) {
      return;
    }
    finalizePolygonDraft(polygonDraft);
  }, [finalizePolygonDraft, polygonDraft]);

  useEffect(() => {
    if (
      activeTool !== "lasso" ||
      selectionOptions.lassoMode !== "polygon" ||
      !polygonDraft
    ) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setPolygonDraft(null);
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        finalizePolygonSelection();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    activeTool,
    finalizePolygonSelection,
    polygonDraft,
    selectionOptions.lassoMode,
  ]);

  useEffect(() => {
    if (
      activeTool !== "lasso" ||
      selectionOptions.lassoMode !== "magnetic" ||
      !magneticLassoDraft
    ) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMagneticLassoDraft(null);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeTool, magneticLassoDraft, selectionOptions.lassoMode]);

  // Transform commit/cancel keyboard shortcuts.
  useEffect(() => {
    if (activeTool !== "transform" || !render?.uiMeta.freeTransform?.active) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Enter") {
        event.preventDefault();
        engine.dispatchCommand(CommandID.CommitFreeTransform);
      } else if (event.key === "Escape") {
        event.preventDefault();
        engine.dispatchCommand(CommandID.CancelFreeTransform);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeTool, engine, render?.uiMeta.freeTransform?.active]);

  // Crop commit/cancel keyboard shortcuts.
  useEffect(() => {
    if (activeTool !== "crop" || !render?.uiMeta.crop?.active) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Enter") {
        event.preventDefault();
        engine.dispatchCommand(CommandID.CommitCrop);
      } else if (event.key === "Escape") {
        event.preventDefault();
        engine.dispatchCommand(CommandID.CancelCrop);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeTool, engine, render?.uiMeta.crop?.active]);

  const marqueeStartCanvas = marqueeDraft
    ? documentPointToCanvas(marqueeDraft.start)
    : null;
  const marqueeConstrainedCurrent = marqueeDraft
    ? constrainedMarqueeEnd(
        marqueeDraft.start,
        marqueeDraft.current,
        marqueeDraft.constrain,
        selectionOptions.marqueeStyle,
        selectionOptions.marqueeRatioW,
        selectionOptions.marqueeRatioH,
      )
    : null;
  const marqueeCurrentCanvas = marqueeConstrainedCurrent
    ? documentPointToCanvas(marqueeConstrainedCurrent)
    : null;
  const marqueeOverlay =
    marqueeStartCanvas && marqueeCurrentCanvas
      ? {
          start: marqueeStartCanvas,
          current: marqueeCurrentCanvas,
        }
      : null;

  const freehandOverlay = freehandDraft
    ? freehandDraft.points
        .map((point) => documentPointToCanvas(point))
        .filter((point): point is DocumentPoint => point !== null)
    : [];

  const polygonOverlay = polygonDraft
    ? {
        points: polygonDraft.points
          .map((point) => documentPointToCanvas(point))
          .filter((point): point is DocumentPoint => point !== null),
        hoverPoint: polygonDraft.hoverPoint
          ? documentPointToCanvas(polygonDraft.hoverPoint)
          : null,
      }
    : null;

  return (
    <div
      ref={hostRef}
      className="relative h-full min-h-[32rem] overflow-hidden rounded-[var(--ui-radius-md)] border border-white/8 bg-[#111419]"
      role="application"
      aria-label="Editor canvas"
      onContextMenu={(event) => {
        if (
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "polygon" &&
          polygonDraft
        ) {
          event.preventDefault();
          finalizePolygonSelection();
        }
      }}
      onPointerDown={(event) => {
        if (!render) {
          return;
        }
        const docPoint = clientPointToDocument(event.clientX, event.clientY);
        if (!docPoint) {
          return;
        }
        if (activeTool === "marquee" && event.button === 0) {
          const marqueeMode = selectionModeFromModifiers(
            event.shiftKey,
            event.altKey,
          );
          if (selectionOptions.marqueeShape === "row") {
            commitSelection("Create row selection", () => {
              engine.createSelection({
                shape: "rect",
                mode: marqueeMode,
                rect: {
                  x: 0,
                  y: Math.floor(docPoint.y),
                  w: render.uiMeta.documentWidth,
                  h: 1,
                },
                antiAlias: false,
              });
            });
            event.preventDefault();
            return;
          }
          if (selectionOptions.marqueeShape === "col") {
            commitSelection("Create column selection", () => {
              engine.createSelection({
                shape: "rect",
                mode: marqueeMode,
                rect: {
                  x: Math.floor(docPoint.x),
                  y: 0,
                  w: 1,
                  h: render.uiMeta.documentHeight,
                },
                antiAlias: false,
              });
            });
            event.preventDefault();
            return;
          }
          if (selectionOptions.marqueeStyle === "fixed-size") {
            commitSelection("Create fixed size selection", () => {
              engine.createSelection({
                shape: selectionOptions.marqueeShape as "rect" | "ellipse",
                mode: marqueeMode,
                rect: {
                  x: Math.floor(
                    docPoint.x - selectionOptions.marqueeSizeW / 2,
                  ),
                  y: Math.floor(
                    docPoint.y - selectionOptions.marqueeSizeH / 2,
                  ),
                  w: selectionOptions.marqueeSizeW,
                  h: selectionOptions.marqueeSizeH,
                },
                antiAlias: selectionOptions.antiAlias,
              });
            });
            event.preventDefault();
            return;
          }
          setMarqueeDraft({
            pointerId: event.pointerId,
            start: { x: docPoint.x, y: docPoint.y },
            current: { x: docPoint.x, y: docPoint.y },
            mode: marqueeMode,
            constrain: event.shiftKey,
          });
          event.currentTarget.setPointerCapture(event.pointerId);
          event.preventDefault();
          return;
        }
        if (activeTool === "move" && event.button === 0 && !isPanMode) {
          const picked = engine.pickLayerAtPoint({
            x: Math.floor(docPoint.x),
            y: Math.floor(docPoint.y),
          });
          const layerId = picked?.uiMeta.activeLayerId ?? "";
          const pickedLayer = picked
            ? findLayerMetaByID(picked.uiMeta.layers, layerId)
            : null;
          if (
            !layerId ||
            pickedLayer?.lockMode === "position" ||
            pickedLayer?.lockMode === "all"
          ) {
            event.preventDefault();
            return;
          }

          // Auto-select group: if enabled and the picked layer has a parent group, select that instead.
          let effectiveLayerId = layerId;
          if (moveAutoSelectGroup && picked && pickedLayer?.parentId) {
            const parentMeta = findLayerMetaByID(picked.uiMeta.layers, pickedLayer.parentId);
            if (parentMeta?.layerType === "group") {
              effectiveLayerId = parentMeta.id;
              engine.dispatchCommand(CommandID.SetActiveLayer, { layerId: effectiveLayerId });
            }
          }

          // Move all selected layers if the picked layer is already in the selection.
          const layersToMove =
            selectedLayerIds.includes(effectiveLayerId) && selectedLayerIds.length > 1
              ? selectedLayerIds
              : [effectiveLayerId];

          engine.beginTransaction("Move layer");
          setMoveDraft({
            pointerId: event.pointerId,
            layerIds: layersToMove,
            start: { x: docPoint.x, y: docPoint.y },
            appliedDX: 0,
            appliedDY: 0,
            moved: false,
          });
          event.currentTarget.setPointerCapture(event.pointerId);
          event.preventDefault();
          return;
        }
        if (
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "freehand" &&
          event.button === 0
        ) {
          setFreehandDraft({
            pointerId: event.pointerId,
            points: [{ x: docPoint.x, y: docPoint.y }],
            mode: selectionModeFromModifiers(event.shiftKey, event.altKey),
          });
          event.currentTarget.setPointerCapture(event.pointerId);
          event.preventDefault();
          return;
        }
        if (
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "polygon"
        ) {
          if (event.button === 2 && polygonDraft) {
            event.preventDefault();
            finalizePolygonSelection();
            return;
          }
          if (event.button !== 0) {
            return;
          }
          const nextPoint = { x: docPoint.x, y: docPoint.y };
          setPolygonDraft((current) => {
            if (!current) {
              return {
                points: [nextPoint],
                hoverPoint: nextPoint,
                mode: selectionModeFromModifiers(event.shiftKey, event.altKey),
              };
            }
            if (
              current.points.length >= 3 &&
              distanceSquared(current.points[0], nextPoint) <= 100
            ) {
              queueMicrotask(() => finalizePolygonDraft(current));
              return current;
            }
            const points = [...current.points, nextPoint];
            if (event.detail >= 2 && points.length >= 3) {
              queueMicrotask(() =>
                finalizePolygonDraft({
                  ...current,
                  points,
                  hoverPoint: nextPoint,
                }),
              );
            }
            return {
              ...current,
              points,
              hoverPoint: nextPoint,
            };
          });
          event.preventDefault();
          return;
        }
        if (
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "magnetic" &&
          event.button === 0
        ) {
          const mode = selectionModeFromModifiers(event.shiftKey, event.altKey);
          if (!magneticLassoDraft) {
            setMagneticLassoDraft({
              anchorPoint: { x: docPoint.x, y: docPoint.y },
              confirmedPoints: [{ x: docPoint.x, y: docPoint.y }],
              suggestedPath: [],
              startPoint: { x: docPoint.x, y: docPoint.y },
              mode,
              lastSuggestX: Math.floor(docPoint.x),
              lastSuggestY: Math.floor(docPoint.y),
            });
          } else {
            const startCanvas = documentPointToCanvas(
              magneticLassoDraft.startPoint,
            );
            const currentCanvas = documentPointToCanvas({
              x: docPoint.x,
              y: docPoint.y,
            });
            const nearStart =
              startCanvas &&
              currentCanvas &&
              Math.abs(currentCanvas.x - startCanvas.x) < 12 &&
              Math.abs(currentCanvas.y - startCanvas.y) < 12;
            const isDoubleClick = event.detail >= 2;

            if (
              (nearStart &&
                magneticLassoDraft.confirmedPoints.length >= 3) ||
              (isDoubleClick &&
                magneticLassoDraft.confirmedPoints.length >= 3)
            ) {
              const allPoints = [
                ...magneticLassoDraft.confirmedPoints,
                ...magneticLassoDraft.suggestedPath.slice(1),
              ];
              if (allPoints.length >= 3) {
                commitSelection("Magnetic lasso selection", () => {
                  engine.createSelection({
                    shape: "polygon",
                    mode: magneticLassoDraft.mode,
                    polygon: allPoints,
                    antiAlias: selectionOptions.antiAlias,
                  });
                });
              }
              setMagneticLassoDraft(null);
            } else {
              const newConfirmed = [
                ...magneticLassoDraft.confirmedPoints,
                ...magneticLassoDraft.suggestedPath.slice(1),
              ];
              setMagneticLassoDraft((current) =>
                current
                  ? {
                      ...current,
                      anchorPoint: { x: docPoint.x, y: docPoint.y },
                      confirmedPoints: newConfirmed,
                      suggestedPath: [],
                      lastSuggestX: Math.floor(docPoint.x),
                      lastSuggestY: Math.floor(docPoint.y),
                    }
                  : current,
              );
            }
          }
          event.preventDefault();
          return;
        }
        if (activeTool === "wand" && event.button === 0) {
          if (selectionOptions.wandMode === "magic") {
            commitSelection("Magic wand selection", () => {
              engine.magicWand({
                x: Math.floor(docPoint.x),
                y: Math.floor(docPoint.y),
                tolerance: selectionOptions.wandTolerance,
                contiguous: selectionOptions.wandContiguous,
                antiAlias: selectionOptions.antiAlias,
                sampleMerged: selectionOptions.wandSampleMerged,
                mode: selectionModeFromModifiers(event.shiftKey, event.altKey),
              });
            });
          } else {
            // Quick Select: open a transaction for the whole drag gesture.
            const pixelX = Math.floor(docPoint.x);
            const pixelY = Math.floor(docPoint.y);
            engine.beginTransaction("Quick Selection");
            engine.quickSelect({
              x: pixelX,
              y: pixelY,
              tolerance: selectionOptions.wandTolerance,
              edgeSensitivity: 0.9,
              sampleMerged: selectionOptions.wandSampleMerged,
              mode: selectionModeFromModifiers(event.shiftKey, event.altKey),
            });
            setQuickSelectDraft({
              pointerId: event.pointerId,
              lastX: pixelX,
              lastY: pixelY,
              dragMode: event.altKey ? "subtract" : "add",
            });
            event.currentTarget.setPointerCapture(event.pointerId);
          }
          event.preventDefault();
          return;
        }
        if (activeTool === "transform" && event.button === 0) {
          const ft = render.uiMeta.freeTransform;
          if (ft?.active) {
            const canvasPoint = canvasPointFromClient(
              event.clientX,
              event.clientY,
            );
            if (canvasPoint) {
              let kind = transformHitTest(
                ft,
                documentPointToCanvas,
                canvasPoint.x,
                canvasPoint.y,
              );
              // Modifier+drag on handles → skew / distort / perspective.
              if (kind && event.ctrlKey) {
                if (event.shiftKey && event.altKey) {
                  // Ctrl+Shift+Alt+drag corner → perspective (symmetric trapezoid).
                  const perspRemap: Partial<Record<TransformDragKind, TransformDragKind>> = {
                    "scale-tl": "perspective-tl",
                    "scale-tr": "perspective-tr",
                    "scale-br": "perspective-br",
                    "scale-bl": "perspective-bl",
                  };
                  kind = perspRemap[kind] ?? kind;
                } else {
                  // Ctrl+drag edge → skew; Ctrl+drag corner → distort.
                  const ctrlRemap: Partial<Record<TransformDragKind, TransformDragKind>> = {
                    "scale-t": "skew-t",
                    "scale-b": "skew-b",
                    "scale-l": "skew-l",
                    "scale-r": "skew-r",
                    "scale-tl": "distort-tl",
                    "scale-tr": "distort-tr",
                    "scale-br": "distort-br",
                    "scale-bl": "distort-bl",
                  };
                  kind = ctrlRemap[kind] ?? kind;
                }
              }
              if (kind) {
                // For "move", "skew-*", "distort-*", "perspective-*", and "warp-*", fixedX/fixedY hold the initial mouse doc position.
                const isSkew = kind === "skew-t" || kind === "skew-b" || kind === "skew-l" || kind === "skew-r";
                const isDistort = kind === "distort-tl" || kind === "distort-tr" || kind === "distort-br" || kind === "distort-bl";
                const isPersp = kind === "perspective-tl" || kind === "perspective-tr" || kind === "perspective-br" || kind === "perspective-bl";
                const isWarp = kind.startsWith("warp-");
                const [fixedX, fixedY] =
                  kind === "move" || isSkew || isDistort || isPersp || isWarp
                    ? [docPoint.x, docPoint.y]
                    : oppositeCorner(ft, kind);
                const startAngle = Math.atan2(
                  docPoint.y - ft.pivotY,
                  docPoint.x - ft.pivotX,
                );
                setTransformDraft({
                  pointerId: event.pointerId,
                  kind,
                  startA: ft.a,
                  startB: ft.b,
                  startC: ft.c,
                  startD: ft.d,
                  startTX: ft.tx,
                  startTY: ft.ty,
                  startPivotX: ft.pivotX,
                  startPivotY: ft.pivotY,
                  fixedX,
                  fixedY,
                  startAngle,
                  startCorners: ft.corners,
                  startWarpGrid: ft.warpGrid ? [...ft.warpGrid] : undefined,
                });
                event.currentTarget.setPointerCapture(event.pointerId);
                event.preventDefault();
                return;
              }
            }
          } else {
            // No transform active yet — begin one on the active layer.
            engine.dispatchCommand(CommandID.BeginFreeTransform);
            event.preventDefault();
            return;
          }
        }
        if (activeTool === "crop" && event.button === 0) {
          const crop = render.uiMeta.crop;
          if (crop?.active) {
            const canvasPoint = canvasPointFromClient(event.clientX, event.clientY);
            if (canvasPoint) {
              const kind = cropHitTest(crop, documentPointToCanvas, canvasPoint.x, canvasPoint.y);
              if (kind) {
                setCropDraft({
                  pointerId: event.pointerId,
                  kind,
                  startDoc: { x: docPoint.x, y: docPoint.y },
                  startBox: { x: crop.x, y: crop.y, w: crop.w, h: crop.h },
                  startRotation: crop.rotation ?? 0,
                  startAngle: 0,
                  cropCenterX: crop.x + crop.w / 2,
                  cropCenterY: crop.y + crop.h / 2,
                });
              } else {
                // Click outside crop box — start rotation drag.
                const cropCX = crop.x + crop.w / 2;
                const cropCY = crop.y + crop.h / 2;
                setCropDraft({
                  pointerId: event.pointerId,
                  kind: "rotate",
                  startDoc: { x: docPoint.x, y: docPoint.y },
                  startBox: { x: crop.x, y: crop.y, w: crop.w, h: crop.h },
                  startRotation: crop.rotation ?? 0,
                  startAngle: Math.atan2(docPoint.y - cropCY, docPoint.x - cropCX),
                  cropCenterX: cropCX,
                  cropCenterY: cropCY,
                });
              }
              event.currentTarget.setPointerCapture(event.pointerId);
              event.preventDefault();
              return;
            }
          }
        }
        if (activeTool === "eraser" && eraserMode === "magic" && event.button === 0 && !isPanMode) {
          const docPoint = clientPointToDocument(event.clientX, event.clientY);
          if (!docPoint) return;
          event.currentTarget.setPointerCapture(event.pointerId);
          engine.dispatchCommand(CommandID.MagicErase, {
            x: docPoint.x,
            y: docPoint.y,
            tolerance: eraserTolerance,
            contiguous: true,
            sampleMerged: false,
          } satisfies MagicEraseCommand);
          return;
        }
        if ((activeTool === "brush" || activeTool === "pencil" || (activeTool === "eraser" && eraserMode !== "magic")) && event.button === 0 && !isPanMode) {
          const docPoint = clientPointToDocument(event.clientX, event.clientY);
          if (!docPoint) return;
          brushActiveRef.current = true;
          event.currentTarget.setPointerCapture(event.pointerId);
          engine.dispatchCommand(CommandID.BeginPaintStroke, {
            x: docPoint.x,
            y: docPoint.y,
            pressure: event.pressure || 0.5,
            brush: {
              size: brushSize,
              hardness: activeTool === "pencil" ? 1.0 : brushHardness,
              flow: brushFlow,
              color: foregroundColor,
              autoErase: activeTool === "pencil" ? pencilAutoErase : undefined,
              erase: activeTool === "eraser" && eraserMode === "normal" ? true : undefined,
              eraseBackground: activeTool === "eraser" && eraserMode === "background" ? true : undefined,
              eraseTolerance: activeTool === "eraser" && eraserMode === "background" ? eraserTolerance : undefined,
            },
          } satisfies BeginPaintStrokeCommand);
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
        const docPoint = clientPointToDocument(event.clientX, event.clientY);
        if (
          polygonDraft &&
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "polygon"
        ) {
          setPolygonDraft((current) =>
            current && docPoint
              ? {
                  ...current,
                  hoverPoint: { x: docPoint.x, y: docPoint.y },
                }
              : current,
          );
        }
        if (
          magneticLassoDraft &&
          activeTool === "lasso" &&
          selectionOptions.lassoMode === "magnetic" &&
          docPoint
        ) {
          const pixelX = Math.floor(docPoint.x);
          const pixelY = Math.floor(docPoint.y);
          if (
            pixelX !== magneticLassoDraft.lastSuggestX ||
            pixelY !== magneticLassoDraft.lastSuggestY
          ) {
            const result = engine.magneticLassoSuggestPath({
              x1: Math.floor(magneticLassoDraft.anchorPoint.x),
              y1: Math.floor(magneticLassoDraft.anchorPoint.y),
              x2: pixelX,
              y2: pixelY,
              sampleMerged: selectionOptions.wandSampleMerged,
            });
            const suggestedPath =
              result?.suggestedPath?.map((p) => ({ x: p.x, y: p.y })) ?? [];
            setMagneticLassoDraft((current) =>
              current
                ? {
                    ...current,
                    suggestedPath,
                    lastSuggestX: pixelX,
                    lastSuggestY: pixelY,
                  }
                : current,
            );
          }
          return;
        }
        if (
          quickSelectDraft &&
          quickSelectDraft.pointerId === event.pointerId &&
          docPoint
        ) {
          const pixelX = Math.floor(docPoint.x);
          const pixelY = Math.floor(docPoint.y);
          if (
            pixelX !== quickSelectDraft.lastX ||
            pixelY !== quickSelectDraft.lastY
          ) {
            engine.quickSelect({
              x: pixelX,
              y: pixelY,
              tolerance: selectionOptions.wandTolerance,
              edgeSensitivity: 0.9,
              sampleMerged: selectionOptions.wandSampleMerged,
              mode: quickSelectDraft.dragMode,
            });
            setQuickSelectDraft((current) =>
              current ? { ...current, lastX: pixelX, lastY: pixelY } : current,
            );
          }
          return;
        }
        if (
          transformDraft &&
          transformDraft.pointerId === event.pointerId &&
          docPoint
        ) {
          const td = transformDraft;
          const ft = render?.uiMeta.freeTransform;
          if (!ft?.active) {
            return;
          }
          let newA = td.startA;
          let newB = td.startB;
          let newC = td.startC;
          let newD = td.startD;
          let newTX = td.startTX;
          let newTY = td.startTY;

          if (td.kind === "move") {
            // The pivot stays fixed; we just translate the whole transform.
            // The drag started at the pivot so: delta = current mouse - pivot.
            // We track via startAngle field which holds the original mouse position here.
            // Instead use fixedX/fixedY which holds the initial mouse doc position.
            newTX = td.startTX + (docPoint.x - td.fixedX);
            newTY = td.startTY + (docPoint.y - td.fixedY);
          } else if (td.kind === "rotate") {
            const currentAngle = Math.atan2(
              docPoint.y - td.startPivotY,
              docPoint.x - td.startPivotX,
            );
            const da = currentAngle - td.startAngle;
            const cos = Math.cos(da);
            const sin = Math.sin(da);
            newA = cos * td.startA - sin * td.startB;
            newB = sin * td.startA + cos * td.startB;
            newC = cos * td.startC - sin * td.startD;
            newD = sin * td.startC + cos * td.startD;
            const relX = td.startTX - td.startPivotX;
            const relY = td.startTY - td.startPivotY;
            newTX = cos * relX - sin * relY + td.startPivotX;
            newTY = sin * relX + cos * relY + td.startPivotY;
          } else if (
            td.kind === "skew-t" ||
            td.kind === "skew-b" ||
            td.kind === "skew-l" ||
            td.kind === "skew-r"
          ) {
            // Skew: Ctrl+drag edge midpoint.
            // fixedX/fixedY = initial mouse doc position (= edge midpoint at drag start).
            // dx/dy = how far the edge has been dragged from its start position.
            const dx = docPoint.x - td.fixedX;
            const dy = docPoint.y - td.fixedY;
            const origW = ft.origW;
            const origH = ft.origH;
            if (td.kind === "skew-t") {
              // Top edge moves → TL and TR shift by (dx,dy); BL/BR fixed.
              // TX, TY shift with TL. A, B (X-basis) unchanged. C, D (Y-basis) adjust.
              newTX = td.startTX + dx;
              newTY = td.startTY + dy;
              // C_new = (BL_x - TX_new) / origH = (startBL_x - (startTX + dx)) / origH
              //       = startC - dx / origH
              newC = td.startC - dx / origH;
              newD = td.startD - dy / origH;
            } else if (td.kind === "skew-b") {
              // Bottom edge moves → BL and BR shift by (dx,dy); TL/TR fixed.
              // TX, TY unchanged. A, B unchanged. C, D adjust.
              // C_new = (BL_new_x - TX) / origH = (startBL_x + dx - startTX) / origH
              //       = startC + dx / origH
              newC = td.startC + dx / origH;
              newD = td.startD + dy / origH;
            } else if (td.kind === "skew-l") {
              // Left edge moves → TL and BL shift by (dx,dy); TR/BR fixed.
              // TX, TY shift with TL. C, D (Y-basis) unchanged. A, B adjust.
              newTX = td.startTX + dx;
              newTY = td.startTY + dy;
              // A_new = (TR_x - TX_new) / origW = (startTR_x - (startTX + dx)) / origW
              //       = startA - dx / origW
              newA = td.startA - dx / origW;
              newB = td.startB - dy / origW;
            } else {
              // skew-r: Right edge moves → TR and BR shift by (dx,dy); TL/BL fixed.
              // TX, TY unchanged. C, D unchanged. A, B adjust.
              // A_new = (TR_new_x - TX) / origW = (startTR_x + dx - startTX) / origW
              //       = startA + dx / origW
              newA = td.startA + dx / origW;
              newB = td.startB + dy / origW;
            }
          } else if (
            td.kind === "distort-tl" ||
            td.kind === "distort-tr" ||
            td.kind === "distort-br" ||
            td.kind === "distort-bl"
          ) {
            // Distort: Ctrl+drag corner. Move the dragged corner to mouse; others fixed.
            const cornerIndex = { "distort-tl": 0, "distort-tr": 1, "distort-br": 2, "distort-bl": 3 }[td.kind];
            const corners = td.startCorners.map((c) => [c[0], c[1]] as [number, number]) as
              [[number, number], [number, number], [number, number], [number, number]];
            corners[cornerIndex] = [docPoint.x, docPoint.y];
            engine.dispatchCommand(CommandID.UpdateFreeTransform, {
              a: td.startA, b: td.startB, c: td.startC, d: td.startD,
              tx: td.startTX, ty: td.startTY,
              pivotX: td.startPivotX, pivotY: td.startPivotY,
              interpolation: ft.interpolation as InterpolMode,
              corners,
            });
            return;
          } else if (
            td.kind === "perspective-tl" ||
            td.kind === "perspective-tr" ||
            td.kind === "perspective-br" ||
            td.kind === "perspective-bl"
          ) {
            // Perspective: Ctrl+Shift+Alt+drag corner.
            // Moving a corner also mirrors its horizontal neighbour on the same edge,
            // keeping the opposite edge stationary. This produces a trapezoid that
            // converges to a single vanishing point — the Photoshop "Perspective" mode.
            //
            // Mirror pairs (same horizontal edge):
            //   TL (0) ↔ TR (1)   — top edge
            //   BL (3) ↔ BR (2)   — bottom edge
            const mirrorIndex: Record<string, number> = {
              "perspective-tl": 1,
              "perspective-tr": 0,
              "perspective-br": 3,
              "perspective-bl": 2,
            };
            const dragIndex = { "perspective-tl": 0, "perspective-tr": 1, "perspective-br": 2, "perspective-bl": 3 }[td.kind];
            const mIdx = mirrorIndex[td.kind];
            const corners = td.startCorners.map((c) => [c[0], c[1]] as [number, number]) as
              [[number, number], [number, number], [number, number], [number, number]];

            // Delta from drag start to current mouse position.
            const dx = docPoint.x - td.fixedX;
            const dy = docPoint.y - td.fixedY;

            // Move the dragged corner.
            corners[dragIndex] = [td.startCorners[dragIndex][0] + dx, td.startCorners[dragIndex][1] + dy];
            // Mirror: negate X delta, same Y delta.
            corners[mIdx] = [td.startCorners[mIdx][0] - dx, td.startCorners[mIdx][1] + dy];

            engine.dispatchCommand(CommandID.UpdateFreeTransform, {
              a: td.startA, b: td.startB, c: td.startC, d: td.startD,
              tx: td.startTX, ty: td.startTY,
              pivotX: td.startPivotX, pivotY: td.startPivotY,
              interpolation: ft.interpolation as InterpolMode,
              corners,
            });
            return;
          } else if (td.kind.startsWith("warp-") && td.startWarpGrid) {
            // Warp: move single control point; rest stays fixed.
            const [, rowStr, colStr] = td.kind.split("-");
            const row = Number(rowStr);
            const col = Number(colStr);
            const dx = docPoint.x - td.fixedX;
            const dy = docPoint.y - td.fixedY;
            // Deep-copy the grid.
            const warpGrid = td.startWarpGrid.map((r) =>
              r.map((p) => [p[0], p[1]] as [number, number]) as [[number, number], [number, number], [number, number], [number, number]],
            ) as [[number, number], [number, number], [number, number], [number, number]][];
            warpGrid[row][col] = [td.startWarpGrid[row][col][0] + dx, td.startWarpGrid[row][col][1] + dy];
            engine.dispatchCommand(CommandID.UpdateFreeTransform, {
              a: td.startA, b: td.startB, c: td.startC, d: td.startD,
              tx: td.startTX, ty: td.startTY,
              pivotX: td.startPivotX, pivotY: td.startPivotY,
              interpolation: ft.interpolation as InterpolMode,
              warpGrid,
            });
            return;
          } else {
            // Scale from fixed corner.
            const origW = ft.origW;
            const origH = ft.origH;
            // Original dragged corner doc position.
            let origDragX: number;
            let origDragY: number;
            switch (td.kind) {
              case "scale-tl":
                origDragX = td.startTX;
                origDragY = td.startTY;
                break;
              case "scale-tr":
                origDragX = td.startA * origW + td.startTX;
                origDragY = td.startB * origW + td.startTY;
                break;
              case "scale-br":
                origDragX = td.startA * origW + td.startC * origH + td.startTX;
                origDragY = td.startB * origW + td.startD * origH + td.startTY;
                break;
              case "scale-bl":
                origDragX = td.startC * origH + td.startTX;
                origDragY = td.startD * origH + td.startTY;
                break;
              default:
                origDragX =
                  (td.fixedX + td.startA * origW + td.startTX) * 0.5;
                origDragY =
                  (td.fixedY + td.startB * origW + td.startTY) * 0.5;
            }
            const d0 = Math.hypot(
              origDragX - td.fixedX,
              origDragY - td.fixedY,
            );
            const d1 = Math.hypot(
              docPoint.x - td.fixedX,
              docPoint.y - td.fixedY,
            );
            const scale = d0 > 0.01 ? d1 / d0 : 1;
            newA = td.startA * scale;
            newB = td.startB * scale;
            newC = td.startC * scale;
            newD = td.startD * scale;
            newTX = scale * (td.startTX - td.fixedX) + td.fixedX;
            newTY = scale * (td.startTY - td.fixedY) + td.fixedY;
          }

          engine.dispatchCommand(CommandID.UpdateFreeTransform, {
            a: newA,
            b: newB,
            c: newC,
            d: newD,
            tx: newTX,
            ty: newTY,
            pivotX: td.startPivotX,
            pivotY: td.startPivotY,
            interpolation: ft.interpolation,
          });
          return;
        }
        if (
          cropDraft &&
          cropDraft.pointerId === event.pointerId &&
          docPoint
        ) {
          const cd = cropDraft;
          const dx = docPoint.x - cd.startDoc.x;
          const dy = docPoint.y - cd.startDoc.y;
          let newX = cd.startBox.x;
          let newY = cd.startBox.y;
          let newW = cd.startBox.w;
          let newH = cd.startBox.h;

          switch (cd.kind) {
            case "rotate": {
              const currentAngle = Math.atan2(
                docPoint.y - cd.cropCenterY,
                docPoint.x - cd.cropCenterX,
              );
              const angleDeltaDeg = ((currentAngle - cd.startAngle) * 180) / Math.PI;
              const newRotation = cd.startRotation + angleDeltaDeg;
              engine.dispatchCommand(CommandID.UpdateCrop, {
                x: newX,
                y: newY,
                w: newW,
                h: newH,
                rotation: newRotation,
                deletePixels: cropDeletePixels,
              });
              return;
            }
            case "move":
              newX += dx;
              newY += dy;
              break;
            case "scale-tl":
              newX += dx;
              newY += dy;
              newW -= dx;
              newH -= dy;
              break;
            case "scale-t":
              newY += dy;
              newH -= dy;
              break;
            case "scale-tr":
              newY += dy;
              newW += dx;
              newH -= dy;
              break;
            case "scale-r":
              newW += dx;
              break;
            case "scale-br":
              newW += dx;
              newH += dy;
              break;
            case "scale-b":
              newH += dy;
              break;
            case "scale-bl":
              newX += dx;
              newW -= dx;
              newH += dy;
              break;
            case "scale-l":
              newX += dx;
              newW -= dx;
              break;
          }

          // Shift-key aspect ratio constraint (corner handles only)
          const isCorner = cd.kind === "scale-tl" || cd.kind === "scale-tr" ||
                           cd.kind === "scale-br" || cd.kind === "scale-bl";
          if (event.shiftKey && isCorner && cd.startBox.w > 0 && cd.startBox.h > 0) {
            const ratio = cd.startBox.w / cd.startBox.h;
            const dWprop = Math.abs(newW - cd.startBox.w) / cd.startBox.w;
            const dHprop = Math.abs(newH - cd.startBox.h) / cd.startBox.h;
            if (dWprop >= dHprop) {
              // Width dominates — derive height from width
              const corrH = newW / ratio;
              if (cd.kind === "scale-tl" || cd.kind === "scale-tr") {
                newY = cd.startBox.y + cd.startBox.h - corrH;
              }
              newH = corrH;
            } else {
              // Height dominates — derive width from height
              const corrW = newH * ratio;
              if (cd.kind === "scale-tl" || cd.kind === "scale-bl") {
                newX = cd.startBox.x + cd.startBox.w - corrW;
              }
              newW = corrW;
            }
          }

          // Ensure positive dimensions
          if (newW < 1) {
            if (cd.kind.includes("l")) {
              newX = cd.startBox.x + cd.startBox.w - 1;
            }
            newW = 1;
          }
          if (newH < 1) {
            if (cd.kind.includes("t")) {
              newY = cd.startBox.y + cd.startBox.h - 1;
            }
            newH = 1;
          }

          engine.dispatchCommand(CommandID.UpdateCrop, {
            x: newX,
            y: newY,
            w: newW,
            h: newH,
            rotation: cd.startRotation,
            deletePixels: cropDeletePixels,
          });
          return;
        }
        if (moveDraft && moveDraft.pointerId === event.pointerId && docPoint) {
          const totalDX = Math.round(docPoint.x - moveDraft.start.x);
          const totalDY = Math.round(docPoint.y - moveDraft.start.y);
          const stepDX = totalDX - moveDraft.appliedDX;
          const stepDY = totalDY - moveDraft.appliedDY;
          if (stepDX !== 0 || stepDY !== 0) {
            for (const id of moveDraft.layerIds) {
              engine.translateLayer({
                layerId: id,
                dx: stepDX,
                dy: stepDY,
              });
            }
            setMoveDraft((current) =>
              current
                ? {
                    ...current,
                    appliedDX: totalDX,
                    appliedDY: totalDY,
                    moved: true,
                  }
                : current,
            );
          }
          return;
        }
        if (
          marqueeDraft &&
          marqueeDraft.pointerId === event.pointerId &&
          docPoint
        ) {
          setMarqueeDraft((current) =>
            current
              ? {
                  ...current,
                  current: { x: docPoint.x, y: docPoint.y },
                  constrain: event.shiftKey,
                }
              : current,
          );
          return;
        }
        if (
          freehandDraft &&
          freehandDraft.pointerId === event.pointerId &&
          docPoint
        ) {
          setFreehandDraft((current) => {
            if (!current) {
              return current;
            }
            const lastPoint = current.points[current.points.length - 1];
            const nextPoint = { x: docPoint.x, y: docPoint.y };
            if (distanceSquared(lastPoint, nextPoint) < 4) {
              return current;
            }
            return {
              ...current,
              points: [...current.points, nextPoint],
            };
          });
          return;
        }
        if ((activeTool === "brush" || activeTool === "pencil" || (activeTool === "eraser" && eraserMode !== "magic")) && brushActiveRef.current) {
          const docPoint = clientPointToDocument(event.clientX, event.clientY);
          if (!docPoint) return;
          engine.dispatchCommand(CommandID.ContinuePaintStroke, {
            x: docPoint.x,
            y: docPoint.y,
            pressure: event.pressure || 0.5,
          } satisfies ContinuePaintStrokeCommand);
          return;
        }
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
        if (transformDraft && transformDraft.pointerId === event.pointerId) {
          setTransformDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        if (cropDraft && cropDraft.pointerId === event.pointerId) {
          setCropDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        if ((activeTool === "brush" || activeTool === "pencil" || (activeTool === "eraser" && eraserMode !== "magic")) && brushActiveRef.current) {
          brushActiveRef.current = false;
          engine.dispatchCommand(CommandID.EndPaintStroke, {});
          return;
        }
        if (quickSelectDraft && quickSelectDraft.pointerId === event.pointerId) {
          engine.endTransaction(true);
          setQuickSelectDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        if (moveDraft && moveDraft.pointerId === event.pointerId) {
          engine.endTransaction(moveDraft.moved);
          setMoveDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        if (marqueeDraft && marqueeDraft.pointerId === event.pointerId) {
          const point = clientPointToDocument(event.clientX, event.clientY);
          const rawEndPoint = point
            ? { x: point.x, y: point.y }
            : marqueeDraft.current;
          const constrainedEnd = constrainedMarqueeEnd(
            marqueeDraft.start,
            rawEndPoint,
            marqueeDraft.constrain,
            selectionOptions.marqueeStyle,
            selectionOptions.marqueeRatioW,
            selectionOptions.marqueeRatioH,
          );
          const w = constrainedEnd.x - marqueeDraft.start.x;
          const h = constrainedEnd.y - marqueeDraft.start.y;
          const rect = {
            x: Math.min(marqueeDraft.start.x, marqueeDraft.start.x + w),
            y: Math.min(marqueeDraft.start.y, marqueeDraft.start.y + h),
            w: Math.max(1, Math.abs(w)),
            h: Math.max(1, Math.abs(h)),
          };
          commitSelection("Create selection", () => {
            engine.createSelection({
              shape: selectionOptions.marqueeShape as "rect" | "ellipse",
              mode: marqueeDraft.mode,
              rect,
              antiAlias: selectionOptions.antiAlias,
            });
          });
          setMarqueeDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        if (freehandDraft && freehandDraft.pointerId === event.pointerId) {
          const point = clientPointToDocument(event.clientX, event.clientY);
          const points = point
            ? [...freehandDraft.points, { x: point.x, y: point.y }]
            : freehandDraft.points;
          if (points.length >= 3) {
            commitSelection("Create lasso selection", () => {
              engine.createSelection({
                shape: "polygon",
                mode: freehandDraft.mode,
                polygon: points,
                antiAlias: selectionOptions.antiAlias,
              });
            });
          }
          setFreehandDraft(null);
          event.currentTarget.releasePointerCapture(event.pointerId);
          return;
        }
        const zoomDrag = zoomDragRef.current;
        if (zoomDrag && zoomDrag.pointerId === event.pointerId) {
          if (!zoomDrag.moved) {
            const step = zoomDrag.zoomOut ? 1 / 1.25 : 1.25;
            engine.setZoom(
              zoomDrag.startZoom * step,
              zoomDrag.anchorX,
              zoomDrag.anchorY,
            );
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
        const docPoint = clientPointToDocument(event.clientX, event.clientY);
        // Read from pending ref first — avoids stale React state when events
        // arrive faster than React can re-render.
        const currentZoom =
          pendingZoomRef.current?.zoom ?? render.viewport.zoom;
        pendingZoomRef.current = {
          zoom: currentZoom * direction,
          anchorX: docPoint?.x,
          anchorY: docPoint?.y,
        };
        if (zoomRafRef.current === null) {
          zoomRafRef.current = requestAnimationFrame(() => {
            zoomRafRef.current = null;
            const pending = pendingZoomRef.current;
            if (pending) {
              // Retain the dispatched zoom so events arriving before React
              // re-renders don't fall back to stale render.viewport.zoom.
              // The useEffect([render]) below clears this once React catches up.
              pendingZoomRef.current = {
                zoom: pending.zoom,
                anchorX: undefined,
                anchorY: undefined,
              };
              engine.setZoom(pending.zoom, pending.anchorX, pending.anchorY);
            }
          });
        }
      }}
    >
      <canvas
        ref={canvasRef}
        className="absolute inset-0 h-full w-full bg-slate-950"
      />
      {marqueeOverlay ||
      freehandOverlay.length > 0 ||
      polygonOverlay ||
      magneticLassoDraft ? (
        <svg
          className="pointer-events-none absolute inset-0 h-full w-full"
          viewBox={`0 0 ${size.width} ${size.height}`}
          aria-hidden="true"
        >
          <title>Selection preview overlay</title>
          {marqueeOverlay ? (
            selectionOptions.marqueeShape === "ellipse" ? (
              <ellipse
                cx={(marqueeOverlay.start.x + marqueeOverlay.current.x) * 0.5}
                cy={(marqueeOverlay.start.y + marqueeOverlay.current.y) * 0.5}
                rx={
                  Math.abs(marqueeOverlay.current.x - marqueeOverlay.start.x) *
                  0.5
                }
                ry={
                  Math.abs(marqueeOverlay.current.y - marqueeOverlay.start.y) *
                  0.5
                }
                fill="rgba(244, 114, 182, 0.12)"
                stroke="rgba(244, 114, 182, 0.95)"
                strokeDasharray="8 6"
                strokeWidth="1.5"
              />
            ) : (
              <rect
                x={Math.min(marqueeOverlay.start.x, marqueeOverlay.current.x)}
                y={Math.min(marqueeOverlay.start.y, marqueeOverlay.current.y)}
                width={Math.abs(
                  marqueeOverlay.current.x - marqueeOverlay.start.x,
                )}
                height={Math.abs(
                  marqueeOverlay.current.y - marqueeOverlay.start.y,
                )}
                fill="rgba(244, 114, 182, 0.12)"
                stroke="rgba(244, 114, 182, 0.95)"
                strokeDasharray="8 6"
                strokeWidth="1.5"
              />
            )
          ) : null}
          {freehandOverlay.length >= 2 ? (
            <path
              d={buildOverlayPath(freehandOverlay)}
              fill="rgba(56, 189, 248, 0.12)"
              stroke="rgba(56, 189, 248, 0.95)"
              strokeDasharray="7 5"
              strokeWidth="1.5"
            />
          ) : null}
          {polygonOverlay && polygonOverlay.points.length > 0 ? (
            <>
              <polyline
                points={[
                  ...polygonOverlay.points,
                  ...(polygonOverlay.hoverPoint
                    ? [polygonOverlay.hoverPoint]
                    : []),
                ]
                  .map((point) => `${point.x},${point.y}`)
                  .join(" ")}
                fill="rgba(56, 189, 248, 0.1)"
                stroke="rgba(56, 189, 248, 0.95)"
                strokeDasharray="7 5"
                strokeWidth="1.5"
              />
              {polygonOverlay.points.map((point, index) => (
                <circle
                  key={`${point.x}-${point.y}-${polygonOverlay.points[index - 1]?.x ?? "start"}-${polygonOverlay.points[index - 1]?.y ?? "start"}`}
                  cx={point.x}
                  cy={point.y}
                  r={index === 0 ? 4 : 3}
                  fill={
                    index === 0
                      ? "rgba(248, 250, 252, 0.95)"
                      : "rgba(56, 189, 248, 0.95)"
                  }
                />
              ))}
            </>
          ) : null}
          {magneticLassoDraft
            ? (() => {
                const allDocPoints = [
                  ...magneticLassoDraft.confirmedPoints,
                  ...magneticLassoDraft.suggestedPath.slice(1),
                ];
                const allCanvasPoints = allDocPoints
                  .map((p) => documentPointToCanvas(p))
                  .filter(
                    (p): p is { x: number; y: number } => p !== null,
                  );
                const anchorCanvas = documentPointToCanvas(
                  magneticLassoDraft.anchorPoint,
                );
                const startCanvas = documentPointToCanvas(
                  magneticLassoDraft.startPoint,
                );
                return (
                  <>
                    {allCanvasPoints.length >= 2 && (
                      <polyline
                        points={allCanvasPoints
                          .map((p) => `${p.x},${p.y}`)
                          .join(" ")}
                        fill="none"
                        stroke="rgba(56, 189, 248, 0.95)"
                        strokeDasharray="7 5"
                        strokeWidth="1.5"
                      />
                    )}
                    {startCanvas && (
                      <circle
                        cx={startCanvas.x}
                        cy={startCanvas.y}
                        r={4}
                        fill="rgba(248, 250, 252, 0.95)"
                      />
                    )}
                    {anchorCanvas && (
                      <circle
                        cx={anchorCanvas.x}
                        cy={anchorCanvas.y}
                        r={3}
                        fill="rgba(56, 189, 248, 0.95)"
                      />
                    )}
                  </>
                );
              })()
            : null}
        </svg>
      ) : null}
      {engine.status !== "ready" ? (
        <div className="editor-backdrop absolute inset-0 flex items-center justify-center p-6">
          <div className="editor-popup max-w-lg rounded-[var(--ui-radius-lg)] p-5 text-center">
            <p className="text-xs uppercase tracking-[0.28em] text-slate-500">
              Wasm bridge
            </p>
            <h2 className="mt-2 text-lg font-semibold text-slate-100">
              {engine.status === "loading"
                ? "Loading engine"
                : "Engine not connected"}
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
