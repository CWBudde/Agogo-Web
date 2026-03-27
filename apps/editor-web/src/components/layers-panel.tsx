import {
  type AddLayerMaskMode,
  CommandID,
  type LayerBlendMode,
  type LayerLockMode,
  type LayerNodeMeta,
} from "@agogo/proto";
import {
  type DragEvent,
  type KeyboardEvent,
  type MouseEvent,
  useEffect,
  useMemo,
  useState,
} from "react";
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
  const [selectedLayerIds, setSelectedLayerIds] = useState<string[]>([]);
  const [lastSelectedLayerId, setLastSelectedLayerId] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<LayerContextMenuState>(null);

  const activeLayer = useMemo(
    () => findLayerById(layers, activeLayerId ?? "") ?? firstLayer(layers),
    [activeLayerId, layers],
  );
  const layerCount = useMemo(() => countLayers(layers), [layers]);
  const displayOrder = useMemo(
    () => collectLayerOrder(layers, collapsedGroups),
    [collapsedGroups, layers],
  );
  const selectedIds =
    selectedLayerIds.length > 0 ? selectedLayerIds : activeLayer ? [activeLayer.id] : [];
  const selectedIdSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  useEffect(() => {
    if (!contextMenu) {
      return;
    }

    const closeContextMenu = () => setContextMenu(null);
    const handleEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        setContextMenu(null);
      }
    };

    window.addEventListener("pointerdown", closeContextMenu);
    window.addEventListener("keydown", handleEscape);
    return () => {
      window.removeEventListener("pointerdown", closeContextMenu);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [contextMenu]);

  const selectLayer = (
    layerId: string,
    event?: Pick<MouseEvent<HTMLElement>, "shiftKey" | "ctrlKey" | "metaKey">,
  ) => {
    const additiveSelection = Boolean(event?.ctrlKey || event?.metaKey);
    if (event?.shiftKey && lastSelectedLayerId) {
      const rangeSelection = getLayerSelectionRange(displayOrder, lastSelectedLayerId, layerId);
      setSelectedLayerIds(rangeSelection.length > 0 ? rangeSelection : [layerId]);
    } else if (additiveSelection) {
      setSelectedLayerIds((current) =>
        current.includes(layerId)
          ? current.filter((candidate) => candidate !== layerId)
          : [...current, layerId],
      );
    } else {
      setSelectedLayerIds([layerId]);
    }
    setLastSelectedLayerId(layerId);
    engine.dispatchCommand(CommandID.SetActiveLayer, { layerId });
  };

  const getCurrentSelection = () =>
    selectedIds.length > 0 ? selectedIds : activeLayer ? [activeLayer.id] : [];

  const getSelectedLayers = () =>
    getCurrentSelection()
      .map((layerId) => findLayerById(layers, layerId))
      .filter((layer): layer is LayerNodeMeta => layer !== null);

  const deleteLayers = (layerIds: string[]) => {
    const orderedIds = [...layerIds].sort((left, right) => {
      return (
        displayOrder.findIndex((candidate) => candidate.id === left) -
        displayOrder.findIndex((candidate) => candidate.id === right)
      );
    });
    for (const layerId of orderedIds.reverse()) {
      engine.dispatchCommand(CommandID.DeleteLayer, { layerId });
    }
    setSelectedLayerIds([]);
    setLastSelectedLayerId(null);
  };

  const duplicateLayers = (layerIds: string[]) => {
    const orderedIds = [...layerIds].sort((left, right) => {
      return (
        displayOrder.findIndex((candidate) => candidate.id === left) -
        displayOrder.findIndex((candidate) => candidate.id === right)
      );
    });
    for (const layerId of orderedIds) {
      engine.dispatchCommand(CommandID.DuplicateLayer, { layerId });
    }
  };

  const addMaskToSelection = (mode: AddLayerMaskMode) => {
    for (const layer of getSelectedLayers()) {
      engine.dispatchCommand(CommandID.AddLayerMask, {
        layerId: layer.id,
        mode,
      });
    }
  };

  const toggleMaskEnabledForSelection = () => {
    for (const layer of getSelectedLayers()) {
      if (!layer.hasMask) {
        continue;
      }
      engine.dispatchCommand(CommandID.SetLayerMaskEnabled, {
        layerId: layer.id,
        enabled: !layer.maskEnabled,
      });
    }
  };

  const toggleClipForSelection = (nextClipState: boolean) => {
    for (const layer of getSelectedLayers()) {
      engine.dispatchCommand(CommandID.SetLayerClipToBelow, {
        layerId: layer.id,
        clipToBelow: nextClipState,
      });
    }
  };

  const groupSelection = () => {
    const selection = getSelectedLayers();
    if (selection.length < 2) {
      return;
    }

    const parentId = selection[0]?.parentId;
    if (selection.some((layer) => layer.parentId !== parentId)) {
      return;
    }

    const orderedIds = displayOrder
      .filter((candidate) => selection.some((layer) => layer.id === candidate.id))
      .map((candidate) => candidate.id);
    const parentChildren = getChildrenForParent(layers, parentId);
    const insertIndex = parentChildren.findIndex((candidate) => candidate.id === orderedIds[0]);
    const render = engine.dispatchCommand(CommandID.AddLayer, {
      layerType: "group",
      name: `Group ${layerCount + 1}`,
      parentLayerId: parentId || undefined,
      index: insertIndex >= 0 ? insertIndex : undefined,
      isolated: true,
    });
    const groupId = render?.uiMeta.activeLayerId;
    if (!groupId) {
      return;
    }

    orderedIds.forEach((layerId, index) => {
      engine.dispatchCommand(CommandID.MoveLayer, {
        layerId,
        parentLayerId: groupId,
        index,
      });
    });
    setSelectedLayerIds([groupId]);
    setLastSelectedLayerId(groupId);
  };

  const ungroupSelection = () => {
    const selection = getSelectedLayers();
    if (selection.length !== 1 || selection[0].layerType !== "group") {
      return;
    }

    const group = selection[0];
    const parentId = group.parentId;
    const parentChildren = getChildrenForParent(layers, parentId);
    const groupIndex = parentChildren.findIndex((candidate) => candidate.id === group.id);
    if (groupIndex < 0) {
      return;
    }

    const childIds = [...(group.children ?? [])].reverse().map((child) => child.id);
    childIds.forEach((layerId, index) => {
      engine.dispatchCommand(CommandID.MoveLayer, {
        layerId,
        parentLayerId: parentId || undefined,
        index: groupIndex + index,
      });
    });
    engine.dispatchCommand(CommandID.DeleteLayer, { layerId: group.id });
    setSelectedLayerIds(childIds);
    setLastSelectedLayerId(childIds.at(-1) ?? null);
  };

  const openContextMenu = (layer: LayerNodeMeta, x: number, y: number) => {
    if (!selectedIdSet.has(layer.id)) {
      setSelectedLayerIds([layer.id]);
      setLastSelectedLayerId(layer.id);
      engine.dispatchCommand(CommandID.SetActiveLayer, { layerId: layer.id });
    }
    setContextMenu({ layerId: layer.id, x, y });
  };

  const contextLayer = contextMenu ? findLayerById(layers, contextMenu.layerId) : null;
  const canGroupSelection =
    getSelectedLayers().length >= 2 &&
    new Set(getSelectedLayers().map((layer) => layer.parentId ?? "")).size <= 1;
  const canUngroupSelection = selectedIds.length === 1 && contextLayer?.layerType === "group";

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
          {selectedIds.length > 1 ? `${selectedIds.length} selected` : layerCount}
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
                  selectedLayerIds={selectedIds}
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
                  onDuplicate={(layerId) => duplicateLayers([layerId])}
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
                  onOpenContextMenu={openContextMenu}
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
                    addMaskToSelection("reveal-all");
                    return;
                  }
                  toggleMaskEnabledForSelection();
                }}
              />
              <ActionButton
                label={activeLayer?.clipToBelow ? "Release Clip" : "Clip To Below"}
                disabled={!activeLayer}
                onClick={() => {
                  if (!activeLayer) {
                    return;
                  }
                  toggleClipForSelection(!activeLayer.clipToBelow);
                }}
              />
            </div>
            <div className="grid grid-cols-2 gap-[var(--ui-gap-1)]">
              <ActionButton label="Group" disabled={!canGroupSelection} onClick={groupSelection} />
              <ActionButton
                label="Ungroup"
                disabled={!canUngroupSelection}
                onClick={ungroupSelection}
              />
            </div>
          </div>
        </div>
      </div>

      {contextMenu ? (
        <LayerContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          canGroupSelection={canGroupSelection}
          canUngroupSelection={canUngroupSelection}
          onClose={() => setContextMenu(null)}
          onDuplicate={() => duplicateLayers(getCurrentSelection())}
          onDelete={() => deleteLayers(getCurrentSelection())}
          onMergeDown={() => {
            if (contextLayer) {
              engine.dispatchCommand(CommandID.MergeDown, {
                layerId: contextLayer.id,
              });
            }
          }}
          onMergeVisible={() => engine.dispatchCommand(CommandID.MergeVisible)}
          onGroup={groupSelection}
          onUngroup={ungroupSelection}
          onAddMaskReveal={() => addMaskToSelection("reveal-all")}
          onAddMaskHide={() => addMaskToSelection("hide-all")}
          onAddMaskFromSelection={() => addMaskToSelection("from-selection")}
          onToggleClip={() => {
            if (!contextLayer) {
              return;
            }
            toggleClipForSelection(!contextLayer.clipToBelow);
          }}
        />
      ) : null}
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
  selectedLayerIds: string[];
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
  onSelect: (
    layerId: string,
    event?: Pick<MouseEvent<HTMLElement>, "shiftKey" | "ctrlKey" | "metaKey">,
  ) => void;
  onToggleVisibility: (layerId: string, visible: boolean) => void;
  onCycleLock: (layerId: string, lockMode: LayerLockMode) => void;
  onDuplicate: (layerId: string) => void;
  onDragStart: (layerId: string) => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent<HTMLDivElement>, layer: LayerNodeMeta) => void;
  onDropLayer: (layer: LayerNodeMeta) => void;
  onOpenContextMenu: (layer: LayerNodeMeta, x: number, y: number) => void;
};

function LayerTreeRow({
  layer,
  depth,
  activeLayerId,
  selectedLayerIds,
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
  onOpenContextMenu,
}: LayerTreeRowProps) {
  const isGroup = layer.layerType === "group";
  const isCollapsed = isGroup && collapsedGroups[layer.id];
  const isActive = layer.id === activeLayerId;
  const isSelected = selectedLayerIds.includes(layer.id);
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
            isSelected || isActive
              ? "border-cyan-400/35 bg-cyan-400/10"
              : "border-white/8 bg-white/[0.02] hover:border-white/12 hover:bg-white/[0.04]",
            dropState === "inside" ? "border-cyan-300/60 bg-cyan-300/10" : "",
          ].join(" ")}
          role="treeitem"
          tabIndex={0}
          aria-selected={isSelected || isActive}
          draggable={!isEditing}
          onClick={(event) => onSelect(layer.id, event)}
          onContextMenu={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onOpenContextMenu(layer, event.clientX, event.clientY);
          }}
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

            <LayerThumbnail layer={layer} />

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
              selectedLayerIds={selectedLayerIds}
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
              onOpenContextMenu={onOpenContextMenu}
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

function LayerThumbnail({ layer }: { layer: LayerNodeMeta }) {
  const toneClass =
    layer.layerType === "group"
      ? "from-slate-500/60 via-slate-700/60 to-slate-950"
      : layer.layerType === "pixel"
        ? "from-cyan-500/25 via-slate-800/60 to-slate-950"
        : layer.layerType === "text"
          ? "from-amber-500/20 via-slate-800/60 to-slate-950"
          : layer.layerType === "vector"
            ? "from-emerald-500/20 via-slate-800/60 to-slate-950"
            : "from-fuchsia-500/20 via-slate-800/60 to-slate-950";

  return (
    <div
      className={[
        "relative flex h-9 w-9 items-center justify-center overflow-hidden rounded-[var(--ui-radius-sm)] border border-white/8 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] text-[9px] font-semibold uppercase tracking-[0.16em] text-slate-200",
        layer.hasMask && !layer.maskEnabled ? "opacity-60" : "",
      ].join(" ")}
      title={`${layer.layerType} layer${layer.hasMask ? (layer.maskEnabled ? ", mask enabled" : ", mask disabled") : ""}`}
    >
      <div className={`absolute inset-0 bg-gradient-to-br ${toneClass}`} />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.18),transparent_60%)]" />
      <span className="relative z-10 text-[8px] uppercase tracking-[0.18em] text-slate-100">
        {layer.layerType === "group" ? "grp" : layer.layerType.slice(0, 2)}
      </span>
      {layer.clipToBelow ? (
        <span className="absolute left-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-sky-300" />
      ) : null}
      {layer.hasMask ? (
        <span className="absolute bottom-0.5 right-0.5 h-1.5 w-1.5 rounded-full bg-fuchsia-300" />
      ) : null}
    </div>
  );
}

type LayerContextMenuState = {
  layerId: string;
  x: number;
  y: number;
} | null;

function LayerContextMenu({
  x,
  y,
  canGroupSelection,
  canUngroupSelection,
  onClose,
  onDuplicate,
  onDelete,
  onMergeDown,
  onMergeVisible,
  onGroup,
  onUngroup,
  onAddMaskReveal,
  onAddMaskHide,
  onAddMaskFromSelection,
  onToggleClip,
}: {
  x: number;
  y: number;
  canGroupSelection: boolean;
  canUngroupSelection: boolean;
  onClose: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  onMergeDown: () => void;
  onMergeVisible: () => void;
  onGroup: () => void;
  onUngroup: () => void;
  onAddMaskReveal: () => void;
  onAddMaskHide: () => void;
  onAddMaskFromSelection: () => void;
  onToggleClip: () => void;
}) {
  return (
    <div
      role="menu"
      className="fixed z-50 min-w-56 rounded-[var(--ui-radius-md)] border border-white/10 bg-[#171b21] p-1 shadow-[0_14px_36px_rgba(0,0,0,0.42)]"
      style={{ left: x, top: y }}
      onContextMenu={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <MenuAction label="Duplicate Layer" onClick={onDuplicate} />
      <MenuAction label="Delete Layer" onClick={onDelete} destructive />
      <MenuSeparator />
      <MenuAction label="Merge Down" onClick={onMergeDown} />
      <MenuAction label="Merge Visible" onClick={onMergeVisible} />
      <MenuSeparator />
      <MenuAction label="Group Layers" onClick={onGroup} disabled={!canGroupSelection} />
      <MenuAction label="Ungroup" onClick={onUngroup} disabled={!canUngroupSelection} />
      <MenuSeparator />
      <MenuAction label="Add Mask: Reveal All" onClick={onAddMaskReveal} />
      <MenuAction label="Add Mask: Hide All" onClick={onAddMaskHide} />
      <MenuAction label="Add Mask: From Selection" onClick={onAddMaskFromSelection} />
      <MenuSeparator />
      <MenuAction label="Toggle Clipping" onClick={onToggleClip} />
    </div>
  );
}

function MenuAction({
  label,
  onClick,
  disabled,
  destructive,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  destructive?: boolean;
}) {
  return (
    <button
      type="button"
      className={[
        "flex w-full items-center rounded-[var(--ui-radius-sm)] px-2.5 py-1.5 text-left text-[12px] transition",
        disabled
          ? "cursor-not-allowed text-slate-600"
          : destructive
            ? "text-rose-200 hover:bg-rose-500/12"
            : "text-slate-100 hover:bg-white/6",
      ].join(" ")}
      disabled={disabled}
      onClick={() => {
        if (disabled) {
          return;
        }
        onClick();
      }}
    >
      {label}
    </button>
  );
}

function MenuSeparator() {
  return <div className="my-1 h-px bg-white/8" />;
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

function collectLayerOrder(
  layers: LayerNodeMeta[],
  collapsedGroups: Record<string, boolean>,
  output: LayerNodeMeta[] = [],
) {
  for (let index = layers.length - 1; index >= 0; index--) {
    const layer = layers[index];
    output.push(layer);
    if (layer.layerType === "group" && !collapsedGroups[layer.id]) {
      collectLayerOrder(layer.children ?? [], collapsedGroups, output);
    }
  }
  return output;
}

function getLayerSelectionRange(
  orderedLayers: LayerNodeMeta[],
  startLayerId: string,
  endLayerId: string,
) {
  const startIndex = orderedLayers.findIndex((layer) => layer.id === startLayerId);
  const endIndex = orderedLayers.findIndex((layer) => layer.id === endLayerId);
  if (startIndex < 0 || endIndex < 0) {
    return [];
  }

  const from = Math.min(startIndex, endIndex);
  const to = Math.max(startIndex, endIndex);
  return orderedLayers.slice(from, to + 1).map((layer) => layer.id);
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
