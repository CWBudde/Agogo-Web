import type {
	BlendIfConfig,
	LayerNodeMeta,
	LayerStyleEntryCommand,
	LayerStyleKind,
} from "@agogo/proto";
import {
	cloneLayerStyleStack,
	createDefaultLayerStyleStack,
	ensureLayerStyleEntry,
	formatLayerStyleLabel,
	supportsLayerStyles,
} from "@/components/layer-style-model";

const blendModeOptions = [
	{ value: "normal", label: "Normal" },
	{ value: "multiply", label: "Multiply" },
	{ value: "screen", label: "Screen" },
	{ value: "overlay", label: "Overlay" },
];

export function LayerStyleForm({
	layer,
	styles,
	onEnabledChange,
	onParamsChange,
}: {
	layer: LayerNodeMeta | null;
	styles: LayerStyleEntryCommand[] | undefined;
	onEnabledChange: (kind: LayerStyleKind, enabled: boolean) => void;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	if (!supportsLayerStyles(layer)) {
		return (
			<div className="space-y-2 p-3 text-[11px] text-slate-400">
				<h2 className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
					Layer Styles
				</h2>
				<p>Layer styles are currently available for pixel, text, and vector layers.</p>
			</div>
		);
	}

	let catalog = styles?.length ? cloneLayerStyleStack(styles) : createDefaultLayerStyleStack();
	for (const entry of createDefaultLayerStyleStack()) {
		catalog = ensureLayerStyleEntry(catalog, entry.kind).styles;
	}
	const blendIf = layer?.blendIf;
	const isBlendIfSupported = blendIf !== undefined;

	const updateParams = (kind: LayerStyleKind, patch: Record<string, unknown>) => {
		const { entry } = ensureLayerStyleEntry(catalog, kind);
		onParamsChange(kind, { ...(entry.params ?? {}), ...patch });
	};
	const updateBlendIf = (patch: Partial<BlendIfConfig>) => {
		if (!isBlendIfSupported || !blendIf) {
			return;
		}
		onParamsChange("blend-if" as LayerStyleKind, { ...blendIf, ...patch });
	};

	return (
		<div className="space-y-3 p-3">
			<h2 className="text-[10px] uppercase tracking-[0.18em] text-slate-500">
				Layer Styles
			</h2>
			<div className="space-y-2">
				{isBlendIfSupported ? (
					<section className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-2">
						<div className="flex items-center justify-between gap-3">
							<h3 className="text-[11px] font-medium text-slate-100">Blend If</h3>
							<span className="text-[10px] text-slate-400">Stub</span>
						</div>
						<div className="mt-3 space-y-2">
							<BlendIfChannelField
								label="Gray"
								value={blendIf.gray}
								onMinChange={(value) => updateBlendIf({ gray: [value, blendIf.gray[1]] })}
								onMaxChange={(value) => updateBlendIf({ gray: [blendIf.gray[0], value] })}
							/>
							<BlendIfChannelField
								label="Red"
								value={blendIf.red}
								onMinChange={(value) => updateBlendIf({ red: [value, blendIf.red[1]] })}
								onMaxChange={(value) => updateBlendIf({ red: [blendIf.red[0], value] })}
							/>
							<BlendIfChannelField
								label="Green"
								value={blendIf.green}
								onMinChange={(value) => updateBlendIf({ green: [value, blendIf.green[1]] })}
								onMaxChange={(value) => updateBlendIf({ green: [blendIf.green[0], value] })}
							/>
							<BlendIfChannelField
								label="Blue"
								value={blendIf.blue}
								onMinChange={(value) => updateBlendIf({ blue: [value, blendIf.blue[1]] })}
								onMaxChange={(value) => updateBlendIf({ blue: [blendIf.blue[0], value] })}
							/>
						</div>
					</section>
				) : null}
				{catalog.map((entry) => (
					<LayerStyleSection
						key={entry.kind}
						entry={entry}
						onEnabledChange={onEnabledChange}
						onParamsChange={updateParams}
					/>
				))}
			</div>
		</div>
	);
}

function LayerStyleSection({
	entry,
	onEnabledChange,
	onParamsChange,
}: {
	entry: LayerStyleEntryCommand;
	onEnabledChange: (kind: LayerStyleKind, enabled: boolean) => void;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<section className="rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 p-2">
			<div className="flex items-center justify-between gap-3">
				<h3 className="text-[11px] font-medium text-slate-100">
					{formatLayerStyleLabel(entry.kind)}
				</h3>
				<label className="flex items-center gap-2 text-[11px] text-slate-300">
					<input
						aria-label={formatLayerStyleLabel(entry.kind)}
						type="checkbox"
						className="accent-cyan-400"
						checked={entry.enabled}
						onChange={(event) => onEnabledChange(entry.kind, event.target.checked)}
					/>
					Enabled
				</label>
			</div>
			{entry.enabled ? (
				<div className="mt-3 space-y-2">
					{renderEffectEditor(entry.kind, entry.params ?? {}, onParamsChange)}
				</div>
			) : null}
		</section>
	);
}

function renderEffectEditor(
	kind: LayerStyleKind,
	params: Record<string, unknown>,
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void,
) {
	switch (kind) {
		case "drop-shadow":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 0.75)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Angle"
						param="angle"
						value={numberParam(params.angle, 120)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Distance"
						param="distance"
						value={numberParam(params.distance, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Spread"
						param="spread"
						value={numberParam(params.spread, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Size"
						param="size"
						value={numberParam(params.size, 0)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "inner-shadow":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 0.75)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Angle"
						param="angle"
						value={numberParam(params.angle, 120)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Distance"
						param="distance"
						value={numberParam(params.distance, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Choke"
						param="choke"
						value={numberParam(params.choke, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Size"
						param="size"
						value={numberParam(params.size, 0)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "outer-glow":
		case "inner-glow":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 0.75)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Spread"
						param="spread"
						value={numberParam(params.spread, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Size"
						param="size"
						value={numberParam(params.size, 0)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "bevel-emboss":
			return (
				<>
					<SelectField
						kind={kind}
						label="Style"
						param="style"
						value={stringParam(params.style, "inner-bevel")}
						options={[
							{ value: "inner-bevel", label: "Inner Bevel" },
							{ value: "outer-bevel", label: "Outer Bevel" },
							{ value: "emboss", label: "Emboss" },
							{ value: "pillow-emboss", label: "Pillow Emboss" },
							{ value: "stroke-emboss", label: "Stroke Emboss" },
						]}
						onParamsChange={onParamsChange}
					/>
					<SelectField
						kind={kind}
						label="Technique"
						param="technique"
						value={stringParam(params.technique, "smooth")}
						options={[
							{ value: "smooth", label: "Smooth" },
							{ value: "chisel-hard", label: "Chisel Hard" },
							{ value: "chisel-soft", label: "Chisel Soft" },
						]}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Depth"
						param="depth"
						value={numberParam(params.depth, 1)}
						onParamsChange={onParamsChange}
					/>
					<SelectField
						kind={kind}
						label="Direction"
						param="direction"
						value={stringParam(params.direction, "up")}
						options={[
							{ value: "up", label: "Up" },
							{ value: "down", label: "Down" },
						]}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Size"
						param="size"
						value={numberParam(params.size, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Soften"
						param="soften"
						value={numberParam(params.soften, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Angle"
						param="angle"
						value={numberParam(params.angle, 120)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Altitude"
						param="altitude"
						value={numberParam(params.altitude, 30)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "satin":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 0.5)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Angle"
						param="angle"
						value={numberParam(params.angle, 19)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Distance"
						param="distance"
						value={numberParam(params.distance, 0)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Size"
						param="size"
						value={numberParam(params.size, 0)}
						onParamsChange={onParamsChange}
					/>
					<CheckboxField
						kind={kind}
						label="Invert"
						param="invert"
						checked={booleanParam(params.invert, false)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "color-overlay":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 1)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "gradient-overlay":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 1)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Angle"
						param="angle"
						value={numberParam(params.angle, 90)}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Scale"
						param="scale"
						value={numberParam(params.scale, 1)}
						onParamsChange={onParamsChange}
					/>
					<CheckboxField
						kind={kind}
						label="Reverse"
						param="reverse"
						checked={booleanParam(params.reverse, false)}
						onParamsChange={onParamsChange}
					/>
					<CheckboxField
						kind={kind}
						label="Dither"
						param="dither"
						checked={booleanParam(params.dither, false)}
						onParamsChange={onParamsChange}
					/>
					<CheckboxField
						kind={kind}
						label="Align"
						param="align"
						checked={booleanParam(params.align, true)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "pattern-overlay":
			return (
				<>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 1)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<NumberField
						kind={kind}
						label="Scale"
						param="scale"
						value={numberParam(params.scale, 1)}
						onParamsChange={onParamsChange}
					/>
					<CheckboxField
						kind={kind}
						label="Link With Layer"
						param="link"
						checked={booleanParam(params.link, true)}
						onParamsChange={onParamsChange}
					/>
				</>
			);
		case "stroke":
			return (
				<>
					<NumberField
						kind={kind}
						label="Stroke Size"
						param="size"
						value={numberParam(params.size, 1)}
						onParamsChange={onParamsChange}
					/>
					<SelectField
						kind={kind}
						label="Position"
						param="position"
						value={stringParam(params.position, "outside")}
						options={[
							{ value: "outside", label: "Outside" },
							{ value: "inside", label: "Inside" },
							{ value: "center", label: "Center" },
						]}
						onParamsChange={onParamsChange}
					/>
					<BlendModeField kind={kind} params={params} onParamsChange={onParamsChange} />
					<RangeField
						kind={kind}
						label="Opacity"
						param="opacity"
						value={numberParam(params.opacity, 1)}
						min={0}
						max={1}
						step={0.01}
						onParamsChange={onParamsChange}
					/>
					<SelectField
						kind={kind}
						label="Fill Type"
						param="fillType"
						value={stringParam(params.fillType, "color")}
						options={[
							{ value: "color", label: "Color" },
							{ value: "gradient", label: "Gradient" },
							{ value: "pattern", label: "Pattern" },
						]}
						onParamsChange={onParamsChange}
					/>
				</>
			);
	}
}

function BlendModeField({
	kind,
	params,
	onParamsChange,
}: {
	kind: LayerStyleKind;
	params: Record<string, unknown>;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<SelectField
			kind={kind}
			label="Blend Mode"
			param="blendMode"
			value={stringParam(params.blendMode, "normal")}
			options={blendModeOptions}
			onParamsChange={onParamsChange}
		/>
	);
}

function NumberField({
	kind,
	label,
	param,
	value,
	onParamsChange,
}: {
	kind: LayerStyleKind;
	label: string;
	param: string;
	value: number;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<label className="block">
			<span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
				{label}
			</span>
			<input
				aria-label={label}
				className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
				type="number"
				value={value}
				onChange={(event) => {
					const nextValue = parseFiniteNumber(event.target.value);
					if (nextValue === null) {
						return;
					}
					onParamsChange(kind, { [param]: nextValue });
				}}
			/>
		</label>
	);
}

function RangeField({
	kind,
	label,
	param,
	value,
	min,
	max,
	step,
	onParamsChange,
}: {
	kind: LayerStyleKind;
	label: string;
	param: string;
	value: number;
	min: number;
	max: number;
	step: number;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<label className="block">
			<div className="mb-0.5 flex items-center justify-between text-[10px] uppercase tracking-[0.15em] text-slate-500">
				<span>{label}</span>
				<span className="text-slate-300">{value}</span>
			</div>
			<input
				aria-label={label}
				className="h-1.5 w-full accent-cyan-400 focus-visible:outline-none"
				type="range"
				min={min}
				max={max}
				step={step}
				value={value}
				onChange={(event) =>
					onParamsChange(kind, { [param]: Number(event.target.value) })
				}
			/>
		</label>
	);
}

function SelectField({
	kind,
	label,
	param,
	value,
	options,
	onParamsChange,
}: {
	kind: LayerStyleKind;
	label: string;
	param: string;
	value: string;
	options: { value: string; label: string }[];
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<label className="block">
			<span className="mb-0.5 block text-[10px] uppercase tracking-[0.15em] text-slate-500">
				{label}
			</span>
			<select
				aria-label={label}
				className="w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/30 px-1.5 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
				value={value}
				onChange={(event) => onParamsChange(kind, { [param]: event.target.value })}
			>
				{options.map((option) => (
					<option key={option.value} value={option.value}>
						{option.label}
					</option>
				))}
			</select>
		</label>
	);
}

function CheckboxField({
	kind,
	label,
	param,
	checked,
	onParamsChange,
}: {
	kind: LayerStyleKind;
	label: string;
	param: string;
	checked: boolean;
	onParamsChange: (kind: LayerStyleKind, params: Record<string, unknown>) => void;
}) {
	return (
		<label className="flex items-center gap-2 text-[11px] text-slate-300">
			<input
				aria-label={label}
				type="checkbox"
				className="accent-cyan-400"
				checked={checked}
				onChange={(event) => onParamsChange(kind, { [param]: event.target.checked })}
			/>
			{label}
		</label>
	);
}

function numberParam(value: unknown, fallback: number): number {
	return typeof value === "number" ? value : fallback;
}

function stringParam(value: unknown, fallback: string): string {
	return typeof value === "string" ? value : fallback;
}

function booleanParam(value: unknown, fallback: boolean): boolean {
	return typeof value === "boolean" ? value : fallback;
}

function parseFiniteNumber(value: string): number | null {
	if (value.trim() === "") {
		return null;
	}
	const parsed = Number(value);
	return Number.isFinite(parsed) ? parsed : null;
}

function BlendIfChannelField({
	label,
	value,
	onMinChange,
	onMaxChange,
}: {
	label: string;
	value: [number, number];
	onMinChange: (value: number) => void;
	onMaxChange: (value: number) => void;
}) {
	const updateMin = (next: string) => {
		const parsed = parseFiniteNumber(next);
		if (parsed === null) {
			return;
		}
		onMinChange(Math.max(0, Math.min(255, parsed)));
	};

	const updateMax = (next: string) => {
		const parsed = parseFiniteNumber(next);
		if (parsed === null) {
			return;
		}
		onMaxChange(Math.max(0, Math.min(255, parsed)));
	};

	return (
		<div className="grid grid-cols-[4.5rem_1fr_1fr] items-center gap-2">
			<span className="text-[11px] text-slate-400">{label}</span>
			<label className="text-[10px] text-slate-500">
				From
				<input
					type="number"
					min={0}
					max={255}
					step={1}
					value={value[0]}
					onChange={(event) => updateMin(event.target.value)}
					className="ml-2 h-6 w-16 rounded border border-white/10 bg-black/30 px-1 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
				/>
			</label>
			<label className="text-[10px] text-slate-500">
				To
				<input
					type="number"
					min={0}
					max={255}
					step={1}
					value={value[1]}
					onChange={(event) => updateMax(event.target.value)}
					className="ml-2 h-6 w-16 rounded border border-white/10 bg-black/30 px-1 py-1 text-[11px] text-slate-200 focus-visible:outline-none"
				/>
			</label>
		</div>
	);
}
