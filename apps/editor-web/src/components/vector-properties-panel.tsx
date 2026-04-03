import {
	CommandID,
	type LayerNodeMeta,
	type SetVectorLayerStyleCommand,
} from "@agogo/proto";
import type { EngineContextValue } from "@/wasm/types";

export function VectorPropertiesPanel({
	engine,
	layer,
}: {
	engine: EngineContextValue;
	layer: LayerNodeMeta;
}) {
	const fillColor = layer.fillColor ?? [0, 0, 0, 255];
	const strokeColor = layer.strokeColor ?? [0, 0, 0, 0];
	const strokeWidth = layer.strokeWidth ?? 0;

	const apply = (
		fill: [number, number, number, number],
		stroke: [number, number, number, number],
		width: number,
	) => {
		engine.dispatchCommand(CommandID.SetVectorLayerStyle, {
			layerId: layer.id,
			fillColor: fill,
			strokeColor: stroke,
			strokeWidth: width,
		} satisfies SetVectorLayerStyleCommand);
	};

	return (
		<div className="flex flex-col gap-3 p-2">
			<div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
				Shape
			</div>

			{/* Fill */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Fill</span>
				<button
					type="button"
					title="Fill color"
					style={{
						background:
							fillColor[3] === 0
								? "repeating-conic-gradient(#555 0% 25%, #333 0% 50%) 0 0/8px 8px"
								: `rgba(${fillColor[0]},${fillColor[1]},${fillColor[2]},${fillColor[3] / 255})`,
					}}
					className="h-6 w-6 flex-shrink-0 rounded border border-white/20 focus-visible:outline-none"
					onClick={() => {
						if (fillColor[3] === 0) {
							apply([0, 0, 0, 255], strokeColor, strokeWidth);
						} else {
							apply([0, 0, 0, 0], strokeColor, strokeWidth);
						}
					}}
				/>
				<span className="text-[10px] text-slate-500">
					{fillColor[3] === 0
						? "None"
						: `rgba(${fillColor[0]},${fillColor[1]},${fillColor[2]},${(fillColor[3] / 255).toFixed(2)})`}
				</span>
			</div>

			{/* Stroke */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Stroke</span>
				<button
					type="button"
					title="Stroke color"
					style={{
						background:
							strokeColor[3] === 0
								? "repeating-conic-gradient(#555 0% 25%, #333 0% 50%) 0 0/8px 8px"
								: `rgba(${strokeColor[0]},${strokeColor[1]},${strokeColor[2]},${strokeColor[3] / 255})`,
					}}
					className="h-6 w-6 flex-shrink-0 rounded border border-white/20 focus-visible:outline-none"
					onClick={() => {
						if (strokeColor[3] === 0) {
							apply(fillColor, [0, 0, 0, 255], strokeWidth || 2);
						} else {
							apply(fillColor, [0, 0, 0, 0], strokeWidth);
						}
					}}
				/>
				<input
					type="number"
					min={0}
					max={200}
					step={1}
					value={strokeWidth}
					disabled={strokeColor[3] === 0}
					onChange={(e) =>
						apply(fillColor, strokeColor, Math.max(0, Number(e.target.value)))
					}
					className="w-14 rounded border border-white/10 bg-transparent px-1 py-0.5 text-[11px] text-slate-200 disabled:opacity-40 focus-visible:outline-none"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Edit Path button */}
			<button
				type="button"
				className="mt-1 rounded border border-cyan-500/40 bg-cyan-500/15 px-2 py-1 text-[11px] text-cyan-200 hover:bg-cyan-500/25 focus-visible:outline-none"
				onClick={() => {
					engine.dispatchCommand(CommandID.EnterVectorEditMode, {
						layerId: layer.id,
					});
				}}
			>
				Edit Path
			</button>
		</div>
	);
}
