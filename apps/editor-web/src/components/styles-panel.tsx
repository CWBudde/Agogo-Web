import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
	CommandID,
	type DocumentStylePresetEntry,
	type LayerNodeMeta,
	type RenderResult,
} from "@agogo/proto";
import { supportsLayerStyles } from "@/components/layer-style-model";
import { Button } from "@/components/ui/button";
import type { EngineContextValue } from "@/wasm/types";

interface StylesPanelProps {
	engine: EngineContextValue;
	render: RenderResult | null;
	activeLayerId: string | null;
}

interface ContextMenuState {
	preset: DocumentStylePresetEntry;
	x: number;
	y: number;
}

function findLayerById(
	layers: LayerNodeMeta[] | undefined,
	id: string | null,
): LayerNodeMeta | null {
	if (!id || !layers) {
		return null;
	}
	for (const layer of layers) {
		if (layer.id === id) {
			return layer;
		}
		const child = findLayerById(layer.children, id);
		if (child) {
			return child;
		}
	}
	return null;
}

export function StylesPanel({ engine, render, activeLayerId }: StylesPanelProps) {
	const presets = useMemo<DocumentStylePresetEntry[]>(
		() => render?.uiMeta.stylePresets ?? [],
		[render?.uiMeta.stylePresets],
	);

	const activeLayer = useMemo(
		() => findLayerById(render?.uiMeta.layers, activeLayerId),
		[render?.uiMeta.layers, activeLayerId],
	);

	const canApply = Boolean(activeLayerId) && supportsLayerStyles(activeLayer);
	const hasStyleStack = (activeLayer?.styleStack?.length ?? 0) > 0;
	const canCreate = canApply && hasStyleStack;

	const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
	const menuRef = useRef<HTMLDivElement | null>(null);

	useEffect(() => {
		if (!contextMenu) {
			return;
		}
		const onDocClick = (event: MouseEvent) => {
			if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
				setContextMenu(null);
			}
		};
		const onEscape = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				setContextMenu(null);
			}
		};
		document.addEventListener("mousedown", onDocClick);
		document.addEventListener("keydown", onEscape);
		return () => {
			document.removeEventListener("mousedown", onDocClick);
			document.removeEventListener("keydown", onEscape);
		};
	}, [contextMenu]);

	const handleApply = useCallback(
		(preset: DocumentStylePresetEntry) => {
			if (!canApply || !activeLayerId) {
				return;
			}
			engine.dispatchCommand(CommandID.ApplyDocumentStylePreset, {
				presetId: preset.id,
				layerId: activeLayerId,
			});
		},
		[canApply, activeLayerId, engine],
	);

	const handleCreate = useCallback(() => {
		if (!canCreate || !activeLayer?.styleStack) {
			return;
		}
		const defaultName = `Preset ${presets.length + 1}`;
		const name = window.prompt("Preset name", defaultName)?.trim();
		if (!name) {
			return;
		}
		engine.dispatchCommand(CommandID.CreateDocumentStylePreset, {
			name,
			styles: activeLayer.styleStack,
		});
	}, [canCreate, activeLayer, presets.length, engine]);

	const handleRename = useCallback(
		(preset: DocumentStylePresetEntry) => {
			setContextMenu(null);
			const next = window.prompt("Rename preset", preset.name)?.trim();
			if (!next || next === preset.name) {
				return;
			}
			engine.dispatchCommand(CommandID.UpdateDocumentStylePreset, {
				presetId: preset.id,
				name: next,
			});
		},
		[engine],
	);

	const handleDelete = useCallback(
		(preset: DocumentStylePresetEntry) => {
			setContextMenu(null);
			if (!window.confirm(`Delete preset "${preset.name}"?`)) {
				return;
			}
			engine.dispatchCommand(CommandID.DeleteDocumentStylePreset, {
				presetId: preset.id,
			});
		},
		[engine],
	);

	return (
		<div className="flex flex-col">
			<div className="flex items-center justify-between border-b border-white/10 px-2 py-1.5">
				<div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
					Styles
				</div>
				<Button
					size="sm"
					variant="secondary"
					onClick={handleCreate}
					disabled={!canCreate}
					title={
						canCreate
							? "Save current layer style as preset"
							: "Select a layer with styles to save a preset"
					}
				>
					+ New
				</Button>
			</div>

			{presets.length === 0 ? (
				<div className="p-3 text-[11px] text-slate-400">
					No styles yet — save one via the Layer Style dialog.
				</div>
			) : (
				<div className="grid grid-cols-4 gap-2 p-2">
					{presets.map((preset) => (
						<button
							key={preset.id}
							type="button"
							data-testid={`style-preset-card-${preset.id}`}
							onClick={() => handleApply(preset)}
							onContextMenu={(event) => {
								event.preventDefault();
								setContextMenu({
									preset,
									x: event.clientX,
									y: event.clientY,
								});
							}}
							disabled={!canApply}
							className={[
								"flex flex-col items-center gap-1 rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-1 text-left transition hover:ring-2 hover:ring-white/20 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
								canApply ? "" : "pointer-events-none opacity-50",
							].join(" ")}
						>
							{preset.thumbnailBase64 ? (
								<img
									src={preset.thumbnailBase64}
									alt={preset.name}
									width={64}
									height={64}
									className="h-16 w-16 rounded-[var(--ui-radius-sm)] bg-[repeating-conic-gradient(#333_0_90deg,#444_90deg_180deg)] bg-[length:8px_8px] object-cover"
								/>
							) : (
								<div className="flex h-16 w-16 items-center justify-center rounded-[var(--ui-radius-sm)] bg-slate-800 text-[10px] text-slate-500">
									No preview
								</div>
							)}
							<span
								className="w-full truncate text-center text-[10px] text-slate-300"
								title={preset.name}
							>
								{preset.name}
							</span>
						</button>
					))}
				</div>
			)}

			{contextMenu ? (
				<div
					ref={menuRef}
					role="menu"
					style={{
						position: "fixed",
						top: contextMenu.y,
						left: contextMenu.x,
						zIndex: 50,
					}}
					className="min-w-[9rem] rounded-[var(--ui-radius-sm)] border border-white/10 bg-slate-900 py-1 text-[12px] text-slate-200 shadow-lg"
				>
					<button
						type="button"
						role="menuitem"
						className="block w-full px-3 py-1.5 text-left hover:bg-white/10"
						onClick={() => handleRename(contextMenu.preset)}
					>
						Rename
					</button>
					<button
						type="button"
						role="menuitem"
						className="block w-full px-3 py-1.5 text-left hover:bg-white/10"
						onClick={() => handleDelete(contextMenu.preset)}
					>
						Delete
					</button>
				</div>
			) : null}
		</div>
	);
}
