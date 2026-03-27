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
    <div className="flex h-full min-h-0 flex-col gap-[var(--ui-gap-2)]">
      <div className="flex items-center justify-between gap-2 text-[11px]">
        <div className="flex items-center gap-2 text-slate-300">
          <span className="font-medium text-slate-100">Active</span>
          <span className="truncate text-slate-400">{activeLayer?.name ?? "None"}</span>
        </div>
        <div className="rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/12 px-1.5 py-1 text-slate-400">
          {layerCount}
        </div>
      </div>

      <div className="grid grid-cols-5 gap-[var(--ui-gap-1)]">
        <ToolbarAction label="+L" title="New Layer" onClick={addPixelLayer} />
        <ToolbarAction label="+G" title="New Group" onClick={addGroupLayer} />
        <ToolbarAction
          label="Mask"
          title="Add Mask"
          onClick={() => addMask("reveal-all")}
          disabled={!activeLayer}
        />
        <ToolbarAction
          label="Merge"
          title="Merge Down"
          onClick={() => {
            if (!activeLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.MergeDown, {
              layerId: activeLayer.id,
            });
          }}
          disabled={!activeLayer}
        />
        <ToolbarAction
          label="Del"
          title="Delete Layer"
          onClick={() => {
            if (!activeLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.DeleteLayer, {
              layerId: activeLayer.id,
            });
          }}
          disabled={!activeLayer}
        />
      </div>

      <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_auto] gap-[var(--ui-gap-2)]">
        <ScrollArea className="min-h-0 rounded-[var(--ui-radius-md)] border border-white/8 bg-black/12">
          {layers.length === 0 ? (
            <div className="px-3 py-4 text-[12px] text-slate-400">
              No layers yet. Create a layer or group to start the stack.
            </div>
          ) : (
            <div className="p-[var(--ui-gap-2)]">
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

        <div className="rounded-[var(--ui-radius-md)] border border-white/8 bg-black/12 p-[var(--ui-gap-2)]">
          <div className="grid gap-[var(--ui-gap-2)]">
            <div className="grid grid-cols-[1fr_auto] items-center gap-2">
              <label className="text-[11px] uppercase tracking-[0.18em] text-slate-500">
                Blend
                <select
                  className="mt-1 h-[var(--ui-h-sm)] w-full rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-2 text-[12px] text-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
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
              <div className="text-right text-[11px] text-slate-400">
                {activeLayer ? describeLayer(activeLayer) : "No selection"}
              </div>
            </div>

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

            <div className="grid grid-cols-2 gap-[var(--ui-gap-1)]">
              <ActionButton
                label={
                  activeLayer?.hasMask
                    ? activeLayer.maskEnabled
                      ? "Disable Mask"
                      : "Enable Mask"
                    : "Reveal Mask"
                }
                disabled={!activeLayer}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  if (!activeLayer.hasMask) {
                    addMask("reveal-all");
                    return;
                  }
                  engine.dispatchCommand(CommandID.SetLayerMaskEnabled, {
                    layerId: activeLayer.id,
                    enabled: !activeLayer.maskEnabled,
                  });
                }}
              />
              <ActionButton
                label={activeLayer?.clipToBelow ? "Release Clip" : "Clip To Below"}
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
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function ToolbarAction({
  label,
  title,
  onClick,
  disabled,
}: {
  label: string;
  title: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <Button
      variant="secondary"
      size="sm"
      className="min-w-0 px-0 text-[11px]"
      title={title}
      disabled={disabled}
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

function ActionButton({
  label,
  onClick,
  disabled,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <Button
      variant="secondary"
      size="sm"
      className="justify-center text-[11px]"
      disabled={disabled}
      onClick={onClick}
    >
      {label}
    </Button>
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
    <div className="space-y-[var(--ui-gap-1)]">
      <div
        className="space-y-[var(--ui-gap-1)]"
        style={{ marginLeft: `${depth * 12 + (layer.clipToBelow ? 10 : 0)}px` }}
      >
        <div
          className={[
            "h-[2px] rounded-full transition",
            dropState === "before" ? "bg-cyan-300/90" : "bg-transparent",
          ].join(" ")}
        />

        <div
          className={[
            "rounded-[var(--ui-radius-md)] border transition",
            isDragging ? "border-white/5 bg-white/[0.02] opacity-50" : "",
            isActive
              ? "border-cyan-400/35 bg-cyan-400/10"
              : "border-white/8 bg-white/[0.02] hover:border-white/12 hover:bg-white/[0.04]",
            dropState === "inside" ? "border-cyan-300/60 bg-cyan-300/10" : "",
          ].join(" ")}
          role="treeitem"
          tabIndex={0}
          aria-selected={isActive}
          draggable={!isEditing}
          onClick={() => onSelect(layer.id)}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              onSelect(layer.id);
            }
          }}
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
          <div className="grid grid-cols-[auto_auto_1fr_auto] items-center gap-[var(--ui-gap-2)] px-2 py-1.5">
            <div className="flex items-center gap-[var(--ui-gap-1)]">
              {isGroup ? (
                <button
                  type="button"
                  className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] text-[10px] text-slate-400 transition hover:bg-white/6 hover:text-slate-100"
                  onClick={(event) => {
                    event.stopPropagation();
                    onToggleGroup(layer.id);
                  }}
                >
                  {isCollapsed ? ">" : "v"}
                </button>
              ) : (
                <span className="block w-5" />
              )}
              <button
                type="button"
                className={[
                  "flex h-5 min-w-5 items-center justify-center rounded-[var(--ui-radius-sm)] px-1 text-[10px] transition",
                  layer.visible
                    ? "bg-emerald-400/12 text-emerald-100"
                    : "bg-black/20 text-slate-500",
                ].join(" ")}
                onClick={(event) => {
                  event.stopPropagation();
                  onToggleVisibility(layer.id, !layer.visible);
                }}
                title={layer.visible ? "Hide layer" : "Show layer"}
              >
                {layer.visible ? "O" : "-"}
              </button>
            </div>

            <div className="flex h-8 w-8 items-center justify-center rounded-[var(--ui-radius-sm)] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] text-[9px] font-semibold uppercase tracking-[0.16em] text-slate-200">
              {layer.layerType === "group" ? "grp" : layer.layerType.slice(0, 2)}
            </div>

            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-[var(--ui-gap-1)]">
                {isEditing ? (
                  <input
                    className="h-6 w-full rounded-[var(--ui-radius-sm)] border border-cyan-400/30 bg-black/25 px-1.5 text-[12px] text-slate-100 outline-none"
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
                  <button
                    type="button"
                    className="min-w-0 text-left"
                    onDoubleClick={(event) => {
                      event.stopPropagation();
                      onStartRename(layer);
                    }}
                  >
                    <span className="block truncate text-[12px] font-medium text-slate-100">
                      {layer.name}
                    </span>
                  </button>
                )}
                {layer.clippingBase ? <MiniBadge label="base" tone="amber" /> : null}
                {layer.clipToBelow ? <MiniBadge label="clip" tone="sky" /> : null}
                {layer.hasMask ? <MiniBadge label="mask" tone="fuchsia" /> : null}
              </div>
              <div className="mt-[2px] flex flex-wrap items-center gap-1 text-[10px] text-slate-400">
                <span>{formatBlendMode(layer.blendMode)}</span>
                <span>{Math.round(layer.opacity * 100)}%</span>
                {isGroup ? <span>{layer.isolated ? "Isolated" : "Pass-through"}</span> : null}
              </div>
            </div>

            <div className="flex items-center gap-[var(--ui-gap-1)]">
              <button
                type="button"
                className="flex h-5 min-w-6 items-center justify-center rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/18 px-1 text-[10px] text-slate-300 transition hover:bg-black/30"
                onClick={(event) => {
                  event.stopPropagation();
                  onCycleLock(layer.id, layer.lockMode);
                }}
                title="Cycle lock mode"
              >
                {shortLockLabel(layer.lockMode)}
              </button>
              <button
                type="button"
                className="flex h-5 min-w-6 cursor-grab items-center justify-center rounded-[var(--ui-radius-sm)] border border-white/8 bg-black/18 px-1 text-[10px] text-slate-300 transition hover:bg-black/30 active:cursor-grabbing"
                onClick={(event) => event.stopPropagation()}
                onDoubleClick={(event) => {
                  event.stopPropagation();
                  onDuplicate(layer.id);
                }}
                title="Drag to reorder, double-click to duplicate"
              >
                ::
              </button>
            </div>
          </div>
        </div>

        <div
          className={[
            "h-[2px] rounded-full transition",
            dropState === "after" ? "bg-cyan-300/90" : "bg-transparent",
          ].join(" ")}
        />
      </div>

      {isGroup && !isCollapsed && children.length > 0 ? (
        <div className="space-y-[var(--ui-gap-1)]">
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

function MiniBadge({ label, tone }: { label: string; tone: "amber" | "sky" | "fuchsia" }) {
  const toneClass =
    tone === "amber"
      ? "border-amber-400/25 bg-amber-400/10 text-amber-100"
      : tone === "sky"
        ? "border-sky-400/25 bg-sky-400/10 text-sky-100"
        : "border-fuchsia-400/25 bg-fuchsia-400/10 text-fuchsia-100";

  return (
    <span
      className={[
        "rounded-[var(--ui-radius-sm)] border px-1 py-[1px] text-[9px] uppercase tracking-[0.16em]",
        toneClass,
      ].join(" ")}
    >
      {label}
    </span>
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
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-slate-500">
        <span>{label}</span>
        <span className="text-slate-300">{value}</span>
      </div>
      <div className="grid grid-cols-[1fr_44px] items-center gap-[var(--ui-gap-2)]">
        <input
          className="h-2 w-full accent-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
          type="range"
          min="0"
          max="100"
          value={value}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value))}
        />
        <input
          className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-white/8 bg-panel-soft px-1.5 text-right text-[12px] text-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
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
