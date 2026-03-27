import {
  type AddLayerMaskMode,
  CommandID,
  type LayerBlendMode,
  type LayerLockMode,
  type LayerNodeMeta,
} from "@agogo/proto";
import { type DragEvent, type KeyboardEvent, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { EngineContextValue } from "@/wasm/types";

const blendModeOptions: LayerBlendMode[] = [
  "normal",
  "multiply",
  "screen",
  "overlay",
  "soft-light",
  "hard-light",
  "difference",
  "exclusion",
  "color",
  "luminosity",
];

const lockModeCycle: LayerLockMode[] = ["none", "pixels", "position", "all"];

type DropPosition = "before" | "after" | "inside";

type DropTarget = {
  layerId: string;
  position: DropPosition;
} | null;

type LayersPanelProps = {
  engine: EngineContextValue;
  layers: LayerNodeMeta[];
  activeLayerId: string | null;
  documentWidth: number;
  documentHeight: number;
};

export function LayersPanel({
  engine,
  layers,
  activeLayerId,
  documentWidth,
  documentHeight,
}: LayersPanelProps) {
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [editingLayerId, setEditingLayerId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [draggedLayerId, setDraggedLayerId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget>(null);

  const activeLayer = useMemo(
    () => findLayerById(layers, activeLayerId ?? "") ?? firstLayer(layers),
    [activeLayerId, layers],
  );
  const layerCount = useMemo(() => countLayers(layers), [layers]);

  const selectLayer = (layerId: string) => {
    engine.dispatchCommand(CommandID.SetActiveLayer, { layerId });
  };

  const addPixelLayer = () => {
    engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "pixel",
      name: `Layer ${layerCount + 1}`,
      bounds: { x: 0, y: 0, w: documentWidth, h: documentHeight },
    });
  };

  const addGroupLayer = () => {
    engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "group",
      name: `Group ${layerCount + 1}`,
      isolated: true,
    });
  };

  const addMask = (mode: AddLayerMaskMode) => {
    if (!activeLayer) {
      return;
    }
    engine.dispatchCommand(CommandID.AddLayerMask, {
      layerId: activeLayer.id,
      mode,
    });
  };

  const startRename = (layer: LayerNodeMeta) => {
    selectLayer(layer.id);
    setEditingLayerId(layer.id);
    setEditingName(layer.name);
  };

  const cancelRename = () => {
    setEditingLayerId(null);
    setEditingName("");
  };

  const commitRename = () => {
    if (!editingLayerId) {
      return;
    }
    engine.dispatchCommand(CommandID.SetLayerName, {
      layerId: editingLayerId,
      name: editingName.trim(),
    });
    setEditingLayerId(null);
    setEditingName("");
  };

  const moveLayer = (layerId: string, targetLayerId: string, position: DropPosition) => {
    if (layerId === targetLayerId) {
      return;
    }

    const targetLayer = findLayerById(layers, targetLayerId);
    if (!targetLayer) {
      return;
    }

    if (position === "inside") {
      if (targetLayer.layerType !== "group" || isDescendantLayer(layers, layerId, targetLayer.id)) {
        return;
      }
      engine.dispatchCommand(CommandID.MoveLayer, {
        layerId,
        parentLayerId: targetLayer.id,
        index: targetLayer.children?.length ?? 0,
      });
      return;
    }

    const siblings = getChildrenForParent(layers, targetLayer.parentId);
    const targetIndex = siblings.findIndex((candidate) => candidate.id === targetLayer.id);
    if (targetIndex < 0) {
      return;
    }

    engine.dispatchCommand(CommandID.MoveLayer, {
      layerId,
      parentLayerId: targetLayer.parentId || undefined,
      index: position === "before" ? targetIndex + 1 : targetIndex,
    });
  };

  const handleDragOver = (event: DragEvent<HTMLDivElement>, layer: LayerNodeMeta) => {
    if (!draggedLayerId || draggedLayerId === layer.id) {
      return;
    }

    event.preventDefault();

    const rect = event.currentTarget.getBoundingClientRect();
    const offsetY = event.clientY - rect.top;
    let position: DropPosition = offsetY < rect.height / 2 ? "before" : "after";

    if (
      layer.layerType === "group" &&
      offsetY > rect.height * 0.28 &&
      offsetY < rect.height * 0.72 &&
      !isDescendantLayer(layers, draggedLayerId, layer.id)
    ) {
      position = "inside";
    }

    setDropTarget({ layerId: layer.id, position });
  };

  const handleDrop = (layer: LayerNodeMeta) => {
    if (!draggedLayerId || !dropTarget || dropTarget.layerId !== layer.id) {
      return;
    }
    moveLayer(draggedLayerId, layer.id, dropTarget.position);
    setDraggedLayerId(null);
    setDropTarget(null);
  };

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-slate-100">Layers</h2>
          <p className="mt-1 text-sm leading-6 text-slate-400">
            Inline rename and drag-reorder are now layered on top of the engine tree model.
          </p>
        </div>
        <div className="rounded-full border border-white/10 bg-black/20 px-3 py-1 text-[11px] uppercase tracking-[0.24em] text-slate-400">
          {layerCount} layers
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" className="h-9 px-3 text-xs" onClick={addPixelLayer}>
          New Layer
        </Button>
        <Button variant="secondary" className="h-9 px-3 text-xs" onClick={addGroupLayer}>
          New Group
        </Button>
        <Button
          variant="secondary"
          className="h-9 px-3 text-xs"
          disabled={!activeLayer}
          onClick={() => addMask("reveal-all")}
        >
          Add Mask
        </Button>
        <Button
          variant="secondary"
          className="h-9 px-3 text-xs"
          disabled={!activeLayer}
          onClick={() => {
            if (!activeLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.MergeDown, {
              layerId: activeLayer.id,
            });
          }}
        >
          Merge Down
        </Button>
        <Button
          variant="secondary"
          className="h-9 px-3 text-xs"
          disabled={!activeLayer}
          onClick={() => {
            if (!activeLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.DeleteLayer, {
              layerId: activeLayer.id,
            });
          }}
        >
          Delete
        </Button>
      </div>

      <div className="grid min-h-0 flex-1 gap-3">
        <ScrollArea
          className="min-h-0 rounded-2xl border border-white/10 bg-black/15"
          viewportClassName="p-3"
        >
          {layers.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-white/10 bg-white/[0.02] px-4 py-6 text-sm text-slate-400">
              No layers yet. Create a pixel layer or a group to start building the document tree.
            </div>
          ) : (
            <div className="space-y-2">
              {[...layers].reverse().map((layer) => (
                <LayerTreeRow
                  key={layer.id}
                  layer={layer}
                  depth={0}
                  activeLayerId={activeLayerId}
                  collapsedGroups={collapsedGroups}
                  draggedLayerId={draggedLayerId}
                  dropTarget={dropTarget}
                  editingLayerId={editingLayerId}
                  editingName={editingName}
                  onEditingNameChange={setEditingName}
                  onStartRename={startRename}
                  onCommitRename={commitRename}
                  onCancelRename={cancelRename}
                  onToggleGroup={(layerId) =>
                    setCollapsedGroups((current) => ({
                      ...current,
                      [layerId]: !current[layerId],
                    }))
                  }
                  onSelect={selectLayer}
                  onToggleVisibility={(layerId, visible) =>
                    engine.dispatchCommand(CommandID.SetLayerVisibility, {
                      layerId,
                      visible,
                    })
                  }
                  onCycleLock={(layerId, lockMode) =>
                    engine.dispatchCommand(CommandID.SetLayerLock, {
                      layerId,
                      lockMode: nextLockMode(lockMode),
                    })
                  }
                  onDuplicate={(layerId) =>
                    engine.dispatchCommand(CommandID.DuplicateLayer, {
                      layerId,
                    })
                  }
                  onDragStart={(layerId) => {
                    setDraggedLayerId(layerId);
                    selectLayer(layerId);
                  }}
                  onDragEnd={() => {
                    setDraggedLayerId(null);
                    setDropTarget(null);
                  }}
                  onDragOver={handleDragOver}
                  onDropLayer={handleDrop}
                />
              ))}
            </div>
          )}
        </ScrollArea>

        <div className="space-y-3 rounded-2xl border border-white/10 bg-black/15 p-3">
          <div>
            <p className="text-xs uppercase tracking-[0.24em] text-slate-500">Selection</p>
            <p className="mt-2 text-sm font-medium text-slate-100">
              {activeLayer?.name ?? "No active layer"}
            </p>
            <p className="mt-1 text-xs text-slate-400">
              {activeLayer ? describeLayer(activeLayer) : "Pick a row in the tree to edit it."}
            </p>
          </div>

          <label className="flex flex-col gap-2 text-xs uppercase tracking-[0.24em] text-slate-500">
            Blend Mode
            <select
              className="h-10 rounded-xl border border-white/10 bg-black/20 px-3 text-sm text-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={!activeLayer}
              value={activeLayer?.blendMode ?? "normal"}
              onChange={(event) => {
                if (!activeLayer) {
                  return;
                }
                engine.dispatchCommand(CommandID.SetLayerBlendMode, {
                  layerId: activeLayer.id,
                  blendMode: event.target.value,
                });
              }}
            >
              {blendModeOptions.map((mode) => (
                <option key={mode} value={mode}>
                  {formatBlendMode(mode)}
                </option>
              ))}
            </select>
          </label>

          <RangeField
            label="Opacity"
            disabled={!activeLayer}
            value={Math.round((activeLayer?.opacity ?? 1) * 100)}
            onChange={(value) => {
              if (!activeLayer) {
                return;
              }
              engine.dispatchCommand(CommandID.SetLayerOpacity, {
                layerId: activeLayer.id,
                opacity: value / 100,
              });
            }}
          />

          <RangeField
            label="Fill"
            disabled={!activeLayer}
            value={Math.round((activeLayer?.fillOpacity ?? 1) * 100)}
            onChange={(value) => {
              if (!activeLayer) {
                return;
              }
              engine.dispatchCommand(CommandID.SetLayerOpacity, {
                layerId: activeLayer.id,
                fillOpacity: value / 100,
              });
            }}
          />

          <div className="space-y-2 rounded-xl border border-white/8 bg-white/[0.02] p-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs uppercase tracking-[0.24em] text-slate-500">Mask</span>
              <span className="text-xs text-slate-400">
                {activeLayer?.hasMask ? (activeLayer.maskEnabled ? "Enabled" : "Disabled") : "None"}
              </span>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={Boolean(activeLayer?.hasMask) || !activeLayer}
                onClick={() => addMask("reveal-all")}
              >
                Reveal All
              </Button>
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={Boolean(activeLayer?.hasMask) || !activeLayer}
                onClick={() => addMask("hide-all")}
              >
                Hide All
              </Button>
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={!activeLayer?.hasMask}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  engine.dispatchCommand(CommandID.SetLayerMaskEnabled, {
                    layerId: activeLayer.id,
                    enabled: !activeLayer.maskEnabled,
                  });
                }}
              >
                {activeLayer?.maskEnabled ? "Disable" : "Enable"}
              </Button>
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={!activeLayer?.hasMask}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  engine.dispatchCommand(CommandID.InvertLayerMask, {
                    layerId: activeLayer.id,
                  });
                }}
              >
                Invert
              </Button>
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={!activeLayer?.hasMask}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  engine.dispatchCommand(CommandID.ApplyLayerMask, {
                    layerId: activeLayer.id,
                  });
                }}
              >
                Apply
              </Button>
              <Button
                variant="secondary"
                className="h-8 px-2 text-xs"
                disabled={!activeLayer?.hasMask}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  engine.dispatchCommand(CommandID.DeleteLayerMask, {
                    layerId: activeLayer.id,
                  });
                }}
              >
                Delete
              </Button>
            </div>
          </div>

          <div className="space-y-2 rounded-xl border border-white/8 bg-white/[0.02] p-3">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs uppercase tracking-[0.24em] text-slate-500">Clipping</span>
              <span className="text-xs text-slate-400">
                {activeLayer?.clipToBelow ? "Clipped to below" : "Independent"}
              </span>
            </div>
            <Button
              variant="secondary"
              className="h-8 w-full px-2 text-xs"
              disabled={!activeLayer}
              onClick={() => {
                if (!activeLayer) {
                  return;
                }
                engine.dispatchCommand(CommandID.SetLayerClipToBelow, {
                  layerId: activeLayer.id,
                  clipToBelow: !activeLayer.clipToBelow,
                });
              }}
            >
              {activeLayer?.clipToBelow ? "Release Clipping" : "Clip To Below"}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

type LayerTreeRowProps = {
  layer: LayerNodeMeta;
  depth: number;
  activeLayerId: string | null;
  collapsedGroups: Record<string, boolean>;
  draggedLayerId: string | null;
  dropTarget: DropTarget;
  editingLayerId: string | null;
  editingName: string;
  onEditingNameChange: (value: string) => void;
  onStartRename: (layer: LayerNodeMeta) => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  onToggleGroup: (layerId: string) => void;
  onSelect: (layerId: string) => void;
  onToggleVisibility: (layerId: string, visible: boolean) => void;
  onCycleLock: (layerId: string, lockMode: LayerLockMode) => void;
  onDuplicate: (layerId: string) => void;
  onDragStart: (layerId: string) => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent<HTMLDivElement>, layer: LayerNodeMeta) => void;
  onDropLayer: (layer: LayerNodeMeta) => void;
};

function LayerTreeRow({
  layer,
  depth,
  activeLayerId,
  collapsedGroups,
  draggedLayerId,
  dropTarget,
  editingLayerId,
  editingName,
  onEditingNameChange,
  onStartRename,
  onCommitRename,
  onCancelRename,
  onToggleGroup,
  onSelect,
  onToggleVisibility,
  onCycleLock,
  onDuplicate,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDropLayer,
}: LayerTreeRowProps) {
  const isGroup = layer.layerType === "group";
  const isCollapsed = isGroup && collapsedGroups[layer.id];
  const isActive = layer.id === activeLayerId;
  const isDragging = layer.id === draggedLayerId;
  const isEditing = layer.id === editingLayerId;
  const children = layer.children ?? [];
  const dropState = dropTarget?.layerId === layer.id ? dropTarget.position : null;

  return (
    <div className="space-y-2">
      <div
        className="space-y-1"
        style={{ marginLeft: `${depth * 16 + (layer.clipToBelow ? 12 : 0)}px` }}
      >
        <div
          className={[
            "h-1 rounded-full transition",
            dropState === "before" ? "bg-cyan-300/90" : "bg-transparent",
          ].join(" ")}
        />
        <div
          className={[
            "rounded-2xl border px-3 py-3 transition",
            isDragging ? "border-white/5 bg-white/[0.015] opacity-50" : "",
            isActive
              ? "border-cyan-400/30 bg-cyan-400/10"
              : "border-white/8 bg-white/[0.02] hover:border-white/15 hover:bg-white/[0.04]",
            dropState === "inside" ? "border-cyan-300/60 bg-cyan-300/10" : "",
          ].join(" ")}
          role="treeitem"
          tabIndex={0}
          aria-selected={isActive}
          draggable={!isEditing}
          onDragStart={(event) => {
            event.stopPropagation();
            onDragStart(layer.id);
          }}
          onDragEnd={onDragEnd}
          onDragOver={(event) => onDragOver(event, layer)}
          onDrop={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onDropLayer(layer);
          }}
        >
          <div className="flex items-start gap-3">
            <div className="flex items-center gap-2 pt-0.5">
              <button
                type="button"
                className="cursor-grab rounded-lg border border-white/10 bg-black/20 px-1.5 py-1 text-[10px] uppercase tracking-[0.16em] text-slate-400 transition hover:bg-black/30 active:cursor-grabbing"
                onClick={(event) => event.stopPropagation()}
                title="Drag to reorder"
              >
                ::
              </button>
              {isGroup ? (
                <button
                  type="button"
                  className="h-6 w-6 rounded-lg border border-white/10 bg-black/20 text-xs text-slate-300 transition hover:bg-black/30"
                  onClick={(event) => {
                    event.stopPropagation();
                    onToggleGroup(layer.id);
                  }}
                >
                  {isCollapsed ? ">" : "v"}
                </button>
              ) : null}
              <button
                type="button"
                className={[
                  "h-6 rounded-lg border px-2 text-[10px] uppercase tracking-[0.18em] transition",
                  layer.visible
                    ? "border-emerald-400/30 bg-emerald-400/10 text-emerald-100"
                    : "border-white/10 bg-black/20 text-slate-500",
                ].join(" ")}
                onClick={(event) => {
                  event.stopPropagation();
                  onToggleVisibility(layer.id, !layer.visible);
                }}
              >
                {layer.visible ? "show" : "hide"}
              </button>
            </div>

            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-white/10 bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.02))] text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-200">
              {layer.layerType === "group" ? "grp" : layer.layerType.slice(0, 2)}
            </div>

            <div className="min-w-0 flex-1">
              <button
                type="button"
                className="w-full min-w-0 text-left"
                onClick={() => onSelect(layer.id)}
                onDoubleClick={() => onStartRename(layer)}
              >
                <div className="flex items-center gap-2">
                  {isEditing ? (
                    <input
                      className="h-8 w-full rounded-lg border border-cyan-400/30 bg-black/25 px-2 text-sm text-slate-100 outline-none"
                      value={editingName}
                      onBlur={onCommitRename}
                      onChange={(event) => onEditingNameChange(event.target.value)}
                      onClick={(event) => event.stopPropagation()}
                      onKeyDown={(event: KeyboardEvent<HTMLInputElement>) => {
                        if (event.key === "Enter") {
                          event.preventDefault();
                          onCommitRename();
                        }
                        if (event.key === "Escape") {
                          event.preventDefault();
                          onCancelRename();
                        }
                      }}
                    />
                  ) : (
                    <span className="truncate text-sm font-medium text-slate-100">
                      {layer.name}
                    </span>
                  )}
                  {layer.clippingBase ? (
                    <span className="rounded-full border border-amber-400/25 bg-amber-400/10 px-2 py-0.5 text-[10px] uppercase tracking-[0.16em] text-amber-100">
                      base
                    </span>
                  ) : null}
                  {layer.clipToBelow ? (
                    <span className="rounded-full border border-sky-400/25 bg-sky-400/10 px-2 py-0.5 text-[10px] uppercase tracking-[0.16em] text-sky-100">
                      clip
                    </span>
                  ) : null}
                  {layer.hasMask ? (
                    <span className="rounded-full border border-fuchsia-400/25 bg-fuchsia-400/10 px-2 py-0.5 text-[10px] uppercase tracking-[0.16em] text-fuchsia-100">
                      mask
                    </span>
                  ) : null}
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                  <span>{describeLayer(layer)}</span>
                  <span>{Math.round(layer.opacity * 100)}%</span>
                  <span>{formatBlendMode(layer.blendMode)}</span>
                </div>
              </button>
            </div>

            <div className="flex items-center gap-2">
              <button
                type="button"
                className="rounded-lg border border-white/10 bg-black/20 px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-slate-300 transition hover:bg-black/30"
                onClick={(event) => {
                  event.stopPropagation();
                  onCycleLock(layer.id, layer.lockMode);
                }}
              >
                {shortLockLabel(layer.lockMode)}
              </button>
              <button
                type="button"
                className="rounded-lg border border-white/10 bg-black/20 px-2 py-1 text-[10px] uppercase tracking-[0.16em] text-slate-300 transition hover:bg-black/30"
                onClick={(event) => {
                  event.stopPropagation();
                  onDuplicate(layer.id);
                }}
              >
                dup
              </button>
            </div>
          </div>
        </div>
        <div
          className={[
            "h-1 rounded-full transition",
            dropState === "after" ? "bg-cyan-300/90" : "bg-transparent",
          ].join(" ")}
        />
      </div>

      {isGroup && !isCollapsed && children.length > 0 ? (
        <div className="space-y-2">
          {[...children].reverse().map((child) => (
            <LayerTreeRow
              key={child.id}
              layer={child}
              depth={depth + 1}
              activeLayerId={activeLayerId}
              collapsedGroups={collapsedGroups}
              draggedLayerId={draggedLayerId}
              dropTarget={dropTarget}
              editingLayerId={editingLayerId}
              editingName={editingName}
              onEditingNameChange={onEditingNameChange}
              onStartRename={onStartRename}
              onCommitRename={onCommitRename}
              onCancelRename={onCancelRename}
              onToggleGroup={onToggleGroup}
              onSelect={onSelect}
              onToggleVisibility={onToggleVisibility}
              onCycleLock={onCycleLock}
              onDuplicate={onDuplicate}
              onDragStart={onDragStart}
              onDragEnd={onDragEnd}
              onDragOver={onDragOver}
              onDropLayer={onDropLayer}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function RangeField({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  return (
    <label className="flex flex-col gap-2 text-xs uppercase tracking-[0.24em] text-slate-500">
      {label}
      <div className="flex items-center gap-2">
        <input
          className="h-2 flex-1 accent-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
          type="range"
          min="0"
          max="100"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <input
          className="h-9 w-16 rounded-lg border border-white/10 bg-black/20 px-2 text-right text-sm text-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
          type="number"
          min="0"
          max="100"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
      </div>
    </label>
  );
}

function findLayerById(layers: LayerNodeMeta[], targetId: string): LayerNodeMeta | null {
  for (const layer of layers) {
    if (layer.id === targetId) {
      return layer;
    }
    if (layer.children?.length) {
      const child = findLayerById(layer.children, targetId);
      if (child) {
        return child;
      }
    }
  }
  return null;
}

function firstLayer(layers: LayerNodeMeta[]): LayerNodeMeta | null {
  if (layers.length === 0) {
    return null;
  }
  const top = layers[layers.length - 1];
  if (top.children?.length) {
    return firstLayer(top.children) ?? top;
  }
  return top;
}

function getChildrenForParent(layers: LayerNodeMeta[], parentId?: string) {
  if (!parentId) {
    return layers;
  }
  return findLayerById(layers, parentId)?.children ?? [];
}

function countLayers(layers: LayerNodeMeta[]): number {
  return layers.reduce((count, layer) => count + 1 + countLayers(layer.children ?? []), 0);
}

function nextLockMode(current: LayerLockMode): LayerLockMode {
  const index = lockModeCycle.indexOf(current);
  return lockModeCycle[(index + 1 + lockModeCycle.length) % lockModeCycle.length];
}

function shortLockLabel(mode: LayerLockMode) {
  switch (mode) {
    case "pixels":
      return "px";
    case "position":
      return "pos";
    case "all":
      return "all";
    default:
      return "open";
  }
}

function formatBlendMode(mode: string) {
  return mode
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function describeLayer(layer: LayerNodeMeta) {
  if (layer.layerType === "group") {
    return layer.isolated ? "Isolated group" : "Pass-through group";
  }
  return `${layer.layerType} layer`;
}

function isDescendantLayer(layers: LayerNodeMeta[], ancestorId: string, candidateId: string) {
  const ancestor = findLayerById(layers, ancestorId);
  if (!ancestor) {
    return false;
  }
  return containsLayerId(ancestor.children ?? [], candidateId);
}

function containsLayerId(layers: LayerNodeMeta[], targetId: string): boolean {
  for (const layer of layers) {
    if (layer.id === targetId || containsLayerId(layer.children ?? [], targetId)) {
      return true;
    }
  }
  return false;
}
