import { useEffect, useRef, useState } from "react";
import {
	CommandID,
	type DocumentStylePresetEntry,
	type LayerNodeMeta,
	type LayerStyleEntryCommand,
	type LayerStyleKind,
} from "@agogo/proto";
import { LayerStyleForm } from "@/components/layer-style-form";
import { cloneLayerStyleStack, ensureLayerStyleEntry } from "@/components/layer-style-model";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";

type LayerStyleDialogEngine = {
	dispatchCommand(commandId: number, payload?: unknown): unknown;
};

export function LayerStyleDialog({
	open,
	engine,
	layer,
	presets,
	onClose,
}: {
	open: boolean;
	engine: LayerStyleDialogEngine;
	layer: LayerNodeMeta | null;
	presets: DocumentStylePresetEntry[];
	onClose: () => void;
}) {
	const [draftStyles, setDraftStyles] = useState<LayerStyleEntryCommand[]>([]);
	const [presetName, setPresetName] = useState("");
	const originalStackRef = useRef<LayerStyleEntryCommand[]>([]);
	const wasOpenRef = useRef(false);
	const sessionLayerIdRef = useRef<string | null>(null);
	const closeRequestedRef = useRef(false);

	useEffect(() => {
		if (!open) {
			wasOpenRef.current = false;
			sessionLayerIdRef.current = null;
			closeRequestedRef.current = false;
			return;
		}
		if (
			wasOpenRef.current &&
			sessionLayerIdRef.current !== null &&
			layer?.id !== sessionLayerIdRef.current
		) {
			if (!closeRequestedRef.current) {
				closeRequestedRef.current = true;
				onClose();
			}
			return;
		}
		if (wasOpenRef.current) {
			return;
		}
		wasOpenRef.current = true;
		sessionLayerIdRef.current = layer?.id ?? null;
		closeRequestedRef.current = false;
		const originalStack = cloneLayerStyleStack(layer?.styleStack);
		originalStackRef.current = originalStack;
		setDraftStyles(originalStack);
		setPresetName("");
	}, [open, layer, onClose]);

	const layerId = layer?.id;

	const handleEnabledChange = (kind: LayerStyleKind, enabled: boolean) => {
		if (!layerId) {
			return;
		}
		setDraftStyles((current) => {
			const { styles, entry } = ensureLayerStyleEntry(current, kind);
			return styles.map((style) => (style.kind === kind ? { ...entry, enabled } : style));
		});
		engine.dispatchCommand(CommandID.SetLayerStyleEnabled, {
			layerId,
			kind,
			enabled,
		});
	};

	const handleParamsChange = (kind: LayerStyleKind, params: Record<string, unknown>) => {
		if (!layerId) {
			return;
		}
		setDraftStyles((current) => {
			const { styles } = ensureLayerStyleEntry(current, kind);
			return styles.map((style) => (style.kind === kind ? { ...style, params } : style));
		});
		engine.dispatchCommand(CommandID.SetLayerStyleParams, {
			layerId,
			kind,
			params,
		});
	};

	const handleReset = () => {
		if (!layerId) {
			return;
		}
		setDraftStyles([]);
		engine.dispatchCommand(CommandID.SetLayerStyleStack, {
			layerId,
			styles: [],
		});
	};

	const handleCancel = () => {
		if (layerId) {
			engine.dispatchCommand(CommandID.SetLayerStyleStack, {
				layerId,
				styles: originalStackRef.current,
			});
		}
		onClose();
	};

	const handleCreatePreset = () => {
		const name = presetName.trim();
		if (!name) {
			return;
		}
		engine.dispatchCommand(CommandID.CreateDocumentStylePreset, {
			name,
			styles: cloneLayerStyleStack(draftStyles),
		});
		setPresetName("");
	};

	const applyPreset = (preset: DocumentStylePresetEntry) => {
		if (!layerId) {
			return;
		}
		setDraftStyles(cloneLayerStyleStack(preset.styles));
		engine.dispatchCommand(CommandID.ApplyDocumentStylePreset, {
			presetId: preset.id,
			layerId,
		});
	};

	return (
		<Dialog
			open={open}
			title="Layer Style"
			description="Preview edits live, then keep them with OK or restore the captured stack with Cancel."
			className="max-w-4xl"
		>
			<div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_16rem]">
				<div className="min-w-0 rounded-[var(--ui-radius-md)] border border-white/10 bg-black/20">
					<LayerStyleForm
						layer={layer}
						styles={draftStyles}
						onEnabledChange={handleEnabledChange}
						onParamsChange={handleParamsChange}
					/>
				</div>

				<div className="space-y-3">
					<section className="rounded-[var(--ui-radius-md)] border border-white/10 bg-black/20 p-3">
						<h3 className="text-[10px] uppercase tracking-[0.18em] text-slate-500">Presets</h3>
						<div className="mt-3 space-y-2">
							<label className="space-y-1 text-[11px] text-slate-300" htmlFor="layer-style-preset-name">
								<span>Preset name</span>
								<input
									id="layer-style-preset-name"
									aria-label="Preset name"
									value={presetName}
									onChange={(event) => setPresetName(event.target.value)}
									className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2 py-1.5 text-[12px] text-slate-100 focus-visible:outline-none"
								/>
							</label>
							<Button size="sm" className="w-full" onClick={handleCreatePreset}>
								Save Preset
							</Button>
						</div>

						<div className="mt-3 space-y-2">
							{presets.length > 0 ? (
								presets.map((preset) => (
									<div
										key={preset.id}
										data-testid={`style-preset-${preset.id}`}
										className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-2"
									>
										<div className="text-[12px] font-medium text-slate-100">{preset.name}</div>
										<div className="mt-2 flex flex-wrap gap-2">
											<Button size="sm" onClick={() => applyPreset(preset)}>
												Apply
											</Button>
										</div>
									</div>
								))
							) : (
								<p className="text-[11px] text-slate-400">No saved style presets yet.</p>
							)}
						</div>
					</section>
				</div>
			</div>

			<div className="mt-4 flex justify-between gap-3">
				<Button variant="ghost" onClick={handleReset}>
					Reset
				</Button>
				<div className="flex gap-2">
					<Button variant="secondary" onClick={handleCancel}>
						Cancel
					</Button>
					<Button onClick={onClose}>OK</Button>
				</div>
			</div>
		</Dialog>
	);
}
