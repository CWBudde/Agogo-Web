import { StrictMode } from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
	DocumentStylePresetEntry,
	LayerNodeMeta,
	LayerStyleEntryCommand,
} from "@agogo/proto";
import { CommandID } from "@agogo/proto";
import { LayerStyleDialog } from "@/components/layer-style-dialog";
import { createDefaultLayerStyleStack, ensureLayerStyleEntry } from "@/components/layer-style-model";

function makeLayer(
	layerType: LayerNodeMeta["layerType"],
	overrides: Partial<LayerNodeMeta> = {},
): LayerNodeMeta {
	return {
		id: `${layerType}-1`,
		name: `${layerType} layer`,
		layerType,
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

function makeDraftStyles(): LayerStyleEntryCommand[] {
	const { styles } = ensureLayerStyleEntry(createDefaultLayerStyleStack(), "stroke");
	return styles.map((entry) =>
		entry.kind === "stroke"
			? {
					...entry,
					enabled: true,
					params: {
						...(entry.params ?? {}),
						size: 3,
					},
				}
			: entry,
	);
}

describe("LayerStyleDialog", () => {
	it("dispatches live preview commands when enabling an effect and editing params", () => {
		const engine = { dispatchCommand: vi.fn() };

		render(
			<StrictMode>
				<LayerStyleDialog
					open
					engine={engine}
					layer={makeLayer("pixel")}
					presets={[]}
					onClose={vi.fn()}
				/>
			</StrictMode>,
		);

		fireEvent.click(screen.getByLabelText("Drop Shadow"));

		expect(engine.dispatchCommand).toHaveBeenCalledTimes(1);
		expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.SetLayerStyleEnabled, {
			layerId: "pixel-1",
			kind: "drop-shadow",
			enabled: true,
		});

		fireEvent.change(screen.getByLabelText("Opacity"), {
			target: { value: "0.42" },
		});

		expect(engine.dispatchCommand).toHaveBeenCalledTimes(2);
		expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.SetLayerStyleParams, {
			layerId: "pixel-1",
			kind: "drop-shadow",
			params: expect.objectContaining({ opacity: 0.42 }),
		});
	});

	it("restores the original stack on cancel", () => {
		const originalStack = makeDraftStyles();
		const engine = { dispatchCommand: vi.fn() };

		render(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { styleStack: originalStack })}
				presets={[]}
				onClose={vi.fn()}
			/>,
		);

		fireEvent.click(screen.getByLabelText("Drop Shadow"));
		fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

		expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetLayerStyleStack, {
			layerId: "pixel-1",
			styles: originalStack,
		});
	});

	it("keeps the original cancel-restore stack stable across rerenders during one open session", () => {
		const originalStack = makeDraftStyles();
		const previewStack = structuredClone(originalStack).map((entry) =>
			entry.kind === "stroke"
				? {
						...entry,
						params: {
							...(entry.params ?? {}),
							size: 9,
						},
					}
				: entry,
		);
		const engine = { dispatchCommand: vi.fn() };
		const onClose = vi.fn();
		const { rerender } = render(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { styleStack: originalStack })}
				presets={[]}
				onClose={onClose}
			/>,
		);

		rerender(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { styleStack: previewStack })}
				presets={[]}
				onClose={onClose}
			/>,
		);

		fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

		expect(engine.dispatchCommand).toHaveBeenLastCalledWith(CommandID.SetLayerStyleStack, {
			layerId: "pixel-1",
			styles: originalStack,
		});
	});

	it("closes if the active layer changes while the dialog is open", () => {
		const originalStack = makeDraftStyles();
		const engine = { dispatchCommand: vi.fn() };
		const onClose = vi.fn();
		const { rerender } = render(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { id: "pixel-1", styleStack: originalStack })}
				presets={[]}
				onClose={onClose}
			/>,
		);

		rerender(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { id: "pixel-2", styleStack: originalStack })}
				presets={[]}
				onClose={onClose}
			/>,
		);

		expect(onClose).toHaveBeenCalledTimes(1);
	});

	it("creates a document preset from the current draft stack", () => {
		const draftStyles = makeDraftStyles();
		const engine = { dispatchCommand: vi.fn() };

		render(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel", { styleStack: draftStyles })}
				presets={[]}
				onClose={vi.fn()}
			/>,
		);

		fireEvent.change(screen.getByLabelText("Preset name"), {
			target: { value: "Soft Shadow" },
		});
		fireEvent.click(screen.getByRole("button", { name: "Save Preset" }));

		expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.CreateDocumentStylePreset, {
			name: "Soft Shadow",
			styles: expect.arrayContaining([
				expect.objectContaining({
					kind: "stroke",
					enabled: true,
				}),
				expect.objectContaining({
					kind: "color-overlay",
					enabled: false,
				}),
			]),
		});
	});

	it("applies a preset to the active layer", () => {
		const presets: DocumentStylePresetEntry[] = [
			{
				id: "preset-shadow",
				name: "Soft Shadow",
				styles: makeDraftStyles(),
			},
		];
		const engine = { dispatchCommand: vi.fn() };

		render(
			<LayerStyleDialog
				open
				engine={engine}
				layer={makeLayer("pixel")}
				presets={presets}
				onClose={vi.fn()}
			/>,
		);

		const presetItem = screen.getByTestId("style-preset-preset-shadow");
		fireEvent.click(within(presetItem).getByRole("button", { name: "Apply" }));

		expect(engine.dispatchCommand).toHaveBeenCalledWith(CommandID.ApplyDocumentStylePreset, {
			presetId: "preset-shadow",
			layerId: "pixel-1",
		});
	});
});
