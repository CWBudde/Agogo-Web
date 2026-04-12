import { CommandID } from "@agogo/proto";
import { type KeyboardEvent, type ReactNode, useRef, useState } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { EngineContextValue } from "@/wasm/types";

type PathsPanelProps = {
  engine: EngineContextValue;
  paths: Array<{ name: string; active: boolean }>;
};

export function PathsPanel({ engine, paths }: PathsPanelProps) {
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editingName, setEditingName] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const activeIndex = paths.findIndex((p) => p.active);

  function handleClick(index: number) {
    // No dedicated SetActivePath command exists, so we dispatch RenamePath
    // with the same name. The engine handler treats this as "activate path".
    engine.dispatchCommand(CommandID.RenamePath, {
      pathIndex: index,
      name: paths[index].name,
    });
  }

  function handleDoubleClick(index: number) {
    setEditingIndex(index);
    setEditingName(paths[index].name);
    requestAnimationFrame(() => inputRef.current?.select());
  }

  function commitRename() {
    if (editingIndex !== null && editingName.trim()) {
      engine.dispatchCommand(CommandID.RenamePath, {
        pathIndex: editingIndex,
        name: editingName.trim(),
      });
    }
    setEditingIndex(null);
  }

  function handleRenameKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      commitRename();
    } else if (e.key === "Escape") {
      setEditingIndex(null);
    }
  }

  function handleCreate() {
    engine.dispatchCommand(CommandID.CreatePath, { name: "" });
  }

  function handleDuplicate() {
    if (activeIndex < 0) return;
    engine.dispatchCommand(CommandID.DuplicatePath, {
      pathIndex: activeIndex,
    });
  }

  function handleDelete() {
    if (activeIndex < 0) return;
    engine.dispatchCommand(CommandID.DeletePath, {
      pathIndex: activeIndex,
    });
  }

  function handleMakeSelection() {
    if (activeIndex < 0) return;
    engine.dispatchCommand(CommandID.MakeSelectionFromPath, {
      pathIndex: activeIndex,
    });
  }

  function handleStroke() {
    if (activeIndex < 0) return;
    engine.dispatchCommand(CommandID.StrokePath, {
      pathIndex: activeIndex,
    });
  }

  function handleFill() {
    if (activeIndex < 0) return;
    engine.dispatchCommand(CommandID.FillPath, {
      pathIndex: activeIndex,
    });
  }

  function handleRasterizeLayer() {
    engine.dispatchCommand(CommandID.RasterizeLayer, {});
  }

  const hasActive = activeIndex >= 0;

  return (
    <div className="flex h-full flex-col">
      <ScrollArea className="flex-1">
        <div className="space-y-px p-1">
          {paths.length === 0 ? (
            <div className="px-2 py-4 text-center text-[11px] text-slate-500">
              No paths
            </div>
          ) : (
            paths.map((path, index) => (
              <button
                type="button"
                key={path.name}
                className={[
                  "flex h-7 w-full cursor-pointer items-center rounded-sm px-2 text-left text-[11px]",
                  path.active
                    ? "bg-blue-600/30 text-slate-100"
                    : "text-slate-300 hover:bg-white/5",
                ].join(" ")}
                onClick={() => handleClick(index)}
                onDoubleClick={() => handleDoubleClick(index)}
              >
                {editingIndex === index ? (
                  <input
                    ref={inputRef}
                    type="text"
                    className="w-full rounded-sm border border-white/20 bg-zinc-800 px-1 py-0.5 text-[11px] text-slate-100 outline-none"
                    value={editingName}
                    onChange={(e) => setEditingName(e.target.value)}
                    onBlur={commitRename}
                    onKeyDown={handleRenameKeyDown}
                    onClick={(e) => e.stopPropagation()}
                  />
                ) : (
                  <span className="truncate">{path.name}</span>
                )}
              </button>
            ))
          )}
        </div>
      </ScrollArea>

      {/* Footer action buttons */}
      <div className="flex items-center gap-0.5 border-t border-white/8 px-1 py-0.5">
        <FooterButton title="Make Selection" disabled={!hasActive} onClick={handleMakeSelection}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="2 2" role="img" aria-label="Make Selection">
            <circle cx="8" cy="8" r="6" />
          </svg>
        </FooterButton>
        <FooterButton title="Stroke Path" disabled={!hasActive} onClick={handleStroke}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="Stroke Path">
            <path d="M2 14 L12 4 L14 2 L12 4 Z" />
            <path d="M10 6 L12 4" />
          </svg>
        </FooterButton>
        <FooterButton title="Fill Path" disabled={!hasActive} onClick={handleFill}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="Fill Path">
            <path d="M3 10 L8 5 L11 8 L6 13 Z" />
            <path d="M13 11 Q15 13 13 14 Q11 13 13 11" />
          </svg>
        </FooterButton>
        <FooterButton title="Rasterize Layer" disabled={!hasActive} onClick={handleRasterizeLayer}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="Rasterize Layer">
            <rect x="3" y="3" width="10" height="10" rx="1" />
            <path d="M5 5 L11 11 M11 5 L5 11" />
          </svg>
        </FooterButton>

        <div className="flex-1" />

        <FooterButton title="New Path" onClick={handleCreate}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="New Path">
            <path d="M8 3 V13 M3 8 H13" />
          </svg>
        </FooterButton>
        <FooterButton title="Duplicate Path" disabled={!hasActive} onClick={handleDuplicate}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="Duplicate Path">
            <rect x="5" y="5" width="8" height="8" rx="1" />
            <path d="M3 11 V3 H11" />
          </svg>
        </FooterButton>
        <FooterButton title="Delete Path" disabled={!hasActive} onClick={handleDelete}>
          <svg viewBox="0 0 16 16" className="size-3.5" fill="none" stroke="currentColor" strokeWidth="1.5" role="img" aria-label="Delete Path">
            <path d="M3 4 H13 M5 4 V3 H11 V4 M6 6 V12 M10 6 V12 M4 4 L5 14 H11 L12 4" />
          </svg>
        </FooterButton>
      </div>
    </div>
  );
}

function FooterButton({
  children,
  title,
  disabled,
  onClick,
}: {
  children: ReactNode;
  title: string;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={title}
      disabled={disabled}
      className="flex size-6 items-center justify-center rounded-sm text-slate-400 hover:bg-white/8 hover:text-slate-200 disabled:pointer-events-none disabled:opacity-40"
      onClick={onClick}
    >
      {children}
    </button>
  );
}
