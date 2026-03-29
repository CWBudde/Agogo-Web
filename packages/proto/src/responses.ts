// RenderResult — returned by the engine after each command dispatch.
// bufferPtr and bufferLen reference a region inside the Wasm linear memory.

import type { FreeTransformMeta } from "./commands.js";

export interface DirtyRect {
	x: number;
	y: number;
	w: number;
	h: number;
}

export interface ViewportMeta {
	centerX: number;
	centerY: number;
	zoom: number;
	rotation: number;
	canvasW: number;
	canvasH: number;
	devicePixelRatio: number;
}

export interface ThumbnailEntry {
	/** Base64-encoded RGBA pixel data at thumbnailSize × thumbnailSize pixels. */
	layerRGBA: string;
	/** Base64-encoded RGBA pixel data for the mask (grayscale converted to RGBA). Present only when the layer has a mask. */
	maskRGBA?: string;
}

export interface SelectionMeta {
	active: boolean;
	bounds?: DirtyRect;
	pixelCount: number;
	lastSelectionAvailable: boolean;
}

export interface UIMeta {
	activeLayerId: string | null;
	activeLayerName: string | null;
	cursorType: string;
	statusText: string;
	rulerOriginX: number;
	rulerOriginY: number;
	history: HistoryEntry[];
	currentHistoryIndex: number;
	canUndo: boolean;
	canRedo: boolean;
	activeDocumentId: string;
	activeDocumentName: string;
	documentWidth: number;
	documentHeight: number;
	documentBackground: string;
	layers: LayerNodeMeta[];
	/** Monotonic counter incremented on every document mutation. Use to detect when thumbnails need refresh. */
	contentVersion: number;
	/** Set when the user is actively editing a layer mask; empty/absent otherwise. */
	maskEditLayerId?: string;
	selection: SelectionMeta;
	/** Present when free transform is active. */
	freeTransform?: FreeTransformMeta;
	/** Present when crop tool is active. */
	crop?: import("./commands.js").CropMeta;
}

export interface LayerNodeMeta {
	id: string;
	name: string;
	layerType: "pixel" | "group" | "adjustment" | "text" | "vector";
	parentId?: string;
	visible: boolean;
	lockMode: "none" | "pixels" | "position" | "all";
	opacity: number;
	fillOpacity: number;
	blendMode: string;
	clipToBelow: boolean;
	clippingBase: boolean;
	hasMask: boolean;
	maskEnabled: boolean;
	hasVectorMask: boolean;
	isolated?: boolean;
	children?: LayerNodeMeta[];
}

export interface HistoryEntry {
	id: number;
	description: string;
	state: "done" | "current" | "undone";
}

export interface RenderResult {
	frameId: number;
	viewport: ViewportMeta;
	dirtyRects: DirtyRect[];
	pixelFormat: "rgba8-premultiplied";
	bufferPtr: number;
	bufferLen: number;
	uiMeta: UIMeta;
	/** Present only in the response to GetLayerThumbnails. Maps layer ID → thumbnail RGBA data. */
	thumbnails?: Record<string, ThumbnailEntry>;
	/** Path points returned only by MagneticLassoSuggestPath command. In document coordinates. */
	suggestedPath?: Array<{ x: number; y: number }>;
}
