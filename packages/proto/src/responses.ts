// RenderResult — returned by the engine after each command dispatch.
// bufferPtr and bufferLen reference a region inside the Wasm linear memory.

import type {
	AdjustmentKind,
	AdjustmentLayerParams,
	DocumentStylePresetEntry,
	FreeTransformMeta,
	LayerStyleEntryCommand,
	PathOverlay,
} from "./commands.js";

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
	/** Present when a path tool is active. */
	pathOverlay?: PathOverlay;
	/** Named paths in the active document. */
	paths?: Array<{ name: string; active: boolean }>;
	/** Non-empty while a VectorLayer's path is being edited via the direct-select tool. */
	editingVectorLayerId?: string;
	/** Non-empty while a TextLayer is in text edit mode. */
	editingTextLayerId?: string;
	/** Doc-space X coordinate of the text insertion cursor. */
	textCursorX?: number;
	/** Doc-space Y coordinate of the text insertion cursor baseline. */
	textCursorY?: number;
	stylePresets?: DocumentStylePresetEntry[];
}

export interface BlendIfConfig {
	gray: [number, number];
	red: [number, number];
	green: [number, number];
	blue: [number, number];
}

export interface LayerNodeMeta {
	id: string;
	name: string;
	layerType: "pixel" | "group" | "adjustment" | "text" | "vector";
	adjustmentKind?: AdjustmentKind;
	params?: AdjustmentLayerParams;
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
	styleStack?: LayerStyleEntryCommand[];
	// VectorLayer-specific style fields. Only present when layerType === "vector".
	fillColor?: [number, number, number, number];
	strokeColor?: [number, number, number, number];
	strokeWidth?: number;
	// TextLayer-specific fields. Only present when layerType === "text".
	text?: string;
	fontFamily?: string;
	fontStyle?: string;
	fontSize?: number;
	antiAlias?: string;
	textColor?: [number, number, number, number];
	textAlignment?: "left" | "center" | "right" | "justify";
	textType?: "point" | "area";
	baselineShift?: number;
	bold?: boolean;
	italic?: boolean;
	tracking?: number;
	kerning?: number;
	language?: string;
	leading?: number;
	orientation?: string;
	superscript?: boolean;
	subscript?: boolean;
	underline?: boolean;
	strikethrough?: boolean;
	allCaps?: boolean;
	smallCaps?: boolean;
	indentLeft?: number;
	indentRight?: number;
	indentFirst?: number;
	spaceBefore?: number;
	spaceAfter?: number;
	blendIf?: BlendIfConfig;
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
	/** RGBA color returned only by SampleMergedColor command. */
	sampledColor?: [number, number, number, number];
}

export interface RawRenderResult {
	frameId: number;
	viewport: ViewportMeta;
	bufferPtr: number;
	bufferLen: number;
	reused: boolean;
}
