import { CommandID, type FilterParams } from "@agogo/proto";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { parseNumericInput } from "@/lib/utils";
import { defaultFilterParams, type FilterParamField, getFilterDefinition } from "./filter-catalog";

/** Full-resolution live preview. The engine re-applies at full res on commit. */
const PREVIEW_SCALE = 1;

/** The slice of the engine the dialog dispatches through. */
export interface FilterDialogEngine {
  dispatchCommand(commandId: number, payload?: unknown): unknown;
}

export interface FilterDialogProps {
  /** Registered filter id whose dialog to show (must exist in the catalog). */
  filterId: string;
  engine: FilterDialogEngine;
  /** Close the dialog (the parent owns the open flag). */
  onClose: () => void;
  /** Called after the filter is applied/committed, for Last Filter + Fade. */
  onApplied: (id: string, name: string) => void;
}

/**
 * Generic parameter dialog for any registered filter, driven by the static
 * filter catalog. It manages the engine's preview lifecycle:
 *
 *   open → PreviewFilter (defaults)   [snapshot taken]
 *   tweak a field → PreviewFilter     [restore + re-apply]
 *   Preview off → CancelFilterPreview [show original]
 *   Apply → CommitFilterPreview (or ApplyFilter when preview is off)
 *   Cancel/Escape/unmount → CancelFilterPreview
 */
export function FilterDialog({ filterId, engine, onClose, onApplied }: FilterDialogProps) {
  const def = getFilterDefinition(filterId);

  const [params, setParams] = useState<FilterParams>(() => (def ? defaultFilterParams(def) : {}));
  const [previewOn, setPreviewOn] = useState(true);

  // Engine identity is stable across frames, but ref it so the preview effect
  // never resubscribes on it and unmount cleanup always sees the latest.
  const engineRef = useRef(engine);
  const previewOnRef = useRef(previewOn);
  const settledRef = useRef(false);
  useEffect(() => {
    engineRef.current = engine;
    previewOnRef.current = previewOn;
  });

  // Live preview: (re)render whenever the params or preview toggle change.
  useEffect(() => {
    if (!previewOn) {
      return;
    }
    engineRef.current.dispatchCommand(CommandID.PreviewFilter, {
      filterId,
      params,
      scale: PREVIEW_SCALE,
    });
  }, [filterId, params, previewOn]);

  // Safety net: an unexpected unmount with a live preview restores the pixels.
  useEffect(
    () => () => {
      if (!settledRef.current && previewOnRef.current) {
        engineRef.current.dispatchCommand(CommandID.CancelFilterPreview, {});
      }
    },
    [],
  );

  if (!def) {
    return null;
  }

  const setField = (name: string, value: number | string | boolean) => {
    setParams((prev) => ({ ...prev, [name]: value }));
  };

  const togglePreview = (on: boolean) => {
    setPreviewOn(on);
    if (!on) {
      // Restore the original pixels; the effect will re-preview when toggled on.
      engineRef.current.dispatchCommand(CommandID.CancelFilterPreview, {});
    }
  };

  const handleApply = () => {
    settledRef.current = true;
    if (previewOn) {
      engineRef.current.dispatchCommand(CommandID.CommitFilterPreview, {});
    } else {
      engineRef.current.dispatchCommand(CommandID.ApplyFilter, { filterId, params });
    }
    onApplied(def.id, def.name);
    onClose();
  };

  const handleCancel = () => {
    settledRef.current = true;
    if (previewOn) {
      engineRef.current.dispatchCommand(CommandID.CancelFilterPreview, {});
    }
    onClose();
  };

  return (
    <Dialog
      open
      onClose={handleCancel}
      title={def.name}
      description="Adjust the parameters below. The preview updates the active layer live; Apply is undoable."
      className="max-w-sm"
    >
      <div className="space-y-4">
        {def.fields.length === 0 ? (
          <p className="text-[12px] text-slate-400">This filter has no adjustable parameters.</p>
        ) : (
          <div className="space-y-4">
            {def.fields.map((field) => (
              <FilterFieldControl
                key={field.name}
                field={field}
                value={params[field.name]}
                onChange={(value) => setField(field.name, value)}
              />
            ))}
          </div>
        )}

        <label className="flex items-center gap-2 text-[12px] text-slate-300">
          <input
            type="checkbox"
            className="accent-accent"
            aria-label="Preview"
            checked={previewOn}
            onChange={(event) => togglePreview(event.target.checked)}
          />
          <span>Preview</span>
        </label>

        <div className="flex justify-end gap-2 border-t border-white/8 pt-3">
          <Button variant="secondary" size="sm" onClick={handleCancel}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleApply}>
            Apply
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function FilterFieldControl({
  field,
  value,
  onChange,
}: {
  field: FilterParamField;
  value: number | string | boolean | undefined;
  onChange: (value: number | string | boolean) => void;
}) {
  if (field.kind === "checkbox") {
    const checked = value === undefined ? field.default : Boolean(value);
    return (
      <label className="flex items-center gap-2 text-[12px] text-slate-300">
        <input
          type="checkbox"
          className="accent-accent"
          aria-label={field.label}
          checked={checked}
          onChange={(event) => onChange(event.target.checked)}
        />
        <span>{field.label}</span>
      </label>
    );
  }

  if (field.kind === "select") {
    const current = value === undefined ? field.default : String(value);
    return (
      <label className="flex flex-col gap-1 text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{field.label}</span>
        <select
          className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
          aria-label={field.label}
          value={current}
          onChange={(event) => onChange(event.target.value)}
        >
          {field.options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </label>
    );
  }

  const numericValue = typeof value === "number" ? value : field.default;
  const step = field.step ?? 1;
  const clamp = (n: number) => Math.max(field.min, Math.min(field.max, n));

  return (
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{field.label}</span>
        <span className="text-slate-300">
          {numericValue}
          {field.unit ? ` ${field.unit}` : ""}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <input
          className="h-2 flex-1 accent-accent focus-visible:outline-none"
          type="range"
          aria-label={field.label}
          min={field.min}
          max={field.max}
          step={step}
          value={numericValue}
          onChange={(event) => onChange(clamp(parseNumericInput(event.target.value, numericValue)))}
        />
        <input
          className="h-[var(--ui-h-sm)] w-20 rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 outline-none"
          type="number"
          min={field.min}
          max={field.max}
          step={step}
          value={numericValue}
          onChange={(event) => onChange(clamp(parseNumericInput(event.target.value, numericValue)))}
        />
      </div>
    </label>
  );
}
