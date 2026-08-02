import {
  type AddLayerMaskMode,
  CommandID,
  type DocumentStylePresetEntry,
  type LayerBlendMode,
  type LayerLockMode,
  type LayerNodeMeta,
  type ThumbnailEntry,
} from "@agogo/proto";
import {
  type DragEvent,
  type KeyboardEvent,
  type MouseEvent,
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { LayerStyleDialog } from "@/components/layer-style-dialog";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  type TransactionalSliderCommitProps,
  useTransactionalSlider,
} from "@/hooks/use-transactional-slider";
import { parseNumericInput } from "@/lib/utils";
import type { EngineContextValue } from "@/wasm/types";

type BlendModeGroup = { label: string; modes: LayerBlendMode[] };

const blendModeGroups: BlendModeGroup[] = [
  { label: "Normal", modes: ["normal", "dissolve"] },
  {
    label: "Darken",
    modes: ["darken", "multiply", "color-burn", "linear-burn", "darker-color"],
  },
  {
    label: "Lighten",
    modes: ["lighten", "screen", "color-dodge", "linear-dodge", "lighter-color"],
  },
  {
    label: "Contrast",
    modes: [
      "overlay",
      "soft-light",
      "hard-light",
      "vivid-light",
      "linear-light",
      "pin-light",
      "hard-mix",
    ],
  },
  {
    label: "Inversion",
    modes: ["difference", "exclusion", "subtract", "divide"],
  },
  { label: "Component", modes: ["hue", "saturation", "color", "luminosity"] },
];

const lockModeCycle: LayerLockMode[] = ["none", "pixels", "position", "all"];

type DropPosition = "before" | "after" | "inside";

type DropTarget = {
  layerId: string;
  position: DropPosition;
} | null;

const THUMBNAIL_SIZE = 32;

// Color labels for layer organization (UI-only, not persisted in the engine).
const COLOR_TAGS = [
  {
    id: "none",
    label: "None",
    bg: "bg-transparent",
    border: "border-white/20",
  },
  { id: "red", label: "Red", bg: "bg-rose-500", border: "border-rose-400" },
  {
    id: "orange",
    label: "Orange",
    bg: "bg-orange-500",
    border: "border-orange-400",
  },
  {
    id: "yellow",
    label: "Yellow",
    bg: "bg-yellow-400",
    border: "border-yellow-300",
  },
  {
    id: "green",
    label: "Green",
    bg: "bg-emerald-500",
    border: "border-emerald-400",
  },
  { id: "blue", label: "Blue", bg: "bg-blue-500", border: "border-blue-400" },
  {
    id: "violet",
    label: "Violet",
    bg: "bg-violet-500",
    border: "border-violet-400",
  },
  { id: "gray", label: "Gray", bg: "bg-slate-500", border: "border-slate-400" },
] as const;

type ColorTagId = (typeof COLOR_TAGS)[number]["id"];

type LayersPanelProps = {
  engine: EngineContextValue;
  layers: LayerNodeMeta[];
  activeLayerId: string | null;
  maskEditLayerId: string | null;
  documentWidth: number;
  documentHeight: number;
  stylePresets?: DocumentStylePresetEntry[];
  thumbnails: Record<string, ThumbnailEntry>;
  selectedLayerIds: string[];
  onSelectedLayerIdsChange: (ids: string[]) => void;
};

export function LayersPanel({
  engine,
  layers,
  activeLayerId,
  maskEditLayerId,
  documentWidth,
  documentHeight,
  stylePresets = [],
  thumbnails,
  selectedLayerIds,
  onSelectedLayerIdsChange,
}: LayersPanelProps) {
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [editingLayerId, setEditingLayerId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState("");
  const [draggedLayerId, setDraggedLayerId] = useState<string | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget>(null);
  const [lastSelectedLayerId, setLastSelectedLayerId] = useState<string | null>(null);
  const [contextMenu, setContextMenu] = useState<LayerContextMenuState>(null);
  const closeContextMenu = useCallback(() => setContextMenu(null), []);
  const [colorTags, setColorTags] = useState<Record<string, ColorTagId>>({});
  const [propertiesLayerId, setPropertiesLayerId] = useState<string | null>(null);
  const [layerStyleLayerId, setLayerStyleLayerId] = useState<string | null>(null);

  const activeLayer = useMemo(
    () => findLayerById(layers, activeLayerId ?? "") ?? firstLayer(layers),
    [activeLayerId, layers],
  );
  const opacitySlider = useTransactionalSlider<number>({
    label: "Change layer opacity",
    engine,
    dispatch: (value) => {
      if (!activeLayer) {
        return;
      }
      engine.dispatchCommand(CommandID.SetLayerOpacity, {
        layerId: activeLayer.id,
        opacity: value / 100,
      });
    },
  });
  const fillSlider = useTransactionalSlider<number>({
    label: "Change fill opacity",
    engine,
    dispatch: (value) => {
      if (!activeLayer) {
        return;
      }
      engine.dispatchCommand(CommandID.SetLayerOpacity, {
        layerId: activeLayer.id,
        fillOpacity: value / 100,
      });
    },
  });

  const layerCount = useMemo(() => countLayers(layers), [layers]);
  const displayOrder = useMemo(
    () => collectLayerOrder(layers, collapsedGroups),
    [collapsedGroups, layers],
  );
  const selectedIds = useMemo(
    () => (selectedLayerIds.length > 0 ? selectedLayerIds : activeLayer ? [activeLayer.id] : []),
    [selectedLayerIds, activeLayer],
  );
  const selectedIdSet = useMemo(() => new Set(selectedIds), [selectedIds]);

  // Row-facing handlers below are stabilized with useCallback so a memoized
  // LayerTreeRow bails out of re-rendering on viewport-only commits (where the
  // engine emits a new uiMeta but preserves layer-node identity). To keep those
  // callbacks stable while still reading fresh state, they pull the values that
  // change per render from this ref instead of closing over them directly.
  const latestRef = useRef({
    layers,
    displayOrder,
    selectedLayerIds,
    selectedIdSet,
    lastSelectedLayerId,
    maskEditLayerId,
    draggedLayerId,
    dropTarget,
    editingLayerId,
    editingName,
    onSelectedLayerIdsChange,
  });
  latestRef.current = {
    layers,
    displayOrder,
    selectedLayerIds,
    selectedIdSet,
    lastSelectedLayerId,
    maskEditLayerId,
    draggedLayerId,
    dropTarget,
    editingLayerId,
    editingName,
    onSelectedLayerIdsChange,
  };

  const selectLayer = useCallback(
    (
      layerId: string,
      event?: Pick<MouseEvent<HTMLElement>, "shiftKey" | "ctrlKey" | "metaKey">,
    ) => {
      const {
        lastSelectedLayerId: lastSelected,
        displayOrder: order,
        selectedLayerIds: currentSelection,
        onSelectedLayerIdsChange: onChange,
      } = latestRef.current;
      const additiveSelection = Boolean(event?.ctrlKey || event?.metaKey);
      if (event?.shiftKey && lastSelected) {
        const rangeSelection = getLayerSelectionRange(order, lastSelected, layerId);
        onChange(rangeSelection.length > 0 ? rangeSelection : [layerId]);
      } else if (additiveSelection) {
        onChange(
          currentSelection.includes(layerId)
            ? currentSelection.filter((candidate) => candidate !== layerId)
            : [...currentSelection, layerId],
        );
      } else {
        onChange([layerId]);
      }
      setLastSelectedLayerId(layerId);
      engine.dispatchCommand(CommandID.SetActiveLayer, { layerId });
    },
    [engine],
  );

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
    onSelectedLayerIdsChange([]);
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

  const toggleMaskEdit = useCallback(
    (layerId: string) => {
      const isCurrentlyEditing = latestRef.current.maskEditLayerId === layerId;
      engine.dispatchCommand(CommandID.SetMaskEditMode, {
        layerId,
        editing: !isCurrentlyEditing,
      });
    },
    [engine],
  );

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
    onSelectedLayerIdsChange([groupId]);
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
    onSelectedLayerIdsChange(childIds);
    setLastSelectedLayerId(childIds.at(-1) ?? null);
  };

  const openContextMenu = useCallback(
    (layer: LayerNodeMeta, x: number, y: number) => {
      const { selectedIdSet: currentSelectedIdSet, onSelectedLayerIdsChange: onChange } =
        latestRef.current;
      if (!currentSelectedIdSet.has(layer.id)) {
        onChange([layer.id]);
        setLastSelectedLayerId(layer.id);
        engine.dispatchCommand(CommandID.SetActiveLayer, { layerId: layer.id });
      }
      setContextMenu({ layerId: layer.id, x, y });
    },
    [engine],
  );

  const openLayerProperties = (layer: LayerNodeMeta) => {
    setPropertiesLayerId(layer.id);
    setContextMenu(null);
  };

  const openLayerStyle = (layerId: string) => {
    setLayerStyleLayerId(layerId);
    setPropertiesLayerId(null);
    setContextMenu(null);
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

  const startRename = useCallback(
    (layer: LayerNodeMeta) => {
      selectLayer(layer.id);
      setEditingLayerId(layer.id);
      setEditingName(layer.name);
    },
    [selectLayer],
  );

  const cancelRename = useCallback(() => {
    setEditingLayerId(null);
    setEditingName("");
  }, []);

  const commitRename = useCallback(() => {
    const { editingLayerId: currentEditingId, editingName: currentName } = latestRef.current;
    if (!currentEditingId) {
      return;
    }
    engine.dispatchCommand(CommandID.SetLayerName, {
      layerId: currentEditingId,
      name: currentName.trim(),
    });
    setEditingLayerId(null);
    setEditingName("");
  }, [engine]);

  const moveLayer = useCallback(
    (layerId: string, targetLayerId: string, position: DropPosition) => {
      if (layerId === targetLayerId) {
        return;
      }

      const { layers: currentLayers } = latestRef.current;
      const targetLayer = findLayerById(currentLayers, targetLayerId);
      if (!targetLayer) {
        return;
      }
      const draggedLayer = findLayerById(currentLayers, layerId);
      const draggedIsArtboard = Boolean(draggedLayer?.isArtboard);

      if (position === "inside") {
        if (
          draggedIsArtboard ||
          targetLayer.layerType !== "group" ||
          isDescendantLayer(currentLayers, layerId, targetLayer.id)
        ) {
          return;
        }
        engine.dispatchCommand(CommandID.MoveLayer, {
          layerId,
          parentLayerId: targetLayer.id,
          index: targetLayer.children?.length ?? 0,
        });
        return;
      }

      const siblings = getChildrenForParent(currentLayers, targetLayer.parentId);
      const targetIndex = siblings.findIndex((candidate) => candidate.id === targetLayer.id);
      if (targetIndex < 0) {
        return;
      }
      if (draggedIsArtboard && targetLayer.parentId) {
        return;
      }

      engine.dispatchCommand(CommandID.MoveLayer, {
        layerId,
        parentLayerId: draggedIsArtboard ? undefined : targetLayer.parentId || undefined,
        index: position === "before" ? targetIndex + 1 : targetIndex,
      });
    },
    [engine],
  );

  const handleDragOver = useCallback((event: DragEvent<HTMLDivElement>, layer: LayerNodeMeta) => {
    const { draggedLayerId: currentDragged, layers: currentLayers } = latestRef.current;
    if (!currentDragged || currentDragged === layer.id) {
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
      !isDescendantLayer(currentLayers, currentDragged, layer.id)
    ) {
      position = "inside";
    }

    setDropTarget({ layerId: layer.id, position });
  }, []);

  const handleDrop = useCallback(
    (layer: LayerNodeMeta) => {
      const { draggedLayerId: currentDragged, dropTarget: currentDropTarget } = latestRef.current;
      if (!currentDragged || !currentDropTarget || currentDropTarget.layerId !== layer.id) {
        return;
      }
      moveLayer(currentDragged, layer.id, currentDropTarget.position);
      setDraggedLayerId(null);
      setDropTarget(null);
    },
    [moveLayer],
  );

  const handleToggleGroup = useCallback((layerId: string) => {
    setCollapsedGroups((current) => ({
      ...current,
      [layerId]: !current[layerId],
    }));
  }, []);

  const handleToggleVisibility = useCallback(
    (layerId: string, visible: boolean, solo: boolean) => {
      engine.dispatchCommand(
        solo ? CommandID.SoloLayerVisibility : CommandID.SetLayerVisibility,
        solo ? { layerId } : { layerId, visible },
      );
    },
    [engine],
  );

  const handleCycleLock = useCallback(
    (layerId: string, lockMode: LayerLockMode) => {
      engine.dispatchCommand(CommandID.SetLayerLock, {
        layerId,
        lockMode: nextLockMode(lockMode),
      });
    },
    [engine],
  );

  const handleDuplicateLayer = useCallback(
    (layerId: string) => {
      engine.dispatchCommand(CommandID.DuplicateLayer, { layerId });
    },
    [engine],
  );

  const handleDragStart = useCallback(
    (layerId: string) => {
      setDraggedLayerId(layerId);
      selectLayer(layerId);
    },
    [selectLayer],
  );

  const handleDragEnd = useCallback(() => {
    setDraggedLayerId(null);
    setDropTarget(null);
  }, []);

  const handleEnterVectorEdit = useCallback(
    (layerId: string) => {
      engine.dispatchCommand(CommandID.EnterVectorEditMode, { layerId });
    },
    [engine],
  );

  const handleEnterTextEdit = useCallback(
    (layerId: string) => {
      engine.dispatchCommand(CommandID.EnterTextEditMode, { layerId });
    },
    [engine],
  );

  return (
    <div className="flex h-full min-h-0 flex-col gap-[var(--ui-gap-2)]">
      <div className="flex items-center justify-between gap-2 text-[11px]">
        <div className="flex items-center gap-2 text-muted-foreground">
          <span className="font-medium text-foreground">Active</span>
          <span className="truncate text-muted-foreground">{activeLayer?.name ?? "None"}</span>
        </div>
        <div className="rounded-[var(--ui-radius-sm)] border border-border bg-black/12 px-1.5 py-1 text-muted-foreground">
          {selectedIds.length > 1 ? `${selectedIds.length} selected` : layerCount}
        </div>
      </div>

      <div className="grid grid-cols-6 gap-[var(--ui-gap-1)]">
        <ToolbarAction label="+L" title="New Layer" onClick={addPixelLayer} />
        <ToolbarAction label="+G" title="New Group" onClick={addGroupLayer} />
        <ToolbarAction
          label="+A"
          title="New Artboard"
          onClick={() =>
            engine.dispatchCommand(CommandID.AddLayer, {
              layerType: "group",
              name: `Artboard ${layerCount + 1}`,
              isArtboard: true,
              artboardBounds: {
                x: Math.round(documentWidth * 0.1),
                y: Math.round(documentHeight * 0.1),
                w: Math.max(320, Math.round(documentWidth * 0.6)),
                h: Math.max(240, Math.round(documentHeight * 0.6)),
              },
              artboardBackground: [255, 255, 255, 255],
            })
          }
        />
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
        <ScrollArea className="min-h-0 rounded-[var(--ui-radius-md)] border border-border bg-black/12">
          {layers.length === 0 ? (
            <div className="px-3 py-4 text-[12px] text-muted-foreground">
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
                  maskEditLayerId={maskEditLayerId}
                  selectedLayerIds={selectedIds}
                  collapsedGroups={collapsedGroups}
                  draggedLayerId={draggedLayerId}
                  dropTarget={dropTarget}
                  editingLayerId={editingLayerId}
                  editingName={editingName}
                  thumbnails={thumbnails}
                  colorTags={colorTags}
                  onEditingNameChange={setEditingName}
                  onStartRename={startRename}
                  onCommitRename={commitRename}
                  onCancelRename={cancelRename}
                  onToggleGroup={handleToggleGroup}
                  onSelect={selectLayer}
                  onToggleVisibility={handleToggleVisibility}
                  onCycleLock={handleCycleLock}
                  onToggleMaskEdit={toggleMaskEdit}
                  onDuplicate={handleDuplicateLayer}
                  onDragStart={handleDragStart}
                  onDragEnd={handleDragEnd}
                  onDragOver={handleDragOver}
                  onDropLayer={handleDrop}
                  onOpenContextMenu={openContextMenu}
                  onEnterVectorEdit={handleEnterVectorEdit}
                  onEnterTextEdit={handleEnterTextEdit}
                />
              ))}
            </div>
          )}
        </ScrollArea>

        <div className="rounded-[var(--ui-radius-md)] border border-border bg-black/12 p-[var(--ui-gap-2)]">
          <div className="grid gap-[var(--ui-gap-2)]">
            <div className="grid grid-cols-[1fr_auto] items-center gap-2">
              <label className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground/70">
                Blend
                <select
                  className="mt-1 h-[var(--ui-h-sm)] w-full rounded-[var(--ui-radius-md)] border border-border bg-panel-soft px-2 text-[12px] text-foreground disabled:cursor-not-allowed disabled:opacity-50"
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
                  {blendModeGroups.map((group) => (
                    <optgroup key={group.label} label={group.label}>
                      {group.modes.map((mode) => (
                        <option key={mode} value={mode}>
                          {formatBlendMode(mode)}
                        </option>
                      ))}
                    </optgroup>
                  ))}
                </select>
              </label>
              <div className="text-right text-[11px] text-muted-foreground">
                {activeLayer ? describeLayer(activeLayer) : "No selection"}
              </div>
            </div>

            <RangeField
              label="Opacity"
              disabled={!activeLayer}
              value={Math.round((activeLayer?.opacity ?? 1) * 100)}
              onChange={opacitySlider.change}
              commitProps={opacitySlider.commitProps}
            />

            <RangeField
              label="Fill"
              disabled={!activeLayer}
              value={Math.round((activeLayer?.fillOpacity ?? 1) * 100)}
              onChange={fillSlider.change}
              commitProps={fillSlider.commitProps}
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
          onClose={closeContextMenu}
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
          onFlattenImage={() => engine.dispatchCommand(CommandID.FlattenImage)}
          onGroup={groupSelection}
          onUngroup={ungroupSelection}
          onAddMaskReveal={() => addMaskToSelection("reveal-all")}
          onAddMaskHide={() => addMaskToSelection("hide-all")}
          onAddMaskFromSelection={() => addMaskToSelection("from-selection")}
          onAddVectorMask={() => {
            if (contextLayer) {
              engine.dispatchCommand(CommandID.AddVectorMask, {
                layerId: contextLayer.id,
              });
            }
          }}
          onDeleteVectorMask={() => {
            if (contextLayer) {
              engine.dispatchCommand(CommandID.DeleteVectorMask, {
                layerId: contextLayer.id,
              });
            }
          }}
          onToggleClip={() => {
            if (!contextLayer) {
              return;
            }
            toggleClipForSelection(!contextLayer.clipToBelow);
          }}
          onLayerStyle={() => {
            if (contextLayer) {
              openLayerStyle(contextLayer.id);
            }
          }}
          onCopyLayerStyle={() => {
            if (!contextLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.CopyLayerStyle, {
              layerId: contextLayer.id,
            });
          }}
          onPasteLayerStyle={() => {
            if (!contextLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.PasteLayerStyle, {
              layerId: contextLayer.id,
            });
          }}
          onClearLayerStyle={() => {
            if (!contextLayer) {
              return;
            }
            engine.dispatchCommand(CommandID.ClearLayerStyle, {
              layerId: contextLayer.id,
            });
          }}
          onLayerProperties={() => {
            if (contextLayer) {
              openLayerProperties(contextLayer);
            }
          }}
        />
      ) : null}

      {propertiesLayerId ? (
        <LayerPropertiesDialog
          layer={findLayerById(layers, propertiesLayerId)}
          colorTag={colorTags[propertiesLayerId] ?? "none"}
          onRename={(name) => {
            engine.dispatchCommand(CommandID.SetLayerName, {
              layerId: propertiesLayerId,
              name,
            });
          }}
          onColorTag={(tag) =>
            setColorTags((current) => ({
              ...current,
              [propertiesLayerId]: tag,
            }))
          }
          onOpenLayerStyle={() => openLayerStyle(propertiesLayerId)}
          onClose={() => setPropertiesLayerId(null)}
        />
      ) : null}

      <LayerStyleDialog
        open={layerStyleLayerId !== null}
        engine={engine}
        layer={layerStyleLayerId ? findLayerById(layers, layerStyleLayerId) : null}
        presets={stylePresets}
        onClose={() => setLayerStyleLayerId(null)}
      />
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
      aria-label={title}
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
  maskEditLayerId: string | null;
  selectedLayerIds: string[];
  collapsedGroups: Record<string, boolean>;
  draggedLayerId: string | null;
  dropTarget: DropTarget;
  editingLayerId: string | null;
  editingName: string;
  thumbnails: Record<string, ThumbnailEntry>;
  colorTags: Record<string, ColorTagId>;
  onEditingNameChange: (value: string) => void;
  onStartRename: (layer: LayerNodeMeta) => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  onToggleGroup: (layerId: string) => void;
  onSelect: (
    layerId: string,
    event?: Pick<MouseEvent<HTMLElement>, "shiftKey" | "ctrlKey" | "metaKey">,
  ) => void;
  onToggleVisibility: (layerId: string, visible: boolean, solo: boolean) => void;
  onCycleLock: (layerId: string, lockMode: LayerLockMode) => void;
  onToggleMaskEdit: (layerId: string) => void;
  onDuplicate: (layerId: string) => void;
  onDragStart: (layerId: string) => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent<HTMLDivElement>, layer: LayerNodeMeta) => void;
  onDropLayer: (layer: LayerNodeMeta) => void;
  onOpenContextMenu: (layer: LayerNodeMeta, x: number, y: number) => void;
  onEnterVectorEdit: (layerId: string) => void;
  onEnterTextEdit: (layerId: string) => void;
};

const LayerTreeRow = memo(function LayerTreeRowInner({
  layer,
  depth,
  activeLayerId,
  maskEditLayerId,
  selectedLayerIds,
  collapsedGroups,
  draggedLayerId,
  dropTarget,
  editingLayerId,
  editingName,
  thumbnails,
  colorTags,
  onEditingNameChange,
  onStartRename,
  onCommitRename,
  onCancelRename,
  onToggleGroup,
  onSelect,
  onToggleVisibility,
  onCycleLock,
  onToggleMaskEdit,
  onDuplicate,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDropLayer,
  onOpenContextMenu,
  onEnterVectorEdit,
  onEnterTextEdit,
}: LayerTreeRowProps) {
  const isGroup = layer.layerType === "group";
  const isCollapsed = isGroup && collapsedGroups[layer.id];
  const isActive = layer.id === activeLayerId;
  const isSelected = selectedLayerIds.includes(layer.id);
  const isDragging = layer.id === draggedLayerId;
  const isEditing = layer.id === editingLayerId;
  const isEditingMask = layer.id === maskEditLayerId;
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
            dropState === "before" ? "bg-accent/90" : "bg-transparent",
          ].join(" ")}
        />

        <div
          className={[
            "rounded-[var(--ui-radius-md)] border transition",
            isDragging ? "border-border/40 bg-muted/20 opacity-50" : "",
            isEditingMask
              ? "border-orange-400/60 bg-orange-400/8"
              : isSelected || isActive
                ? "border-accent/35 bg-accent/10"
                : "border-border/60 bg-muted/20 hover:border-border hover:bg-muted/30",
            dropState === "inside" ? "border-accent/60 bg-accent/10" : "",
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
                  className="flex h-5 w-5 items-center justify-center rounded-[var(--ui-radius-sm)] text-[10px] text-muted-foreground transition hover:bg-muted/50 hover:text-foreground focus-visible:outline-none"
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
                  "flex h-5 min-w-5 items-center justify-center rounded-[var(--ui-radius-sm)] px-1 text-[10px] transition focus-visible:outline-none",
                  layer.visible
                    ? "bg-success/12 text-success"
                    : "bg-black/20 text-muted-foreground/70",
                ].join(" ")}
                onClick={(event) => {
                  event.stopPropagation();
                  onToggleVisibility(layer.id, !layer.visible, event.altKey || event.metaKey);
                }}
                title={layer.visible ? "Hide layer" : "Show layer"}
                aria-label={layer.visible ? "Hide layer" : "Show layer"}
              >
                {layer.visible ? "O" : "-"}
              </button>
            </div>

            <LayerThumbnail
              layer={layer}
              thumbnail={thumbnails[layer.id]}
              colorTag={colorTags[layer.id] ?? "none"}
              isEditingMask={isEditingMask}
              onToggleMaskEdit={() => onToggleMaskEdit(layer.id)}
              onDoubleClick={
                layer.layerType === "vector"
                  ? () => onEnterVectorEdit(layer.id)
                  : layer.layerType === "text"
                    ? () => onEnterTextEdit(layer.id)
                    : undefined
              }
            />

            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-[var(--ui-gap-1)]">
                {isEditing ? (
                  <input
                    className="h-6 w-full rounded-[var(--ui-radius-sm)] border border-accent/30 bg-black/25 px-1.5 text-[12px] text-foreground outline-none focus-visible:ring-1 focus-visible:ring-ring/30"
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
                    className="min-w-0 rounded-[var(--ui-radius-sm)] text-left focus-visible:outline-none"
                    onDoubleClick={(event) => {
                      event.stopPropagation();
                      onStartRename(layer);
                    }}
                  >
                    <span className="block truncate text-[12px] font-medium text-foreground">
                      {layer.name}
                    </span>
                  </button>
                )}
                {layer.clippingBase ? <MiniBadge label="base" tone="amber" /> : null}
                {layer.clipToBelow ? <MiniBadge label="clip" tone="sky" /> : null}
                {layer.isArtboard ? <MiniBadge label="artboard" tone="sky" /> : null}
                {layer.hasMask ? <MiniBadge label="mask" tone="fuchsia" /> : null}
                {layer.hasVectorMask ? <MiniBadge label="vmask" tone="emerald" /> : null}
              </div>
              <div className="mt-[2px] flex flex-wrap items-center gap-1 text-[10px] text-muted-foreground">
                <span>{formatBlendMode(layer.blendMode)}</span>
                <span>{Math.round(layer.opacity * 100)}%</span>
                {isGroup ? <span>{layer.isolated ? "Isolated" : "Pass-through"}</span> : null}
              </div>
            </div>

            <div className="flex items-center gap-[var(--ui-gap-1)]">
              <button
                type="button"
                className="flex h-5 min-w-6 items-center justify-center rounded-[var(--ui-radius-sm)] border border-border bg-black/18 px-1 text-[10px] text-muted-foreground transition hover:bg-black/30 focus-visible:outline-none"
                onClick={(event) => {
                  event.stopPropagation();
                  onCycleLock(layer.id, layer.lockMode);
                }}
                title="Cycle lock mode"
                aria-label="Cycle lock mode"
              >
                {shortLockLabel(layer.lockMode)}
              </button>
              <button
                type="button"
                className="flex h-5 min-w-6 cursor-grab items-center justify-center rounded-[var(--ui-radius-sm)] border border-border bg-black/18 px-1 text-[10px] text-muted-foreground transition hover:bg-black/30 active:cursor-grabbing focus-visible:outline-none"
                onClick={(event) => event.stopPropagation()}
                onDoubleClick={(event) => {
                  event.stopPropagation();
                  onDuplicate(layer.id);
                }}
                title="Drag to reorder, double-click to duplicate"
                aria-label="Drag to reorder, double-click to duplicate"
              >
                ::
              </button>
            </div>
          </div>
        </div>

        <div
          className={[
            "h-[2px] rounded-full transition",
            dropState === "after" ? "bg-accent/90" : "bg-transparent",
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
              maskEditLayerId={maskEditLayerId}
              selectedLayerIds={selectedLayerIds}
              collapsedGroups={collapsedGroups}
              draggedLayerId={draggedLayerId}
              dropTarget={dropTarget}
              editingLayerId={editingLayerId}
              editingName={editingName}
              thumbnails={thumbnails}
              colorTags={colorTags}
              onEditingNameChange={onEditingNameChange}
              onStartRename={onStartRename}
              onCommitRename={onCommitRename}
              onCancelRename={onCancelRename}
              onToggleGroup={onToggleGroup}
              onSelect={onSelect}
              onToggleVisibility={onToggleVisibility}
              onCycleLock={onCycleLock}
              onToggleMaskEdit={onToggleMaskEdit}
              onDuplicate={onDuplicate}
              onDragStart={onDragStart}
              onDragEnd={onDragEnd}
              onDragOver={onDragOver}
              onDropLayer={onDropLayer}
              onOpenContextMenu={onOpenContextMenu}
              onEnterVectorEdit={onEnterVectorEdit}
              onEnterTextEdit={onEnterTextEdit}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
});

function MiniBadge({
  label,
  tone,
}: {
  label: string;
  tone: "amber" | "sky" | "fuchsia" | "emerald";
}) {
  const toneClass =
    tone === "amber"
      ? "border-amber-400/25 bg-amber-400/10 text-amber-100"
      : tone === "sky"
        ? "border-sky-400/25 bg-sky-400/10 text-sky-100"
        : tone === "emerald"
          ? "border-emerald-400/25 bg-emerald-400/10 text-emerald-100"
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

function base64ToUint8ClampedArray(b64: string): Uint8ClampedArray {
  const binary = atob(b64);
  const bytes = new Uint8ClampedArray(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function ThumbnailCanvas({
  rgbaB64,
  size,
  className,
}: {
  rgbaB64: string;
  size: number;
  className?: string;
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !rgbaB64) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    const bytes = base64ToUint8ClampedArray(rgbaB64);
    const imageData = ctx.createImageData(size, size);
    imageData.data.set(bytes);
    ctx.putImageData(imageData, 0, 0);
  }, [rgbaB64, size]);
  return (
    <canvas
      ref={canvasRef}
      width={size}
      height={size}
      className={className}
      style={{ imageRendering: "pixelated" }}
    />
  );
}

function LayerThumbnail({
  layer,
  thumbnail,
  colorTag,
  isEditingMask,
  onToggleMaskEdit,
  onDoubleClick,
}: {
  layer: LayerNodeMeta;
  thumbnail: ThumbnailEntry | undefined;
  colorTag: ColorTagId;
  isEditingMask: boolean;
  onToggleMaskEdit: () => void;
  onDoubleClick?: () => void;
}) {
  const toneClass = layer.isArtboard
    ? "from-cyan-500/30 via-slate-800/70 to-slate-950"
    : layer.layerType === "group"
      ? "from-slate-500/60 via-slate-700/60 to-slate-950"
      : layer.layerType === "pixel"
        ? "from-cyan-500/25 via-slate-800/60 to-slate-950"
        : layer.layerType === "text"
          ? "from-amber-500/20 via-slate-800/60 to-slate-950"
          : layer.layerType === "vector"
            ? "from-emerald-500/20 via-slate-800/60 to-slate-950"
            : "from-fuchsia-500/20 via-slate-800/60 to-slate-950";

  const colorTagInfo = COLOR_TAGS.find((c) => c.id === colorTag);

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: role conditionally set to "button" when onDoubleClick is present
    <div
      className="flex items-center gap-0.5"
      onDoubleClick={onDoubleClick}
      role={onDoubleClick ? "button" : undefined}
      tabIndex={onDoubleClick ? 0 : undefined}
      onKeyDown={
        onDoubleClick
          ? (e) => {
              if (e.key === "Enter") onDoubleClick();
            }
          : undefined
      }
    >
      <div
        className={[
          "relative flex h-8 w-8 items-center justify-center overflow-hidden rounded-[var(--ui-radius-sm)] border border-border text-[9px] font-semibold uppercase tracking-[0.16em] text-foreground",
          layer.hasMask && !layer.maskEnabled ? "opacity-60" : "",
        ].join(" ")}
        title={`${layer.layerType} layer${layer.hasMask ? (layer.maskEnabled ? ", mask enabled" : ", mask disabled") : ""}`}
      >
        {thumbnail?.layerRGBA ? (
          <ThumbnailCanvas
            rgbaB64={thumbnail.layerRGBA}
            size={THUMBNAIL_SIZE}
            className="absolute inset-0 h-full w-full object-cover"
          />
        ) : (
          <>
            <div
              className={`absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] bg-gradient-to-br ${toneClass}`}
            />
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.18),transparent_60%)]" />
            <span className="relative z-10 text-[8px] uppercase tracking-[0.18em] text-foreground">
              {layer.isArtboard
                ? "ab"
                : layer.layerType === "group"
                  ? "grp"
                  : layer.layerType.slice(0, 2)}
            </span>
          </>
        )}
        {colorTag !== "none" && colorTagInfo ? (
          <span
            className={[
              "absolute left-0.5 top-0.5 h-2 w-2 rounded-full border",
              colorTagInfo.bg,
              colorTagInfo.border,
            ].join(" ")}
          />
        ) : null}
        {layer.clipToBelow ? (
          <span className="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-sky-300" />
        ) : null}
        {layer.hasVectorMask ? (
          <span className="absolute bottom-0.5 left-0.5 h-1.5 w-1.5 rounded-full bg-emerald-300" />
        ) : null}
      </div>

      {layer.hasMask ? (
        <button
          type="button"
          title={isEditingMask ? "Exit mask edit mode" : "Edit mask"}
          aria-label={isEditingMask ? "Exit mask edit mode" : "Edit mask"}
          className={[
            "relative flex h-8 w-8 items-center justify-center overflow-hidden rounded-[var(--ui-radius-sm)] border transition focus-visible:outline-none",
            isEditingMask
              ? "border-orange-400/60 bg-orange-400/8"
              : "border-border bg-black/18 hover:border-fuchsia-400/40",
          ].join(" ")}
          onClick={(event) => {
            event.stopPropagation();
            onToggleMaskEdit();
          }}
        >
          {thumbnail?.maskRGBA ? (
            <ThumbnailCanvas
              rgbaB64={thumbnail.maskRGBA}
              size={THUMBNAIL_SIZE}
              className="absolute inset-0 h-full w-full object-cover"
            />
          ) : (
            <span className="text-[8px] uppercase tracking-[0.18em] text-muted-foreground">m</span>
          )}
          {isEditingMask ? (
            <span className="absolute inset-0 rounded-[var(--ui-radius-sm)] ring-1 ring-orange-400/60" />
          ) : null}
        </button>
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
  onFlattenImage,
  onGroup,
  onUngroup,
  onAddMaskReveal,
  onAddMaskHide,
  onAddMaskFromSelection,
  onAddVectorMask,
  onDeleteVectorMask,
  onToggleClip,
  onLayerStyle,
  onCopyLayerStyle,
  onPasteLayerStyle,
  onClearLayerStyle,
  onLayerProperties,
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
  onFlattenImage: () => void;
  onGroup: () => void;
  onUngroup: () => void;
  onAddMaskReveal: () => void;
  onAddMaskHide: () => void;
  onAddMaskFromSelection: () => void;
  onAddVectorMask: () => void;
  onDeleteVectorMask: () => void;
  onToggleClip: () => void;
  onLayerStyle: () => void;
  onCopyLayerStyle: () => void;
  onPasteLayerStyle: () => void;
  onClearLayerStyle: () => void;
  onLayerProperties: () => void;
}) {
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const handlePointerDown = (event: globalThis.PointerEvent) => {
      // A pointerdown inside the menu must not close it, or the ensuing
      // click on the menu item never fires.
      if (menuRef.current?.contains(event.target as Node)) {
        return;
      }
      onClose();
    };
    const handleEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleEscape);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [onClose]);

  const runAction = (action: () => void) => () => {
    action();
    onClose();
  };

  return (
    <div
      ref={menuRef}
      role="menu"
      className="editor-popup fixed z-50 min-w-56 rounded-[var(--ui-radius-md)] p-1"
      style={{ left: x, top: y }}
      onContextMenu={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <MenuAction label="Duplicate Layer" onClick={runAction(onDuplicate)} />
      <MenuAction label="Delete Layer" onClick={runAction(onDelete)} destructive />
      <MenuSeparator />
      <MenuAction label="Merge Down" onClick={runAction(onMergeDown)} />
      <MenuAction label="Merge Visible" onClick={runAction(onMergeVisible)} />
      <MenuAction label="Flatten Image" onClick={runAction(onFlattenImage)} />
      <MenuSeparator />
      <MenuAction label="Group Layers" onClick={runAction(onGroup)} disabled={!canGroupSelection} />
      <MenuAction label="Ungroup" onClick={runAction(onUngroup)} disabled={!canUngroupSelection} />
      <MenuSeparator />
      <MenuAction label="Add Mask: Reveal All" onClick={runAction(onAddMaskReveal)} />
      <MenuAction label="Add Mask: Hide All" onClick={runAction(onAddMaskHide)} />
      <MenuAction label="Add Mask: From Selection" onClick={runAction(onAddMaskFromSelection)} />
      <MenuSeparator />
      <MenuAction label="Add Vector Mask" onClick={runAction(onAddVectorMask)} />
      <MenuAction label="Delete Vector Mask" onClick={runAction(onDeleteVectorMask)} />
      <MenuSeparator />
      <MenuAction label="Toggle Clipping" onClick={runAction(onToggleClip)} />
      <MenuSeparator />
      <MenuAction label="Layer Style..." onClick={runAction(onLayerStyle)} />
      <MenuAction label="Copy Layer Style" onClick={runAction(onCopyLayerStyle)} />
      <MenuAction label="Paste Layer Style" onClick={runAction(onPasteLayerStyle)} />
      <MenuAction label="Clear Layer Style" onClick={runAction(onClearLayerStyle)} />
      <MenuSeparator />
      <MenuAction label="Layer Properties..." onClick={runAction(onLayerProperties)} />
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
        "flex w-full items-center rounded-[var(--ui-radius-sm)] px-2.5 py-1.5 text-left text-[12px] transition focus-visible:outline-none",
        disabled
          ? "cursor-not-allowed text-muted-foreground/60"
          : destructive
            ? "text-rose-200 hover:bg-rose-500/12"
            : "text-foreground hover:bg-muted/50",
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
  return <div className="my-1 h-px bg-border" />;
}

function RangeField({
  label,
  value,
  disabled,
  onChange,
  commitProps,
}: {
  label: string;
  value: number;
  disabled: boolean;
  onChange: (value: number) => void;
  commitProps?: TransactionalSliderCommitProps;
}) {
  // Cleared/invalid input parses to the current value; skip the change so a
  // no-op transaction (phantom undo entry) is never opened.
  const handleChange = (raw: string) => {
    const parsed = parseNumericInput(raw, value);
    if (parsed !== value) {
      onChange(parsed);
    }
  };
  return (
    <label className="block">
      <div className="mb-1 flex items-center justify-between text-[11px] uppercase tracking-[0.18em] text-muted-foreground/70">
        <span>{label}</span>
        <span className="text-muted-foreground">{value}</span>
      </div>
      <div className="grid grid-cols-[1fr_44px] items-center gap-[var(--ui-gap-2)]">
        <input
          className="h-2 w-full accent-accent disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none"
          type="range"
          min="0"
          max="100"
          value={value}
          disabled={disabled}
          onChange={(event) => handleChange(event.target.value)}
          {...commitProps}
        />
        <input
          className="h-[var(--ui-h-sm)] rounded-[var(--ui-radius-md)] border border-border bg-panel-soft px-1.5 text-right text-[12px] text-foreground disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none"
          type="number"
          min="0"
          max="100"
          value={value}
          disabled={disabled}
          onChange={(event) => handleChange(event.target.value)}
          {...commitProps}
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

export function LayerPropertiesDialog({
  layer,
  colorTag,
  onRename,
  onColorTag,
  onOpenLayerStyle,
  onClose,
}: {
  layer: LayerNodeMeta | null;
  colorTag: ColorTagId;
  onRename: (name: string) => void;
  onColorTag: (tag: ColorTagId) => void;
  onOpenLayerStyle: () => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(layer?.name ?? "");

  useEffect(() => {
    setName(layer?.name ?? "");
  }, [layer]);

  if (!layer) {
    return null;
  }

  const handleApply = () => {
    const trimmed = name.trim();
    if (trimmed && trimmed !== layer.name) {
      onRename(trimmed);
    }
    onClose();
  };

  return (
    <div
      className="editor-backdrop fixed inset-0 z-50 flex items-center justify-center"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose();
        }
      }}
    >
      <div className="editor-popup w-72 rounded-[var(--ui-radius-md)] p-4">
        <h2 className="mb-3 text-[13px] font-semibold text-foreground">Layer Properties</h2>

        <div className="mb-3">
          <label className="block text-[11px] uppercase tracking-[0.18em] text-muted-foreground/70">
            Name
            <input
              className="mt-1 h-[var(--ui-h-sm)] w-full rounded-[var(--ui-radius-md)] border border-border bg-black/25 px-2 text-[12px] text-foreground outline-none focus:border-accent/40 focus-visible:ring-1 focus-visible:ring-ring/30"
              value={name}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  handleApply();
                }
                if (event.key === "Escape") {
                  onClose();
                }
              }}
              // biome-ignore lint/a11y/noAutofocus: dialog input intentionally focused on open
              autoFocus
            />
          </label>
        </div>

        <div className="mb-4">
          <div className="mb-2 text-[11px] uppercase tracking-[0.18em] text-muted-foreground/70">
            Color Label
          </div>
          <div className="flex gap-1.5">
            {COLOR_TAGS.map((tag) => (
              <button
                key={tag.id}
                type="button"
                title={tag.label}
                aria-label={tag.label}
                className={[
                  "h-5 w-5 rounded-full border-2 transition hover:scale-110",
                  tag.id === "none"
                    ? "border-border bg-transparent hover:border-muted-foreground"
                    : `${tag.bg} ${tag.border}`,
                  colorTag === tag.id
                    ? "ring-2 ring-ring/60 ring-offset-1 ring-offset-panel-soft"
                    : "",
                ].join(" ")}
                onClick={() => onColorTag(tag.id)}
              >
                {tag.id === "none" ? (
                  <span className="flex h-full w-full items-center justify-center text-[8px] text-muted-foreground">
                    ×
                  </span>
                ) : null}
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center justify-between gap-2">
          <Button variant="ghost" size="sm" onClick={onOpenLayerStyle}>
            Layer Style...
          </Button>
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={onClose}>
              Cancel
            </Button>
            <Button size="sm" onClick={handleApply}>
              Apply
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
