import {
	CommandID,
	type LayerNodeMeta,
	type SetTextStyleCommand,
} from "@agogo/proto";
import type { EngineContextValue } from "@/wasm/types";

export function CharacterPanel({
	engine,
	layer,
}: {
	engine: EngineContextValue;
	layer: LayerNodeMeta;
}) {
	const fontFamily = layer.fontFamily ?? "Arial";
	const fontStyle = layer.fontStyle ?? "Regular";
	const fontSize = layer.fontSize ?? 16;
	const color = layer.textColor ?? [0, 0, 0, 255];
	const alignment = layer.textAlignment ?? "left";
	const leading = layer.leading ?? 1.2;
	const tracking = layer.tracking ?? 0;
	const kerning = layer.kerning ?? 0;
	const antiAlias = layer.antiAlias ?? "None";
	const baselineShift = layer.baselineShift ?? 0;
	const language = layer.language ?? "en-US";
	const bold = layer.bold ?? false;
	const italic = layer.italic ?? false;
	const superscript = layer.superscript ?? false;
	const subscript = layer.subscript ?? false;
	const underline = layer.underline ?? false;
	const strikethrough = layer.strikethrough ?? false;
	const allCaps = layer.allCaps ?? false;
	const smallCaps = layer.smallCaps ?? false;
	const indentLeft = layer.indentLeft ?? 0;
	const indentRight = layer.indentRight ?? 0;
	const indentFirst = layer.indentFirst ?? 0;
	const spaceBefore = layer.spaceBefore ?? 0;
	const spaceAfter = layer.spaceAfter ?? 0;

	const applyStyle = (overrides: Partial<SetTextStyleCommand>) => {
		engine.dispatchCommand(CommandID.SetTextStyle, {
			layerId: layer.id,
			...overrides,
		} satisfies SetTextStyleCommand);
	};

	return (
		<div className="flex flex-col gap-3 p-2">
			<div className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
				Character
			</div>

			{/* Font Family */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Font</span>
				<input
					type="text"
					value={fontFamily}
					onChange={(e) => applyStyle({ fontFamily: e.target.value })}
					className="h-6 flex-1 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
			</div>

			{/* Font Style */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Font Style</span>
				<input
					type="text"
					value={fontStyle}
					onChange={(e) => applyStyle({ fontStyle: e.target.value })}
					className="h-6 flex-1 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
			</div>

			{/* Font Size */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Size</span>
				<input
					type="number"
					min={1}
					max={1000}
					step={1}
					value={fontSize}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (val > 0) applyStyle({ fontSize: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Baseline Shift */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Baseline</span>
				<input
					type="number"
					min={-100}
					max={200}
					step={0.5}
					value={baselineShift}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ baselineShift: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Kerning */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Kerning</span>
				<input
					type="number"
					min={-200}
					max={500}
					step={0.5}
					value={kerning}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ kerning: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
			</div>

			{/* Anti-Alias */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Antialias</span>
				<select
					value={antiAlias}
					onChange={(e) => applyStyle({ antiAlias: e.target.value })}
					className="h-6 w-28 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				>
					<option value="None">None</option>
					<option value="Grayscale">Grayscale</option>
					<option value="Standard">Standard</option>
					<option value="Crisp">Crisp</option>
					<option value="Subpixel">Subpixel</option>
				</select>
			</div>

			{/* Language */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Language</span>
				<input
					type="text"
					value={language}
					onChange={(e) => applyStyle({ language: e.target.value })}
					className="h-6 flex-1 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
			</div>

			{/* Leading */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Leading</span>
				<input
					type="number"
					min={0.5}
					max={5}
					step={0.1}
					value={leading}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (val > 0) applyStyle({ leading: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">×</span>
			</div>

			{/* Tracking */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Tracking</span>
				<input
					type="number"
					min={-50}
					max={200}
					step={0.5}
					value={tracking}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ tracking: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Color */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Color</span>
				<button
					type="button"
					title="Text color"
					style={{
						background: `rgba(${color[0]},${color[1]},${color[2]},${color[3] / 255})`,
					}}
					className="h-6 w-6 flex-shrink-0 rounded border border-white/20 focus-visible:outline-none"
					onClick={() => {
						// Toggle between black and red as a simple demo.
						// A full color picker will be wired in Phase 6.4.
						const isBlack = color[0] === 0 && color[1] === 0 && color[2] === 0;
						applyStyle({
							color: isBlack ? [255, 0, 0, 255] : [0, 0, 0, 255],
						});
					}}
				/>
				<span className="text-[10px] text-slate-500">
					rgba({color[0]},{color[1]},{color[2]},{(color[3] / 255).toFixed(2)})
				</span>
			</div>

			{/* Alignment */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Align</span>
				<div className="flex gap-0.5">
					{(["left", "center", "right", "justify"] as const).map((a) => (
						<button
							key={a}
							type="button"
							title={`Align ${a}`}
							className={`h-6 w-7 rounded text-[10px] ${
								alignment === a
									? "bg-blue-600 text-white"
									: "bg-slate-700 text-slate-400 hover:bg-slate-600"
							}`}
							onClick={() => applyStyle({ alignment: a })}
						>
							{a === "left"
								? "L"
								: a === "center"
									? "C"
									: a === "right"
										? "R"
										: "J"}
						</button>
					))}
				</div>
			</div>

			{/* Decoration */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Style</span>
				<div className="flex gap-0.5">
					<button
						type="button"
						title="Bold"
						className={`h-6 w-7 rounded text-[10px] font-bold ${
							bold
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() => applyStyle({ bold: !bold })}
					>
						B
					</button>
					<button
						type="button"
						title="Italic"
						className={`h-6 w-7 rounded text-[10px] italic ${
							italic
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() => applyStyle({ italic: !italic })}
					>
						I
					</button>
					<button
						type="button"
						title="Underline"
						className={`h-6 w-7 rounded text-[10px] underline ${
							underline
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() => applyStyle({ underline: !underline })}
					>
						U
					</button>
					<button
						type="button"
						title="Strikethrough"
						className={`h-6 w-7 rounded text-[10px] line-through ${
							strikethrough
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() => applyStyle({ strikethrough: !strikethrough })}
					>
						S
					</button>
					<button
						type="button"
						title="All Caps"
						className={`h-6 w-7 rounded text-[10px] font-semibold ${
							allCaps
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() =>
							applyStyle({ allCaps: !allCaps, smallCaps: false })
						}
					>
						AA
					</button>
					<button
						type="button"
						title="Small Caps"
						className={`h-6 w-7 rounded text-[10px] font-semibold ${
							smallCaps
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() =>
							applyStyle({ smallCaps: !smallCaps, allCaps: false })
						}
					>
						Aa
					</button>
				</div>
			</div>

			{/* Superscript / Subscript */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Script</span>
				<div className="flex gap-0.5">
					<button
						type="button"
						title="Superscript"
						className={`h-6 w-9 rounded text-[10px] ${
							superscript
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() =>
							applyStyle({ superscript: !superscript, subscript: false })
						}
					>
						Sup
					</button>
					<button
						type="button"
						title="Subscript"
						className={`h-6 w-9 rounded text-[10px] ${
							subscript
								? "bg-blue-600 text-white"
								: "bg-slate-700 text-slate-400 hover:bg-slate-600"
						}`}
						onClick={() =>
							applyStyle({ subscript: !subscript, superscript: false })
						}
					>
						Sub
					</button>
				</div>
			</div>

			<div className="mt-1 text-[10px] uppercase tracking-[0.18em] text-slate-500">
				Paragraph
			</div>

			{/* Indent Left */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Indent L</span>
				<input
					type="number"
					min={0}
					max={1000}
					step={1}
					value={indentLeft}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ indentLeft: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Indent Right */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Indent R</span>
				<input
					type="number"
					min={0}
					max={1000}
					step={1}
					value={indentRight}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ indentRight: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Indent First Line */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Indent 1st</span>
				<input
					type="number"
					min={-500}
					max={1000}
					step={1}
					value={indentFirst}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ indentFirst: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Space Before */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Space ↑</span>
				<input
					type="number"
					min={0}
					max={500}
					step={1}
					value={spaceBefore}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ spaceBefore: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>

			{/* Space After */}
			<div className="flex items-center gap-2">
				<span className="w-14 text-[11px] text-slate-400">Space ↓</span>
				<input
					type="number"
					min={0}
					max={500}
					step={1}
					value={spaceAfter}
					onChange={(e) => {
						const val = Number.parseFloat(e.target.value);
						if (!Number.isNaN(val)) applyStyle({ spaceAfter: val });
					}}
					className="h-6 w-16 rounded border border-white/10 bg-slate-800 px-1 text-[11px] text-slate-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
				/>
				<span className="text-[10px] text-slate-500">px</span>
			</div>
		</div>
	);
}
