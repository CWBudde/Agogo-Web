import type { CreateDocumentCommand } from "@agogo/proto";
import type { Dispatch, SetStateAction } from "react";
import { Field, fieldClassName } from "@/components/field";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import {
  type DocumentUnit,
  formatDimension,
  pixelsToUnit,
  unitSteps,
  unitToPixels,
} from "@/lib/units";
import { parseNumericInput } from "@/lib/utils";
import { useColorState } from "@/state/color-state";
import { useDialogState } from "@/state/dialog-state";
import { useViewState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";

const presets = [
  { id: "web", label: "Web", width: 1920, height: 1080, resolution: 72 },
  { id: "photo", label: "Photo", width: 4032, height: 3024, resolution: 300 },
  { id: "print", label: "Print", width: 2480, height: 3508, resolution: 300 },
  { id: "square", label: "Custom", width: 2048, height: 2048, resolution: 144 },
];

/**
 * Create Document dialog. The document draft is App-owned (it also feeds
 * useDocumentIo and mirrors the engine document), so it is passed in as
 * draft/setDraft rather than owned locally.
 */
export function NewDocumentDialog({
  draft,
  setDraft,
}: {
  draft: CreateDocumentCommand;
  setDraft: Dispatch<SetStateAction<CreateDocumentCommand>>;
}) {
  const engine = useEngine();
  const { newDocumentOpen, setNewDocumentOpen } = useDialogState();
  const { documentUnit, setDocumentUnit } = useViewState();
  const { setColorSamplerPoints } = useColorState();

  const widthValue = formatDimension(
    pixelsToUnit(draft.width, draft.resolution, documentUnit),
    documentUnit,
  );
  const heightValue = formatDimension(
    pixelsToUnit(draft.height, draft.resolution, documentUnit),
    documentUnit,
  );

  return (
    <Dialog
      open={newDocumentOpen}
      onClose={() => setNewDocumentOpen(false)}
      title="Create Document"
      description="Presets, dimensions, resolution, color mode, bit depth, and background feed the Go engine document manager."
    >
      <div className="grid gap-4 md:grid-cols-[11rem_minmax(0,1fr)]">
        <div className="space-y-[var(--ui-gap-2)]">
          {presets.map((preset) => (
            <button
              key={preset.id}
              type="button"
              className="w-full rounded-[var(--ui-radius-sm)] border border-white/8 bg-panel-soft px-3 py-2 text-left transition hover:border-cyan-400/30 hover:bg-cyan-400/8"
              onClick={() =>
                setDraft((current) => ({
                  ...current,
                  width: preset.width,
                  height: preset.height,
                  resolution: preset.resolution,
                }))
              }
            >
              <div className="text-[12px] font-medium text-slate-100">{preset.label}</div>
              <div className="mt-1 text-[11px] text-slate-400">
                {preset.width} x {preset.height} · {preset.resolution} DPI
              </div>
            </button>
          ))}
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Name">
            <input
              className={fieldClassName}
              value={draft.name}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
            />
          </Field>
          <Field label="Background">
            <select
              className={fieldClassName}
              value={draft.background}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  background: event.target.value as CreateDocumentCommand["background"],
                }))
              }
            >
              <option value="transparent">Transparent</option>
              <option value="white">White</option>
              <option value="color">Color</option>
            </select>
          </Field>
          <Field label="Units">
            <select
              className={fieldClassName}
              value={documentUnit}
              onChange={(event) => setDocumentUnit(event.target.value as DocumentUnit)}
            >
              <option value="px">Pixels</option>
              <option value="in">Inches</option>
              <option value="cm">Centimeters</option>
              <option value="mm">Millimeters</option>
            </select>
          </Field>
          <Field label={`Width (${documentUnit})`}>
            <input
              className={fieldClassName}
              type="number"
              min={documentUnit === "px" ? 1 : 0.01}
              step={unitSteps[documentUnit]}
              value={widthValue}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  width: Math.max(
                    1,
                    Math.round(
                      unitToPixels(
                        parseNumericInput(
                          event.target.value,
                          pixelsToUnit(current.width, current.resolution, documentUnit),
                        ),
                        current.resolution,
                        documentUnit,
                      ),
                    ),
                  ),
                }))
              }
            />
          </Field>
          <Field label={`Height (${documentUnit})`}>
            <input
              className={fieldClassName}
              type="number"
              min={documentUnit === "px" ? 1 : 0.01}
              step={unitSteps[documentUnit]}
              value={heightValue}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  height: Math.max(
                    1,
                    Math.round(
                      unitToPixels(
                        parseNumericInput(
                          event.target.value,
                          pixelsToUnit(current.height, current.resolution, documentUnit),
                        ),
                        current.resolution,
                        documentUnit,
                      ),
                    ),
                  ),
                }))
              }
            />
          </Field>
          <Field label="Resolution (DPI)">
            <input
              className={fieldClassName}
              type="number"
              min={1}
              value={draft.resolution}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  resolution: parseNumericInput(event.target.value, current.resolution),
                }))
              }
            />
          </Field>
          <Field label="Bit Depth">
            <select
              className={fieldClassName}
              value={draft.bitDepth}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  bitDepth: parseNumericInput(event.target.value, current.bitDepth) as 8 | 16 | 32,
                }))
              }
            >
              <option value={8}>8-bit</option>
              <option value={16}>16-bit</option>
              <option value={32}>32-bit</option>
            </select>
          </Field>
          <Field label="Color Mode">
            <select
              className={fieldClassName}
              value={draft.colorMode}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  colorMode: event.target.value as CreateDocumentCommand["colorMode"],
                }))
              }
            >
              <option value="rgb">RGB</option>
              <option value="gray">Grayscale</option>
            </select>
          </Field>
        </div>
      </div>

      <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
        <Button variant="ghost" size="sm" onClick={() => setNewDocumentOpen(false)}>
          Cancel
        </Button>
        <Button
          size="sm"
          onClick={() => {
            engine.createDocument(draft);
            setColorSamplerPoints([]);
            setNewDocumentOpen(false);
          }}
        >
          Create Document
        </Button>
      </div>
    </Dialog>
  );
}
