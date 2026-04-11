import type {
	LayerNodeMeta,
	LayerStyleEntryCommand,
	LayerStyleKind,
} from "@agogo/proto";

export interface LayerStyleCatalogEntry {
	kind: LayerStyleKind;
	label: string;
}

export interface EnsureLayerStyleEntryResult {
	styles: LayerStyleEntryCommand[];
	entry: LayerStyleEntryCommand;
}

export const layerStyleCatalog: readonly LayerStyleCatalogEntry[] = [
	{ kind: "color-overlay", label: "Color Overlay" },
	{ kind: "gradient-overlay", label: "Gradient Overlay" },
	{ kind: "pattern-overlay", label: "Pattern Overlay" },
	{ kind: "stroke", label: "Stroke" },
	{ kind: "inner-shadow", label: "Inner Shadow" },
	{ kind: "inner-glow", label: "Inner Glow" },
	{ kind: "bevel-emboss", label: "Bevel & Emboss" },
	{ kind: "satin", label: "Satin" },
	{ kind: "drop-shadow", label: "Drop Shadow" },
	{ kind: "outer-glow", label: "Outer Glow" },
] as const;

const layerStyleOrder = new Map(
	layerStyleCatalog.map((entry, index) => [entry.kind, index] as const),
);

export function defaultLayerStyleParams(kind: LayerStyleKind): Record<string, unknown> {
	switch (kind) {
		case "drop-shadow":
			return { blendMode: "multiply", color: [0, 0, 0, 255], opacity: 0.75, angle: 120 };
		case "inner-shadow":
			return { blendMode: "multiply", color: [0, 0, 0, 255], opacity: 0.75, angle: 120 };
		case "outer-glow":
			return { blendMode: "screen", color: [255, 255, 255, 255], opacity: 0.75 };
		case "inner-glow":
			return { blendMode: "screen", color: [255, 255, 255, 255], opacity: 0.75 };
		case "bevel-emboss":
			return {
				style: "inner-bevel",
				technique: "smooth",
				depth: 1,
				direction: "up",
				angle: 120,
				altitude: 30,
				highlightBlendMode: "screen",
				highlightColor: [255, 255, 255, 255],
				highlightOpacity: 0.75,
				shadowBlendMode: "multiply",
				shadowColor: [0, 0, 0, 255],
				shadowOpacity: 0.75,
				contour: "linear",
			};
		case "satin":
			return {
				blendMode: "multiply",
				color: [0, 0, 0, 255],
				opacity: 0.5,
				angle: 19,
				contour: "gaussian",
			};
		case "color-overlay":
			return { blendMode: "normal", color: [0, 0, 0, 255], opacity: 1 };
		case "gradient-overlay":
			return { blendMode: "normal", opacity: 1, angle: 90, scale: 1, align: true };
		case "pattern-overlay":
			return { blendMode: "normal", opacity: 1, scale: 1, link: true };
		case "stroke":
			return {
				size: 1,
				position: "outside",
				blendMode: "normal",
				opacity: 1,
				color: [0, 0, 0, 255],
				fillType: "color",
			};
	}
}

export function createDefaultLayerStyleStack(): LayerStyleEntryCommand[] {
	return layerStyleCatalog.map((entry) => ({
		kind: entry.kind,
		enabled: false,
		params: defaultLayerStyleParams(entry.kind),
	}));
}

export function cloneLayerStyleStack(styles: LayerStyleEntryCommand[] | undefined): LayerStyleEntryCommand[] {
	return (styles ?? []).map((entry) => ({
		kind: entry.kind,
		enabled: entry.enabled,
		params: entry.params ? structuredClone(entry.params) : undefined,
	}));
}

export function ensureLayerStyleEntry(
	styles: LayerStyleEntryCommand[],
	kind: LayerStyleKind,
): EnsureLayerStyleEntryResult {
	const nextStyles = cloneLayerStyleStack(styles);
	const existingIndex = nextStyles.findIndex((entry) => entry.kind === kind);
	const existing = existingIndex >= 0 ? nextStyles[existingIndex] : undefined;
	if (existing) {
		const entry =
			existing.params === undefined
				? {
						...existing,
						params: defaultLayerStyleParams(kind),
					}
				: existing;
		if (entry !== existing) {
			nextStyles[existingIndex] = entry;
		}
		return { styles: nextStyles, entry };
	}

	const nextEntry: LayerStyleEntryCommand = {
		kind,
		enabled: false,
		params: defaultLayerStyleParams(kind),
	};
	const nextIndex = layerStyleOrder.get(kind) ?? styles.length;
	const insertAt = nextStyles.findIndex(
		(entry) => (layerStyleOrder.get(entry.kind) ?? nextStyles.length) > nextIndex,
	);
	if (insertAt === -1) {
		nextStyles.push(nextEntry);
	} else {
		nextStyles.splice(insertAt, 0, nextEntry);
	}
	return { styles: nextStyles, entry: nextEntry };
}

export function formatLayerStyleLabel(kind: LayerStyleKind): string {
	return layerStyleCatalog.find((entry) => entry.kind === kind)?.label ?? kind;
}

export function supportsLayerStyles(layer: Pick<LayerNodeMeta, "layerType"> | null | undefined): boolean {
	return layer?.layerType === "pixel" || layer?.layerType === "text" || layer?.layerType === "vector";
}
