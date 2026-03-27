import type { CreateDocumentCommand } from "@agogo/proto";
import { useState } from "react";
import { EditorCanvas } from "@/components/editor-canvas";
import { LayersPanel } from "@/components/layers-panel";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";
import { useEngine } from "@/wasm/context";

const menuItems = ["File", "Edit", "Image", "Layer", "Select", "Filter", "View", "Window", "Help"];

const toolItems = [
  { id: "move", label: "Move", glyph: "M" },
  { id: "marquee", label: "Marquee", glyph: "R" },
  { id: "lasso", label: "Lasso", glyph: "L" },
  { id: "brush", label: "Brush", glyph: "B" },
  { id: "eraser", label: "Eraser", glyph: "E" },
  { id: "type", label: "Type", glyph: "T" },
  { id: "shape", label: "Shape", glyph: "S" },
  { id: "hand", label: "Hand", glyph: "H" },
  { id: "zoom", label: "Zoom", glyph: "Z" },
];

const defaultDocumentDraft: CreateDocumentCommand = {
  name: "Untitled",
  width: 1920,
  height: 1080,
  resolution: 72,
  colorMode: "rgb",
  bitDepth: 8,
  background: "transparent",
};

const presets = [
  { id: "web", label: "Web", width: 1920, height: 1080, resolution: 72 },
  { id: "photo", label: "Photo", width: 4032, height: 3024, resolution: 300 },
  { id: "print", label: "Print", width: 2480, height: 3508, resolution: 300 },
  { id: "square", label: "Custom", width: 2048, height: 2048, resolution: 144 },
];

type DocumentUnit = "px" | "in" | "cm" | "mm";

const unitSteps: Record<DocumentUnit, number> = {
  px: 1,
  in: 0.01,
  cm: 0.1,
  mm: 1,
};

function pixelsToUnit(pixels: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return pixels / resolution;
    case "cm":
      return (pixels / resolution) * 2.54;
    case "mm":
      return (pixels / resolution) * 25.4;
    default:
      return pixels;
  }
}

function unitToPixels(value: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return value * resolution;
    case "cm":
      return (value / 2.54) * resolution;
    case "mm":
      return (value / 25.4) * resolution;
    default:
      return value;
  }
}

function formatDimension(value: number, unit: DocumentUnit) {
  if (unit === "px" || unit === "mm") {
    return Math.round(value).toString();
  }
  return value.toFixed(2);
}

export default function App() {
  const engine = useEngine();
  const render = engine.render;
  const [activeTool, setActiveTool] = useState("brush");
  const [activePanel, setActivePanel] = useState<"layers" | "history" | "properties" | "navigator">(
    "layers",
  );
  const [newDocumentOpen, setNewDocumentOpen] = useState(false);
  const [draft, setDraft] = useState<CreateDocumentCommand>(defaultDocumentDraft);
  const [cursor, setCursor] = useState<{ x: number; y: number } | null>(null);
  const [isPanMode, setIsPanMode] = useState(false);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [panelWidth, setPanelWidth] = useState(352);
  const [documentUnit, setDocumentUnit] = useState<DocumentUnit>("px");

  useKeyboardShortcuts({
    onPanModeChange: setIsPanMode,
    onZoomIn() {
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom * 1.1);
    },
    onZoomOut() {
      if (!render) {
        return;
      }
      engine.setZoom(render.viewport.zoom / 1.1);
    },
    onFitToView() {
      engine.fitToView();
    },
    onUndo() {
      engine.undo();
    },
    onRedo() {
      engine.redo();
    },
  });

  const documentSize = render
    ? `${render.uiMeta.documentWidth} x ${render.uiMeta.documentHeight}`
    : "No document";
  const zoomPercent = render ? `${Math.round(render.viewport.zoom * 100)}%` : "0%";
  const cursorText = cursor ? `${cursor.x}, ${cursor.y}` : "Outside";
  const statusText = render?.uiMeta.statusText ?? "Waiting for engine";
  const historyEntries = render?.uiMeta.history ?? [];
  const currentHistoryIndex = render?.uiMeta.currentHistoryIndex ?? 0;
  const widthValue = formatDimension(
    pixelsToUnit(draft.width, draft.resolution, documentUnit),
    documentUnit,
  );
  const heightValue = formatDimension(
    pixelsToUnit(draft.height, draft.resolution, documentUnit),
    documentUnit,
  );

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,rgba(55,65,81,0.35),transparent_36%),linear-gradient(180deg,#05070c_0%,#0a0d14_38%,#070910_100%)] text-slate-100">
      <div className="mx-auto flex min-h-screen max-w-[1920px] flex-col gap-4 px-4 py-4">
        <header className="editor-surface flex flex-wrap items-center justify-between gap-4 rounded-[1.4rem] px-4 py-3">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-300 to-blue-500 font-black text-slate-950">
              A
            </div>
            <div>
              <p className="text-[11px] uppercase tracking-[0.32em] text-slate-500">
                Agogo Web Editor
              </p>
              <p className="text-sm text-slate-300">{statusText}</p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="secondary" onClick={() => setNewDocumentOpen(true)}>
              New
            </Button>
            <Button variant="secondary" onClick={() => engine.fitToView()}>
              Fit
            </Button>
            <Button
              variant="secondary"
              onClick={() => engine.undo()}
              disabled={!render?.uiMeta.canUndo}
            >
              Undo
            </Button>
            <Button onClick={() => engine.redo()} disabled={!render?.uiMeta.canRedo}>
              Redo
            </Button>
          </div>
        </header>

        <nav className="editor-surface flex flex-wrap items-center gap-2 rounded-[1.2rem] px-3 py-2">
          {menuItems.map((item) => (
            <button
              key={item}
              className="rounded-lg px-3 py-2 text-sm text-slate-300 transition hover:bg-white/5 hover:text-white"
              type="button"
            >
              {item}
            </button>
          ))}
        </nav>

        <div className="editor-surface flex flex-wrap items-center justify-between gap-3 rounded-[1.2rem] px-4 py-3">
          <div>
            <p className="text-[11px] uppercase tracking-[0.28em] text-slate-500">Options Bar</p>
            <p className="text-sm text-slate-300">
              Active tool: {isPanMode ? "Hand (temporary)" : activeTool}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.24em] text-slate-500">
            <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-slate-300">
              {zoomPercent}
            </span>
            <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-slate-300">
              {documentSize}
            </span>
            <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-slate-300">
              {render?.viewport.rotation.toFixed(0) ?? 0}°
            </span>
          </div>
        </div>

        <section
          className="grid flex-1 gap-4"
          style={{
            gridTemplateColumns: `4.5rem minmax(0,1fr) ${panelCollapsed ? "3.75rem" : `${panelWidth}px`}`,
          }}
        >
          <aside className="editor-surface flex flex-col items-center gap-2 rounded-[1.2rem] px-2 py-3">
            {toolItems.map((tool) => {
              const active = (isPanMode && tool.id === "hand") || activeTool === tool.id;
              return (
                <button
                  key={tool.id}
                  type="button"
                  className={[
                    "flex h-11 w-11 items-center justify-center rounded-xl border text-[11px] font-medium transition",
                    active
                      ? "border-cyan-400/40 bg-cyan-400/10 text-cyan-100"
                      : "border-white/8 bg-white/[0.03] text-slate-400 hover:border-white/15 hover:bg-white/5 hover:text-slate-100",
                  ].join(" ")}
                  title={tool.label}
                  onClick={() => {
                    setActiveTool(tool.id);
                    if (tool.id !== "hand") {
                      setIsPanMode(false);
                    }
                  }}
                >
                  {tool.glyph}
                </button>
              );
            })}
          </aside>

          <main className="flex min-w-0 flex-col gap-4">
            <div className="min-h-[42rem] flex-1">
              <EditorCanvas
                isPanMode={isPanMode || activeTool === "hand"}
                isZoomTool={activeTool === "zoom"}
                onCursorChange={setCursor}
              />
            </div>
          </main>

          <aside className="editor-surface relative flex min-h-[42rem] flex-col rounded-[1.2rem] p-3">
            <div
              className="absolute inset-y-3 left-0 w-3 -translate-x-1/2 cursor-col-resize"
              onPointerDown={(event) => {
                if (panelCollapsed) {
                  return;
                }
                const startX = event.clientX;
                const startWidth = panelWidth;
                const handleMove = (moveEvent: PointerEvent) => {
                  setPanelWidth(
                    Math.min(520, Math.max(280, startWidth - (moveEvent.clientX - startX))),
                  );
                };
                const handleUp = () => {
                  window.removeEventListener("pointermove", handleMove);
                  window.removeEventListener("pointerup", handleUp);
                };
                window.addEventListener("pointermove", handleMove);
                window.addEventListener("pointerup", handleUp);
              }}
            >
              <div className="mx-auto h-full w-px bg-white/10" />
            </div>

            <div className="flex items-center justify-between gap-2">
              <div className="flex gap-2">
                {[
                  ["layers", "Layers"],
                  ["history", "History"],
                  ["properties", "Properties"],
                  ["navigator", "Navigator"],
                ].map(([id, label]) => (
                  <button
                    key={id}
                    type="button"
                    className={[
                      "rounded-xl px-3 py-2 text-xs font-medium transition",
                      activePanel === id
                        ? "bg-white/10 text-white"
                        : "text-slate-400 hover:bg-white/5 hover:text-slate-100",
                    ].join(" ")}
                    onClick={() => setActivePanel(id as typeof activePanel)}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <Button
                variant="ghost"
                className="h-9 px-3 text-xs"
                onClick={() => setPanelCollapsed((current) => !current)}
              >
                {panelCollapsed ? "Open" : "Collapse"}
              </Button>
            </div>

            {panelCollapsed ? (
              <div className="mt-3 flex flex-1 flex-col items-center justify-center gap-3 rounded-2xl border border-white/10 bg-white/[0.03] py-6">
                <span className="[writing-mode:vertical-rl] text-xs uppercase tracking-[0.28em] text-slate-500">
                  Panels
                </span>
              </div>
            ) : (
              <div className="mt-3 flex-1 rounded-2xl border border-white/10 bg-white/[0.03] p-4">
                {activePanel === "layers" ? (
                  <LayersPanel
                    engine={engine}
                    layers={render?.uiMeta.layers ?? []}
                    activeLayerId={render?.uiMeta.activeLayerId ?? null}
                    documentWidth={render?.uiMeta.documentWidth ?? draft.width}
                    documentHeight={render?.uiMeta.documentHeight ?? draft.height}
                  />
                ) : null}

                {activePanel === "history" ? (
                  <div className="space-y-3">
                    <PanelTitle
                      title="History"
                      subtitle="Viewport and document commands are now tracked in-engine."
                    />
                    <div className="flex justify-end">
                      <Button
                        variant="secondary"
                        className="h-9 px-3 text-xs"
                        disabled={historyEntries.length === 0}
                        onClick={() => engine.clearHistory()}
                      >
                        Clear History
                      </Button>
                    </div>
                    <div className="space-y-2">
                      {historyEntries.length === 0 ? (
                        <p className="text-sm text-slate-400">No history entries yet.</p>
                      ) : (
                        historyEntries.map((entry) => (
                          <button
                            key={entry.id}
                            type="button"
                            className={[
                              "w-full rounded-xl border px-3 py-2 text-left text-sm transition",
                              entry.id === currentHistoryIndex
                                ? "border-cyan-400/30 bg-cyan-400/10 text-slate-100"
                                : entry.state === "undone"
                                  ? "border-white/6 bg-black/10 text-slate-500 hover:border-white/12 hover:text-slate-300"
                                  : "border-white/8 bg-black/10 text-slate-200 hover:border-white/12 hover:bg-black/20",
                            ].join(" ")}
                            onClick={() => engine.jumpHistory(entry.id)}
                          >
                            {entry.description}
                          </button>
                        ))
                      )}
                    </div>
                  </div>
                ) : null}

                {activePanel === "properties" ? (
                  <div className="space-y-3">
                    <PanelTitle
                      title="Properties"
                      subtitle="Document and viewport state exposed from the engine."
                    />
                    <PropertyRow label="Document" value={documentSize} />
                    <PropertyRow label="Zoom" value={zoomPercent} />
                    <PropertyRow
                      label="Rotation"
                      value={`${render?.viewport.rotation.toFixed(0) ?? 0}°`}
                    />
                    <PropertyRow label="DPI" value={draft.resolution.toString()} />
                    <div className="pt-2">
                      <label
                        htmlFor="rotate-view-range"
                        className="text-xs uppercase tracking-[0.24em] text-slate-500"
                      >
                        Rotate View
                      </label>
                      <input
                        id="rotate-view-range"
                        className="mt-2 h-2 w-full accent-cyan-400"
                        type="range"
                        min="0"
                        max="360"
                        value={render?.viewport.rotation ?? 0}
                        onChange={(event) => engine.setRotation(Number(event.target.value))}
                      />
                    </div>
                  </div>
                ) : null}

                {activePanel === "navigator" ? (
                  <div className="space-y-3">
                    <PanelTitle
                      title="Navigator"
                      subtitle="Mini viewport controls for Phase 1 pan and zoom."
                    />
                    <div className="rounded-2xl border border-white/10 bg-[linear-gradient(135deg,rgba(255,255,255,0.06),rgba(255,255,255,0.02))] p-4">
                      <div className="aspect-[4/3] rounded-xl border border-white/10 bg-[linear-gradient(135deg,rgba(14,165,233,0.18),rgba(15,23,42,0.8))]" />
                      <label
                        htmlFor="navigator-zoom-range"
                        className="mt-4 block text-xs uppercase tracking-[0.24em] text-slate-500"
                      >
                        Zoom
                      </label>
                      <input
                        id="navigator-zoom-range"
                        className="mt-2 h-2 w-full accent-cyan-400"
                        type="range"
                        min="5"
                        max="3200"
                        step="5"
                        value={Math.round((render?.viewport.zoom ?? 1) * 100)}
                        onChange={(event) => engine.setZoom(Number(event.target.value) / 100)}
                      />
                    </div>
                  </div>
                ) : null}
              </div>
            )}
          </aside>
        </section>

        <footer className="editor-surface flex flex-wrap items-center justify-between gap-3 rounded-[1.2rem] px-4 py-3 text-sm text-slate-400">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-slate-200">{zoomPercent}</span>
            <span>{documentSize}</span>
            <span>Cursor {cursorText}</span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <span>
              Engine:{" "}
              {engine.status === "ready" ? `ready (#${engine.handle?.handle})` : engine.status}
            </span>
            <Separator orientation="vertical" className="h-4" />
            <span>
              Canvas {render?.viewport.canvasW ?? 0} x {render?.viewport.canvasH ?? 0}
            </span>
          </div>
        </footer>
      </div>

      <Dialog
        open={newDocumentOpen}
        title="Create Document"
        description="Presets, dimensions, resolution, color mode, bit depth, and background feed the Go engine document manager."
      >
        <div className="grid gap-6 md:grid-cols-[14rem_minmax(0,1fr)]">
          <div className="space-y-2">
            {presets.map((preset) => (
              <button
                key={preset.id}
                type="button"
                className="w-full rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3 text-left transition hover:border-cyan-400/30 hover:bg-cyan-400/10"
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    width: preset.width,
                    height: preset.height,
                    resolution: preset.resolution,
                  }))
                }
              >
                <div className="text-sm font-medium text-slate-100">{preset.label}</div>
                <div className="mt-1 text-xs text-slate-400">
                  {preset.width} x {preset.height} · {preset.resolution} DPI
                </div>
              </button>
            ))}
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name">
              <input
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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
                        unitToPixels(Number(event.target.value), current.resolution, documentUnit),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label={`Height (${documentUnit})`}>
              <input
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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
                        unitToPixels(Number(event.target.value), current.resolution, documentUnit),
                      ),
                    ),
                  }))
                }
              />
            </Field>
            <Field label="Resolution (DPI)">
              <input
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
                type="number"
                min={1}
                value={draft.resolution}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    resolution: Number(event.target.value),
                  }))
                }
              />
            </Field>
            <Field label="Bit Depth">
              <select
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
                value={draft.bitDepth}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    bitDepth: Number(event.target.value) as 8 | 16 | 32,
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
                className="h-11 rounded-xl border border-white/10 bg-black/20 px-3 text-sm"
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

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setNewDocumentOpen(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => {
              engine.createDocument(draft);
              setNewDocumentOpen(false);
            }}
          >
            Create Document
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

function PanelTitle({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div>
      <h2 className="text-sm font-semibold text-slate-100">{title}</h2>
      <p className="mt-1 text-sm leading-6 text-slate-400">{subtitle}</p>
    </div>
  );
}

function PropertyRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded-xl border border-white/8 bg-black/10 px-3 py-2 text-sm">
      <span className="text-slate-400">{label}</span>
      <span className="text-slate-100">{value}</span>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: label wraps its control via children (implicit label pattern)
    <label className="flex flex-col gap-2">
      <span className="text-xs uppercase tracking-[0.24em] text-slate-500">{label}</span>
      {children}
    </label>
  );
}
