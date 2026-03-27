import { CommandID, type CreateDocumentCommand, type ThumbnailEntry } from "@agogo/proto";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { EditorCanvas } from "@/components/editor-canvas";
import {
  BrushToolIcon,
  ClipboardIcon,
  CopyIcon,
  EraserToolIcon,
  FitScreenIcon,
  HandToolIcon,
  InfoIcon,
  LassoToolIcon,
  LayersIcon,
  MarqueeToolIcon,
  MoveToolIcon,
  NewDocumentIcon,
  OpenFolderIcon,
  PanelsIcon,
  RedoIcon,
  SaveIcon,
  ScissorsIcon,
  SelectionIcon,
  ShapeToolIcon,
  SlidersIcon,
  TypeToolIcon,
  UndoIcon,
  ZoomToolIcon,
} from "@/components/editor-icons";
import { LayersPanel } from "@/components/layers-panel";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { useKeyboardShortcuts } from "@/hooks/use-keyboard-shortcuts";
import { useEngine } from "@/wasm/context";

type MenuPreviewTone = "default" | "accent" | "muted";

type FileMenuActionId =
  | "new-document"
  | "open-project"
  | "open-recent"
  | "save-project"
  | "export-project"
  | "generate-assets";

type MenuPreviewItem = {
  label: string;
  shortcut?: string;
  tone?: MenuPreviewTone;
  actionId?: FileMenuActionId;
  disabled?: boolean;
};

type MenuPreviewMenu = {
  label: string;
  caption: string;
  align?: "left" | "right";
  sections: { title: string; items: MenuPreviewItem[] }[];
};

const menuItems: MenuPreviewMenu[] = [
  {
    label: "File",
    caption: "Document lifecycle and export flow preview.",
    sections: [
      {
        title: "Document",
        items: [
          {
            label: "New Document",
            shortcut: "Ctrl+N",
            tone: "accent",
            actionId: "new-document",
          },
          { label: "Open...", shortcut: "Ctrl+O", actionId: "open-project" },
          { label: "Open Recent", actionId: "open-recent" },
        ],
      },
      {
        title: "Output",
        items: [
          { label: "Save", shortcut: "Ctrl+S", actionId: "save-project" },
          { label: "Export As...", shortcut: "Ctrl+Shift+E", actionId: "export-project" },
          {
            label: "Generate Assets",
            tone: "muted",
            actionId: "generate-assets",
            disabled: true,
          },
        ],
      },
    ],
  },
  {
    label: "Edit",
    caption: "History, clipboard, and transform placeholders.",
    sections: [
      {
        title: "History",
        items: [
          { label: "Undo", shortcut: "Ctrl+Z", tone: "accent" },
          { label: "Redo", shortcut: "Ctrl+Shift+Z" },
        ],
      },
      {
        title: "Clipboard",
        items: [
          { label: "Cut", shortcut: "Ctrl+X" },
          { label: "Copy", shortcut: "Ctrl+C" },
          { label: "Paste", shortcut: "Ctrl+V" },
        ],
      },
    ],
  },
  {
    label: "Image",
    caption: "Canvas-wide operations and color management preview.",
    sections: [
      {
        title: "Adjustments",
        items: [{ label: "Levels..." }, { label: "Curves..." }, { label: "Hue/Saturation..." }],
      },
      {
        title: "Geometry",
        items: [{ label: "Image Size..." }, { label: "Canvas Size..." }, { label: "Trim" }],
      },
    ],
  },
  {
    label: "Layer",
    caption: "Layer stack actions matching the right-side dock.",
    sections: [
      {
        title: "Create",
        items: [
          { label: "New Layer", shortcut: "Shift+Ctrl+N", tone: "accent" },
          { label: "New Group" },
          { label: "Layer Mask" },
        ],
      },
      {
        title: "Arrange",
        items: [
          { label: "Duplicate Layer", shortcut: "Ctrl+J" },
          { label: "Merge Down", shortcut: "Ctrl+E" },
          { label: "Rasterize", tone: "muted" },
        ],
      },
    ],
  },
  {
    label: "Select",
    caption: "Selection workflows and edge refinement preview.",
    sections: [
      {
        title: "Global",
        items: [
          { label: "All", shortcut: "Ctrl+A" },
          { label: "Deselect", shortcut: "Ctrl+D" },
          { label: "Inverse", shortcut: "Shift+Ctrl+I" },
        ],
      },
      {
        title: "Refine",
        items: [
          { label: "Grow" },
          { label: "Feather..." },
          { label: "Select and Mask", tone: "muted" },
        ],
      },
    ],
  },
  {
    label: "Filter",
    caption: "Effect categories and future gallery entry points.",
    sections: [
      {
        title: "Recent",
        items: [
          { label: "Last Filter", shortcut: "Ctrl+F" },
          { label: "Fade Last Filter", tone: "muted" },
        ],
      },
      {
        title: "Families",
        items: [{ label: "Blur" }, { label: "Noise" }, { label: "Stylize" }],
      },
    ],
  },
  {
    label: "View",
    caption: "Viewport controls that mirror the current chrome.",
    sections: [
      {
        title: "Zoom",
        items: [
          { label: "Zoom In", shortcut: "Ctrl++", tone: "accent" },
          { label: "Zoom Out", shortcut: "Ctrl+-" },
          { label: "Fit on Screen", shortcut: "Ctrl+0" },
        ],
      },
      {
        title: "Overlays",
        items: [{ label: "Pixel Grid" }, { label: "Rulers" }, { label: "Guides", tone: "muted" }],
      },
    ],
  },
  {
    label: "Window",
    caption: "Dock and workspace organization preview.",
    align: "right",
    sections: [
      {
        title: "Panels",
        items: [{ label: "Layers", tone: "accent" }, { label: "Navigator" }, { label: "History" }],
      },
      {
        title: "Workspace",
        items: [{ label: "Essentials" }, { label: "Painting" }, { label: "Reset Workspace" }],
      },
    ],
  },
  {
    label: "Help",
    caption: "Support, onboarding, and diagnostics preview.",
    align: "right",
    sections: [
      {
        title: "Learn",
        items: [
          { label: "Welcome Tour" },
          { label: "Keyboard Shortcuts" },
          { label: "What’s New" },
        ],
      },
      {
        title: "Support",
        items: [
          { label: "Report Feedback" },
          { label: "System Info" },
          { label: "Release Notes", tone: "muted" },
        ],
      },
    ],
  },
];

const toolItems = [
  { id: "move", label: "Move", Icon: MoveToolIcon },
  { id: "marquee", label: "Marquee", Icon: MarqueeToolIcon },
  { id: "lasso", label: "Lasso", Icon: LassoToolIcon },
  { id: "brush", label: "Brush", Icon: BrushToolIcon },
  { id: "eraser", label: "Eraser", Icon: EraserToolIcon },
  { id: "type", label: "Type", Icon: TypeToolIcon },
  { id: "shape", label: "Shape", Icon: ShapeToolIcon },
  { id: "hand", label: "Hand", Icon: HandToolIcon },
  { id: "zoom", label: "Zoom", Icon: ZoomToolIcon },
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
type AuxPanel = "properties" | "history" | "navigator" | "channels";

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
  const menuBarRef = useRef<HTMLDivElement | null>(null);
  const projectInputRef = useRef<HTMLInputElement | null>(null);
  const lastSavedVersionRef = useRef<number>(0);
  const [activeTool, setActiveTool] = useState("brush");
  const [activeAuxPanel, setActiveAuxPanel] = useState<AuxPanel>("properties");
  const [newDocumentOpen, setNewDocumentOpen] = useState(false);
  const [openRecentOpen, setOpenRecentOpen] = useState(false);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [openMenu, setOpenMenu] = useState<string | null>(null);
  const [draft, setDraft] = useState<CreateDocumentCommand>(defaultDocumentDraft);
  const [cursor, setCursor] = useState<{ x: number; y: number } | null>(null);
  const [isPanMode, setIsPanMode] = useState(false);
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [panelWidth, setPanelWidth] = useState(328);
  const [documentUnit, setDocumentUnit] = useState<DocumentUnit>("px");
  const [layerThumbnails, setLayerThumbnails] = useState<Record<string, ThumbnailEntry>>({});
  const [isDragOver, setIsDragOver] = useState(false);

  const contentVersion = render?.uiMeta.contentVersion;
  useEffect(() => {
    if (contentVersion === undefined || !engine.handle) {
      return;
    }
    const result = engine.dispatchCommand(CommandID.GetLayerThumbnails);
    if (result?.thumbnails) {
      setLayerThumbnails(result.thumbnails);
    }
  }, [contentVersion, engine.dispatchCommand, engine.handle]);

  useEffect(() => {
    if (!engine.handle || contentVersion === undefined || contentVersion === 0) {
      return;
    }
    if (contentVersion - lastSavedVersionRef.current < AUTOSAVE_EVERY_N_VERSIONS) {
      return;
    }
    const base64Zip = engine.exportProject();
    if (!base64Zip) {
      return;
    }
    try {
      localStorage.setItem(AUTOSAVE_KEY, base64Zip);
      lastSavedVersionRef.current = contentVersion;
    } catch {
      // localStorage quota exceeded — silently skip
    }
  }, [contentVersion, engine.exportProject, engine.handle]);

  const downloadBlob = (blob: Blob, fileName: string) => {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  };

  const activeDocumentName = render?.uiMeta.activeDocumentName ?? draft.name;

  const openProjectPicker = () => {
    projectInputRef.current?.click();
  };

  const openNewDocumentDialog = () => {
    setNewDocumentOpen(true);
  };

  const saveProject = () => {
    const base64Zip = engine.exportProject();
    if (!base64Zip) {
      return;
    }
    const binary = atob(base64Zip);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    const fileName = `${activeDocumentName}.agp`;
    const blob = new Blob([bytes], { type: "application/zip" });
    downloadBlob(blob, fileName);
  };

  const openProject = async (file: File) => {
    const buffer = await file.arrayBuffer();
    const bytes = new Uint8Array(buffer);
    let payload: string;
    // PK magic bytes 0x50 0x4B = ZIP file
    if (bytes[0] === 0x50 && bytes[1] === 0x4b) {
      let binary = "";
      for (const byte of bytes) {
        binary += String.fromCharCode(byte);
      }
      payload = btoa(binary);
    } else {
      // Legacy JSON — pass as plain text
      payload = new TextDecoder().decode(bytes);
    }
    const imported = engine.importProject(payload);
    if (imported) {
      setDraft((current) => ({
        ...current,
        name: imported.uiMeta.activeDocumentName || current.name,
        width: imported.uiMeta.documentWidth || current.width,
        height: imported.uiMeta.documentHeight || current.height,
        background: imported.uiMeta.documentBackground as CreateDocumentCommand["background"],
      }));
    }
  };

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer.types.includes("Files")) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (event: React.DragEvent) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) {
      setIsDragOver(false);
    }
  };

  const handleDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    setIsDragOver(false);
    const file = event.dataTransfer.files[0];
    if (file && (file.name.endsWith(".agp") || file.type === "application/json")) {
      await openProject(file);
    }
  };

  const isFileMenuActionDisabled = (actionId: FileMenuActionId) => {
    switch (actionId) {
      case "save-project":
      case "export-project":
      case "generate-assets":
        return !render || actionId === "generate-assets";
      default:
        return false;
    }
  };

  const isMenuItemDisabled = (item: MenuPreviewItem) => {
    if (item.disabled) {
      return true;
    }
    if (!item.actionId) {
      return true;
    }
    return isFileMenuActionDisabled(item.actionId);
  };

  const handleFileMenuAction = (actionId: FileMenuActionId) => {
    if (isFileMenuActionDisabled(actionId)) {
      return;
    }

    setOpenMenu(null);

    switch (actionId) {
      case "new-document":
        openNewDocumentDialog();
        break;
      case "open-project":
        openProjectPicker();
        break;
      case "open-recent":
        setOpenRecentOpen(true);
        break;
      case "save-project":
        saveProject();
        break;
      case "export-project":
        setExportDialogOpen(true);
        break;
      default:
        break;
    }
  };

  useKeyboardShortcuts({
    onPanModeChange: setIsPanMode,
    onNewDocument() {
      openNewDocumentDialog();
    },
    onOpenDocument() {
      openProjectPicker();
    },
    onSaveDocument() {
      if (!isFileMenuActionDisabled("save-project")) {
        saveProject();
      }
    },
    onExportDocument() {
      if (!isFileMenuActionDisabled("export-project")) {
        setExportDialogOpen(true);
      }
    },
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

  useEffect(() => {
    if (!openMenu) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuBarRef.current?.contains(event.target as Node)) {
        setOpenMenu(null);
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpenMenu(null);
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [openMenu]);

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

  const activeToolLabel = isPanMode
    ? "Hand (temporary)"
    : (toolItems.find((tool) => tool.id === activeTool)?.label ?? activeTool);
  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#202329_0%,#171a1f_100%)] text-slate-100">
      <input
        ref={projectInputRef}
        type="file"
        accept=".agp,application/json"
        className="hidden"
        onChange={async (event) => {
          const file = event.target.files?.[0];
          if (!file) {
            return;
          }

          await openProject(file);
          event.target.value = "";
        }}
      />

      <div className="mx-auto min-h-screen max-w-[1920px] px-0">
        <div className="flex min-h-screen flex-col bg-[#1d2026]">
          <header className="editor-titlebar flex h-[34px] items-center justify-between gap-3 border-b border-border px-2">
            <div
              ref={menuBarRef}
              className="flex min-w-0 flex-nowrap items-center gap-3 overflow-visible"
            >
              <div className="flex shrink-0 items-center gap-2 pr-3">
                <div className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] bg-cyan-400/95 text-[11px] font-black text-slate-950">
                  A
                </div>
                <span className="font-serif text-[12px] font-semibold italic tracking-[0.01em] text-white">
                  Agogo Studio
                </span>
              </div>

              <nav className="flex min-w-0 flex-nowrap items-center gap-1 border-l border-white/8 pl-3">
                {menuItems.map((menu) => {
                  const isOpen = openMenu === menu.label;
                  return (
                    <div key={menu.label} className="relative shrink-0">
                      <button
                        type="button"
                        className={[
                          "px-1.5 py-1 text-[12px] transition",
                          isOpen ? "text-white" : "text-slate-400 hover:text-slate-100",
                        ].join(" ")}
                        aria-expanded={isOpen}
                        aria-haspopup="menu"
                        onClick={() =>
                          setOpenMenu((current) => (current === menu.label ? null : menu.label))
                        }
                        onPointerEnter={() => {
                          if (openMenu) {
                            setOpenMenu(menu.label);
                          }
                        }}
                      >
                        {menu.label}
                      </button>

                      {isOpen ? (
                        <MenuPreviewPanel
                          menu={menu}
                          isItemDisabled={isMenuItemDisabled}
                          onAction={handleFileMenuAction}
                        />
                      ) : null}
                    </div>
                  );
                })}
              </nav>
            </div>

            <div className="flex shrink-0 items-center gap-1">
              <Button variant="ghost" size="sm" onClick={openProjectPicker}>
                <OpenFolderIcon className="mr-1.5 h-3.5 w-3.5" />
                Open
              </Button>
              <Button variant="ghost" size="sm" onClick={saveProject}>
                <SaveIcon className="mr-1.5 h-3.5 w-3.5" />
                Save
              </Button>
              <Button variant="ghost" size="sm" onClick={openNewDocumentDialog}>
                <NewDocumentIcon className="mr-1.5 h-3.5 w-3.5" />
                New
              </Button>
              <Button variant="ghost" size="sm" onClick={() => engine.fitToView()}>
                <FitScreenIcon className="mr-1.5 h-3.5 w-3.5" />
                Fit
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => engine.undo()}
                disabled={!render?.uiMeta.canUndo}
              >
                <UndoIcon className="mr-1.5 h-3.5 w-3.5" />
                Undo
              </Button>
              <Button size="sm" onClick={() => engine.redo()} disabled={!render?.uiMeta.canRedo}>
                <RedoIcon className="mr-1.5 h-3.5 w-3.5" />
                Redo
              </Button>
            </div>
          </header>

          <div className="editor-chrome flex h-[36px] items-center justify-between gap-3 border-b border-border px-2">
            <div className="flex min-w-0 items-center gap-3 overflow-hidden">
              <ChromeLabel label="Tool">{activeToolLabel}</ChromeLabel>
              <ChromeLabel label="Document">{draft.name}</ChromeLabel>
              <ChromeLabel label="Status">{statusText}</ChromeLabel>
            </div>
            <div className="flex items-center gap-1 text-[11px] text-slate-300">
              <MetricChip value={zoomPercent} />
              <MetricChip value={documentSize} />
              <MetricChip value={`${render?.viewport.rotation.toFixed(0) ?? 0}°`} />
            </div>
          </div>

          <section
            className="grid min-h-0 flex-1"
            style={{
              gridTemplateColumns: `46px minmax(0,1fr) ${panelCollapsed ? "34px" : `${panelWidth}px`}`,
            }}
          >
            <aside className="editor-chrome editor-toolrail flex min-h-[36rem] flex-col items-center gap-[var(--ui-gap-1)] border-r border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
              {toolItems.map((tool) => {
                const active = (isPanMode && tool.id === "hand") || activeTool === tool.id;
                const ToolIcon = tool.Icon;
                return (
                  <button
                    key={tool.id}
                    type="button"
                    className={[
                      "flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] font-semibold transition",
                      active
                        ? "bg-cyan-400/14 text-cyan-100"
                        : "bg-transparent text-slate-400 hover:bg-white/6 hover:text-slate-100",
                    ].join(" ")}
                    title={tool.label}
                    onClick={() => {
                      setActiveTool(tool.id);
                      if (tool.id !== "hand") {
                        setIsPanMode(false);
                      }
                    }}
                  >
                    <ToolIcon className="h-4 w-4" />
                  </button>
                );
              })}
            </aside>

            <main className="editor-stage flex min-w-0 min-h-[36rem] flex-col p-[var(--ui-gap-2)]">
              <div className="flex items-center justify-between border-b border-border px-[var(--ui-gap-2)] pb-[var(--ui-gap-2)] text-[11px] text-slate-400">
                <div className="flex min-w-0 items-center gap-2 overflow-hidden">
                  <span className="truncate text-slate-200">
                    {draft.name}.agp {render ? `(Layer 1, RGB)` : ""}
                  </span>
                  <span>{zoomPercent}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span>
                    Canvas {render?.viewport.canvasW ?? 0} x {render?.viewport.canvasH ?? 0}
                  </span>
                </div>
              </div>
              <section
                className={`min-h-0 flex-1 pt-[var(--ui-gap-2)]${isDragOver ? " ring-2 ring-inset ring-blue-500" : ""}`}
                aria-label="Canvas drop zone"
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
              >
                <EditorCanvas
                  isPanMode={isPanMode || activeTool === "hand"}
                  isZoomTool={activeTool === "zoom"}
                  onCursorChange={setCursor}
                />
              </section>
            </main>

            <aside className="relative min-h-[36rem]">
              <div
                className="absolute inset-y-[var(--ui-gap-2)] left-0 z-10 w-2 -translate-x-1/2 cursor-col-resize"
                onPointerDown={(event) => {
                  if (panelCollapsed) {
                    return;
                  }
                  const startX = event.clientX;
                  const startWidth = panelWidth;
                  const handleMove = (moveEvent: PointerEvent) => {
                    setPanelWidth(
                      Math.min(420, Math.max(280, startWidth - (moveEvent.clientX - startX))),
                    );
                  };
                  const handleUp = () => {
                    window.removeEventListener("pointermove", handleMove);
                    window.removeEventListener("pointerup", handleUp);
                  };
                  window.addEventListener("pointermove", handleMove);
                  window.addEventListener("pointerup", handleUp);
                }}
              />

              {panelCollapsed ? (
                <div className="editor-panel flex h-full flex-col items-center gap-[var(--ui-gap-1)] border-l border-border px-[var(--ui-gap-1)] py-[var(--ui-gap-2)]">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-[11px]"
                    onClick={() => setPanelCollapsed(false)}
                  >
                    »
                  </Button>
                  {["P", "C", "H", "N", "L"].map((label) => (
                    <div
                      key={label}
                      className="flex h-8 w-8 items-center justify-center rounded-[1px] text-[11px] text-slate-400"
                    >
                      {label}
                    </div>
                  ))}
                </div>
              ) : (
                <div className="editor-panel flex h-full flex-col overflow-hidden border-l border-border">
                  <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-[var(--ui-gap-1)]">
                        {[
                          ["properties", "Properties"],
                          ["channels", "Channels"],
                          ["history", "History"],
                          ["navigator", "Navigator"],
                        ].map(([id, label]) => (
                          <button
                            key={id}
                            type="button"
                            className={[
                              "rounded-[1px] border px-2 py-1 text-[11px] transition",
                              activeAuxPanel === id
                                ? "border-white/12 bg-panel-soft text-slate-100"
                                : "border-transparent text-slate-400 hover:border-white/8 hover:bg-white/5 hover:text-slate-200",
                            ].join(" ")}
                            onClick={() => setActiveAuxPanel(id as AuxPanel)}
                          >
                            {label}
                          </button>
                        ))}
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-[11px]"
                        onClick={() => setPanelCollapsed(true)}
                      >
                        «
                      </Button>
                    </div>
                  </div>

                  <div className="grid min-h-0 flex-1 grid-rows-[minmax(15rem,18rem)_minmax(0,1fr)]">
                    <DockSection title={dockTitle(activeAuxPanel)}>
                      {activeAuxPanel === "properties" ? (
                        <div className="space-y-[var(--ui-gap-3)]">
                          <PropertyGridRow label="Document" value={documentSize} />
                          <PropertyGridRow label="Zoom" value={zoomPercent} />
                          <PropertyGridRow
                            label="Rotation"
                            value={`${render?.viewport.rotation.toFixed(0) ?? 0}°`}
                          />
                          <PropertyGridRow label="DPI" value={draft.resolution.toString()} />
                          <CompactRange
                            id="rotate-view-range"
                            label="Rotate View"
                            min={0}
                            max={360}
                            step={1}
                            value={render?.viewport.rotation ?? 0}
                            onChange={(value) => engine.setRotation(value)}
                          />
                        </div>
                      ) : null}

                      {activeAuxPanel === "history" ? (
                        <div className="flex h-full min-h-0 flex-col gap-[var(--ui-gap-2)]">
                          <div className="flex items-center justify-end">
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={historyEntries.length === 0}
                              onClick={() => engine.clearHistory()}
                            >
                              Clear
                            </Button>
                          </div>
                          <div className="min-h-0 flex-1 overflow-auto">
                            <div className="space-y-[var(--ui-gap-1)]">
                              {historyEntries.length === 0 ? (
                                <p className="text-[12px] text-slate-400">
                                  No history entries yet.
                                </p>
                              ) : (
                                historyEntries.map((entry) => (
                                  <button
                                    key={entry.id}
                                    type="button"
                                    className={[
                                      "w-full rounded-[var(--ui-radius-sm)] border px-2 py-1.5 text-left text-[12px] transition",
                                      entry.id === currentHistoryIndex
                                        ? "border-cyan-400/35 bg-cyan-400/10 text-slate-100"
                                        : entry.state === "undone"
                                          ? "border-white/8 bg-black/10 text-slate-500 hover:text-slate-300"
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
                        </div>
                      ) : null}

                      {activeAuxPanel === "navigator" ? (
                        <div className="space-y-[var(--ui-gap-3)]">
                          <div className="border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0.01))] p-[var(--ui-gap-2)]">
                            <div className="aspect-[4/3] border border-white/8 bg-[linear-gradient(135deg,rgba(56,189,248,0.18),rgba(15,23,42,0.82))]" />
                          </div>
                          <CompactRange
                            id="navigator-zoom-range"
                            label="Zoom"
                            min={5}
                            max={3200}
                            step={5}
                            value={Math.round((render?.viewport.zoom ?? 1) * 100)}
                            onChange={(value) => engine.setZoom(value / 100)}
                          />
                        </div>
                      ) : null}

                      {activeAuxPanel === "channels" ? <ChannelsPanel /> : null}
                    </DockSection>

                    <DockSection title="Layers" className="border-t border-border">
                      <LayersPanel
                        engine={engine}
                        layers={render?.uiMeta.layers ?? []}
                        activeLayerId={render?.uiMeta.activeLayerId ?? null}
                        maskEditLayerId={render?.uiMeta.maskEditLayerId ?? null}
                        documentWidth={render?.uiMeta.documentWidth ?? draft.width}
                        documentHeight={render?.uiMeta.documentHeight ?? draft.height}
                        thumbnails={layerThumbnails}
                      />
                    </DockSection>
                  </div>
                </div>
              )}
            </aside>
          </section>

          <footer className="editor-footerbar flex h-[28px] items-center justify-between gap-3 border-t border-white/8 px-2 text-[11px] text-slate-500">
            <div className="flex items-center gap-2 overflow-hidden">
              <span className="text-slate-200">{zoomPercent}</span>
              <Separator orientation="vertical" className="h-3 bg-white/8" />
              <span>{documentSize}</span>
              <Separator orientation="vertical" className="h-3 bg-white/8" />
              <span>Cursor {cursorText}</span>
            </div>
            <div className="flex items-center gap-2">
              <span>{statusText}</span>
              <Separator orientation="vertical" className="h-3 bg-white/8" />
              <span>
                {engine.status === "ready" ? `Engine #${engine.handle?.handle}` : engine.status}
              </span>
            </div>
          </footer>
        </div>
      </div>

      <Dialog
        open={newDocumentOpen}
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
                        unitToPixels(Number(event.target.value), current.resolution, documentUnit),
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
                        unitToPixels(Number(event.target.value), current.resolution, documentUnit),
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
                    resolution: Number(event.target.value),
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
              setNewDocumentOpen(false);
            }}
          >
            Create Document
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={openRecentOpen}
        title="Open Recent"
        description="The browser build cannot reopen local files automatically yet, so recent documents are informational only for now."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <p>
            Recent document tracking needs a persistent file-access layer. That is not wired into
            the web shell yet.
          </p>
          <p className="text-slate-400">
            Use Open to pick an .agp archive or legacy JSON project from disk.
          </p>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button variant="ghost" size="sm" onClick={() => setOpenRecentOpen(false)}>
            Close
          </Button>
          <Button
            size="sm"
            onClick={() => {
              setOpenRecentOpen(false);
              openProjectPicker();
            }}
          >
            Open...
          </Button>
        </div>
      </Dialog>

      <Dialog
        open={exportDialogOpen}
        title="Export As"
        description="Project archive export is available now. Flattened image exports still need dedicated engine support."
        className="max-w-lg"
      >
        <div className="space-y-3 text-[13px] text-slate-300">
          <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/20 p-3">
            <div className="text-[12px] font-medium text-slate-100">Project Archive (.agp)</div>
            <div className="mt-1 text-[12px] text-slate-400">
              Saves the current document state, layer tree, and history as {activeDocumentName}.agp.
            </div>
          </div>
        </div>

        <div className="mt-4 flex justify-end gap-[var(--ui-gap-2)] border-t border-border pt-3">
          <Button variant="ghost" size="sm" onClick={() => setExportDialogOpen(false)}>
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => {
              saveProject();
              setExportDialogOpen(false);
            }}
          >
            Export Archive
          </Button>
        </div>
      </Dialog>
    </div>
  );
}

const fieldClassName =
  "h-[var(--ui-h-md)] w-full rounded-[var(--ui-radius-sm)] border border-white/10 bg-black/20 px-2.5 text-[13px] text-slate-100 outline-none transition focus:border-cyan-400/40";

function ChromeLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 items-center gap-1 text-[11px]">
      <span className="uppercase tracking-[0.18em] text-slate-500">{label}</span>
      <span className="truncate text-slate-200">{children}</span>
    </div>
  );
}

function MetricChip({ value }: { value: string }) {
  return (
    <span className="rounded-[1px] border border-white/8 bg-panel-soft px-1.5 py-1">{value}</span>
  );
}

function MenuPreviewPanel({
  menu,
  isItemDisabled,
  onAction,
}: {
  menu: MenuPreviewMenu;
  isItemDisabled(item: MenuPreviewItem): boolean;
  onAction(actionId: FileMenuActionId): void;
}) {
  const items = menu.sections.flatMap((section) => section.items);

  return (
    <div
      className={[
        "absolute top-[calc(100%+4px)] z-40 w-[18.5rem] max-w-[calc(100vw-1rem)] overflow-hidden border border-white/10 bg-[#171b21] shadow-[0_14px_36px_rgba(0,0,0,0.42)]",
        menu.align === "right" ? "right-0" : "left-0",
      ].join(" ")}
    >
      <div className="border-b border-white/8 px-2.5 py-2 text-[11px] text-slate-400">
        {menu.caption}
      </div>

      <div className="py-1">
        {items.map((item) => {
          const disabled = isItemDisabled(item);
          return (
            <MenuPreviewAction
              key={`${menu.label}-${item.label}`}
              item={item}
              disabled={disabled}
              onClick={
                item.actionId ? () => onAction(item.actionId as FileMenuActionId) : undefined
              }
            />
          );
        })}
      </div>
    </div>
  );
}

function MenuPreviewAction({
  item,
  disabled,
  onClick,
}: {
  item: MenuPreviewItem;
  disabled: boolean;
  onClick?: () => void;
}) {
  const ItemIcon = iconForMenuItem(item.label);

  return (
    <button
      type="button"
      className={[
        "flex w-full items-center justify-between px-2.5 py-1.5 text-left text-[12px] transition",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:bg-white/6 focus:bg-white/6 focus:outline-none",
      ].join(" ")}
      disabled={disabled}
      aria-disabled={disabled}
      onClick={onClick}
    >
      <span className="flex min-w-0 items-center gap-2">
        <ItemIcon
          className={[
            "h-3.5 w-3.5 shrink-0",
            disabled || item.tone === "muted"
              ? "text-slate-600"
              : item.tone === "accent"
                ? "text-cyan-300"
                : "text-slate-400",
          ].join(" ")}
        />
        <span
          className={
            disabled || item.tone === "muted"
              ? "truncate text-slate-500"
              : "truncate text-slate-100"
          }
        >
          {item.label}
        </span>
      </span>
      {item.shortcut ? (
        <span className="ml-4 shrink-0 text-[11px] text-slate-500">{item.shortcut}</span>
      ) : null}
    </button>
  );
}

function iconForMenuItem(label: string) {
  const lower = label.toLowerCase();

  if (lower.includes("new")) {
    return NewDocumentIcon;
  }
  if (lower.includes("open")) {
    return OpenFolderIcon;
  }
  if (lower.includes("save") || lower.includes("export") || lower.includes("assets")) {
    return SaveIcon;
  }
  if (lower.includes("undo")) {
    return UndoIcon;
  }
  if (lower.includes("redo")) {
    return RedoIcon;
  }
  if (lower.includes("cut")) {
    return ScissorsIcon;
  }
  if (lower.includes("copy")) {
    return CopyIcon;
  }
  if (lower.includes("paste")) {
    return ClipboardIcon;
  }
  if (lower.includes("layer") || lower.includes("rasterize") || lower.includes("merge")) {
    return LayersIcon;
  }
  if (lower.includes("select") || lower.includes("feather") || lower.includes("inverse")) {
    return SelectionIcon;
  }
  if (
    lower.includes("levels") ||
    lower.includes("curves") ||
    lower.includes("hue") ||
    lower.includes("blur") ||
    lower.includes("noise") ||
    lower.includes("stylize") ||
    lower.includes("filter")
  ) {
    return SlidersIcon;
  }
  if (
    lower.includes("zoom") ||
    lower.includes("rulers") ||
    lower.includes("grid") ||
    lower.includes("guides")
  ) {
    return ZoomToolIcon;
  }
  if (
    lower.includes("workspace") ||
    lower.includes("navigator") ||
    lower.includes("history") ||
    lower.includes("panels")
  ) {
    return PanelsIcon;
  }
  return InfoIcon;
}

function DockSection({
  title,
  className,
  children,
}: {
  title: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section className={className}>
      <div className="border-b border-border px-[var(--ui-gap-2)] py-[var(--ui-gap-2)]">
        <h2 className="text-[12px] font-medium text-slate-100">{title}</h2>
      </div>
      <div className="h-[calc(100%-33px)] min-h-0 p-[var(--ui-gap-2)]">{children}</div>
    </section>
  );
}

function PropertyGridRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/14 px-2 py-1.5 text-[12px]">
      <span className="text-slate-400">{label}</span>
      <span className="text-slate-100">{value}</span>
    </div>
  );
}

function CompactRange({
  id,
  label,
  min,
  max,
  step,
  value,
  onChange,
}: {
  id: string;
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{Math.round(value)}</span>
      </div>
      <input
        id={id}
        className="h-2 w-full accent-cyan-400"
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  );
}

function dockTitle(panel: AuxPanel) {
  switch (panel) {
    case "history":
      return "History";
    case "navigator":
      return "Navigator";
    case "channels":
      return "Channels";
    default:
      return "Properties";
  }
}

const AUTOSAVE_KEY = "agogo:autosave";
const AUTOSAVE_EVERY_N_VERSIONS = 10;

// Channel descriptor: short label, long name, indicator colour class.
const CHANNELS = [
  { id: "rgb", label: "RGB", name: "Composite", color: "bg-slate-400", shortcut: "~" },
  { id: "r", label: "R", name: "Red", color: "bg-rose-400", shortcut: "1" },
  { id: "g", label: "G", name: "Green", color: "bg-emerald-400", shortcut: "2" },
  { id: "b", label: "B", name: "Blue", color: "bg-blue-400", shortcut: "3" },
  { id: "a", label: "A", name: "Alpha", color: "bg-slate-300", shortcut: "4" },
] as const;

function ChannelsPanel() {
  // Channel visibility is cosmetic for now; actual channel isolation is Phase 3+.
  const [visible, setVisible] = useState<Record<string, boolean>>({
    rgb: true,
    r: true,
    g: true,
    b: true,
    a: true,
  });

  return (
    <div className="space-y-[var(--ui-gap-1)]">
      {CHANNELS.map((ch) => (
        <div
          key={ch.id}
          className={[
            "flex items-center gap-2 rounded-[var(--ui-radius-sm)] border px-2 py-1.5 transition",
            visible[ch.id]
              ? "border-white/8 bg-white/[0.02]"
              : "border-white/4 bg-transparent opacity-50",
          ].join(" ")}
        >
          <button
            type="button"
            title={visible[ch.id] ? "Hide channel" : "Show channel"}
            className={[
              "flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] text-[10px] transition",
              visible[ch.id] ? "bg-emerald-400/12 text-emerald-100" : "bg-black/20 text-slate-500",
            ].join(" ")}
            onClick={() => setVisible((current) => ({ ...current, [ch.id]: !current[ch.id] }))}
          >
            {visible[ch.id] ? "O" : "-"}
          </button>
          <span className={`h-2.5 w-2.5 rounded-full ${ch.color}`} />
          <span className="flex-1 text-[12px] font-medium text-slate-100">{ch.name}</span>
          <span className="text-[11px] text-slate-500">{ch.shortcut}</span>
        </div>
      ))}
      <p className="px-1 pt-1 text-[11px] text-slate-600">Channel isolation active in Phase 3+.</p>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: label wraps its control via children (implicit label pattern)
    <label className="flex flex-col gap-1.5">
      <span className="text-[11px] uppercase tracking-[0.18em] text-slate-500">{label}</span>
      {children}
    </label>
  );
}
