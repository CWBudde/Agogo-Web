import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
	CommandID,
	type DocumentStylePresetEntry,
	type LayerNodeMeta,
	type RenderResult,
} from "@agogo/proto";
import { StylesPanel } from "@/components/styles-panel";
import type { EngineContextValue } from "@/wasm/types";

function makeLayer(
	id: string,
	name: string,
	overrides: Partial<LayerNodeMeta> = {},
): LayerNodeMeta {
	return {
		id,
		name,
		layerType: "pixel",
		visible: true,
		lockMode: "none",
		opacity: 1,
		fillOpacity: 1,
		blendMode: "normal",
		clipToBelow: false,
		clippingBase: false,
		hasMask: false,
		maskEnabled: false,
		hasVectorMask: false,
		...overrides,
	};
}

function makeRender(
	presets: DocumentStylePresetEntry[],
	layers: LayerNodeMeta[] = [],
	activeLayerId: string | null = null,
): RenderResult {
	return {
		uiMeta: {
			stylePresets: presets,
			layers,
			activeLayerId,
		},
	} as unknown as RenderResult;
}

function createEngine(): EngineContextValue & {
	dispatchCommand: ReturnType<typeof vi.fn>;
} {
	const dispatchCommand = vi.fn(() => null);
	return {
		status: "ready",
		handle: null,
		render: null,
		error: null,
		ready: null,
		dispatchCommand,
		createDocument: vi.fn(() => null),
		createSelection: vi.fn(() => null),
		selectAll: vi.fn(() => null),
		deselect: vi.fn(() => null),
		reselect: vi.fn(() => null),
		invertSelection: vi.fn(() => null),
		magicWand: vi.fn(() => null),
		quickSelect: vi.fn(() => null),
		magneticLassoSuggestPath: vi.fn(() => null),
		pickLayerAtPoint: vi.fn(() => null),
		translateLayer: vi.fn(() => null),
		transformSelection: vi.fn(() => null),
		resizeViewport: vi.fn(() => null),
		setZoom: vi.fn(() => null),
		setPan: vi.fn(() => null),
		dispatchPointerEvent: vi.fn(() => null),
		beginTransaction: vi.fn(() => null),
		endTransaction: vi.fn(() => null),
		jumpHistory: vi.fn(() => null),
		clearHistory: vi.fn(() => null),
		setRotation: vi.fn(() => null),
		fitToView: vi.fn(() => null),
		setShowGuides: vi.fn(() => null),
		exportProject: vi.fn(() => null),
		importProject: vi.fn(() => null),
		undo: vi.fn(() => null),
		redo: vi.fn(() => null),
		reload: vi.fn(),
	};
}

const PNG_1 = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";
const PNG_2 = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII=";

describe("StylesPanel", () => {
	it("renders preset thumbnails with correct src and alt", () => {
		const engine = createEngine();
		const presets: DocumentStylePresetEntry[] = [
			{ id: "p1", name: "Soft Shadow", styles: [], thumbnailBase64: PNG_1 },
			{ id: "p2", name: "Hard Glow", styles: [], thumbnailBase64: PNG_2 },
		];
		const layer = makeLayer("layer-1", "Layer 1", {
			styleStack: [{ kind: "stroke", enabled: true, params: { size: 2 } }],
		});
		const renderResult = makeRender(presets, [layer], layer.id);

		render(
			<StylesPanel
				engine={engine}
				render={renderResult}
				activeLayerId={layer.id}
			/>,
		);

		const first = screen.getByAltText("Soft Shadow") as HTMLImageElement;
		const second = screen.getByAltText("Hard Glow") as HTMLImageElement;
		expect(first.src).toBe(PNG_1);
		expect(second.src).toBe(PNG_2);
	});

	it("dispatches ApplyDocumentStylePreset when a preset card is clicked", () => {
		const engine = createEngine();
		const presets: DocumentStylePresetEntry[] = [
			{ id: "p1", name: "Soft Shadow", styles: [], thumbnailBase64: PNG_1 },
		];
		const layer = makeLayer("layer-1", "Layer 1", {
			styleStack: [{ kind: "stroke", enabled: true, params: { size: 2 } }],
		});
		const renderResult = makeRender(presets, [layer], layer.id);

		render(
			<StylesPanel
				engine={engine}
				render={renderResult}
				activeLayerId={layer.id}
			/>,
		);

		fireEvent.click(screen.getByTestId("style-preset-card-p1"));

		expect(engine.dispatchCommand).toHaveBeenCalledWith(
			CommandID.ApplyDocumentStylePreset,
			{ presetId: "p1", layerId: layer.id },
		);
	});

	it("shows an empty-state message when there are no presets", () => {
		const engine = createEngine();
		const renderResult = makeRender([]);

		render(
			<StylesPanel engine={engine} render={renderResult} activeLayerId={null} />,
		);

		expect(screen.getByText(/No styles yet/i)).toBeTruthy();
	});

	it("does not dispatch when no active layer is set", () => {
		const engine = createEngine();
		const presets: DocumentStylePresetEntry[] = [
			{ id: "p1", name: "Soft Shadow", styles: [], thumbnailBase64: PNG_1 },
		];
		const renderResult = makeRender(presets, [], null);

		render(
			<StylesPanel
				engine={engine}
				render={renderResult}
				activeLayerId={null}
			/>,
		);

		const card = screen.getByTestId("style-preset-card-p1");
		fireEvent.click(card);

		expect(engine.dispatchCommand).not.toHaveBeenCalled();
	});
});
