import { filtersByCategory } from "@/components/filters/filter-catalog";

export type MenuPreviewTone = "default" | "accent" | "muted";

export type MenuActionId =
  | "new-document"
  | "open-project"
  | "open-recent"
  | "save-project"
  | "save-psd"
  | "save-psb"
  | "export-project"
  | "canvas-size"
  | "edit-undo"
  | "edit-redo"
  | "transform-free"
  | "transform-warp"
  | "transform-flip-h"
  | "transform-flip-v"
  | "transform-rotate-cw"
  | "transform-rotate-ccw"
  | "transform-rotate-180"
  | "edit-fill"
  | "image-levels"
  | "image-curves"
  | "image-hue-sat"
  | "image-invert"
  | "image-channel-mixer"
  | "image-threshold"
  | "image-posterize"
  | "image-selective-color"
  | "image-photo-filter"
  | "image-gradient-map"
  | "layer-new"
  | "layer-new-group"
  | "layer-add-mask"
  | "layer-duplicate"
  | "layer-merge-down"
  | "layer-rasterize"
  | "select-all"
  | "select-deselect"
  | "select-reselect"
  | "select-invert"
  | "select-feather"
  | "select-expand"
  | "select-contract"
  | "select-smooth"
  | "select-border"
  | "select-transform"
  | "select-color-range"
  | "select-save-channel"
  | "select-load-channel"
  | "select-and-mask"
  | "view-zoom-in"
  | "view-zoom-out"
  | "view-fit-screen"
  | "view-toggle-guides"
  | "window-layers"
  | "window-navigator"
  | "window-history"
  | "filter-last"
  | "filter-fade";

export type MenuPreviewItem = {
  label: string;
  shortcut?: string;
  tone?: MenuPreviewTone;
  actionId?: MenuActionId;
  /** Registered filter id — filter items are routed to the filter runner. */
  filterId?: string;
  disabled?: boolean;
  checked?: boolean;
};

export type MenuPreviewMenu = {
  label: string;
  caption: string;
  align?: "left" | "right";
  sections: { title: string; items: MenuPreviewItem[] }[];
};

/**
 * The Filter menu is generated from the shared filter catalog: a Recent section
 * (Last Filter / Fade) followed by one section per non-empty filter category.
 * Dialog filters get a "..." suffix; parameterless filters apply on click.
 */
function buildFilterMenu(): MenuPreviewMenu {
  const categorySections = filtersByCategory().map((group) => ({
    title: group.label,
    items: group.filters.map((filter) => ({
      label: filter.hasDialog ? `${filter.name}...` : filter.name,
      filterId: filter.id,
    })),
  }));

  return {
    label: "Filter",
    caption: "Destructive filters with live preview, reapply, and fade.",
    sections: [
      {
        title: "Recent",
        items: [
          { label: "Last Filter", shortcut: "Ctrl+F", actionId: "filter-last" as const },
          { label: "Fade...", tone: "muted", actionId: "filter-fade" as const },
        ],
      },
      ...categorySections,
    ],
  };
}

export const menuItems: MenuPreviewMenu[] = [
  {
    label: "File",
    caption: "Create, open, save, and export documents.",
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
          { label: "Save Archive", shortcut: "Ctrl+S", actionId: "save-project" },
          { label: "Save as PSD", actionId: "save-psd" },
          { label: "Save as PSB", actionId: "save-psb" },
          {
            label: "Export As...",
            shortcut: "Ctrl+Shift+E",
            actionId: "export-project",
          },
        ],
      },
    ],
  },
  {
    label: "Edit",
    caption: "History, fill, and layer transformations.",
    sections: [
      {
        title: "History",
        items: [
          {
            label: "Undo",
            shortcut: "Ctrl+Z",
            tone: "accent",
            actionId: "edit-undo",
          },
          { label: "Redo", shortcut: "Ctrl+Shift+Z", actionId: "edit-redo" },
        ],
      },
      {
        title: "Fill",
        items: [
          {
            label: "Fill...",
            shortcut: "Shift+F5",
            actionId: "edit-fill" as const,
            tone: "accent",
          },
        ],
      },
      {
        title: "Transform",
        items: [
          {
            label: "Free Transform",
            shortcut: "Ctrl+T",
            tone: "accent",
            actionId: "transform-free" as const,
          },
          { label: "Warp", actionId: "transform-warp" as const },
          { label: "Flip Horizontal", actionId: "transform-flip-h" as const },
          { label: "Flip Vertical", actionId: "transform-flip-v" as const },
          { label: "Rotate 90° CW", actionId: "transform-rotate-cw" as const },
          { label: "Rotate 90° CCW", actionId: "transform-rotate-ccw" as const },
          { label: "Rotate 180°", actionId: "transform-rotate-180" as const },
        ],
      },
    ],
  },
  {
    label: "Image",
    caption: "Adjustment layers and canvas geometry.",
    sections: [
      {
        title: "Adjustments",
        items: [
          { label: "Levels", actionId: "image-levels" as const },
          { label: "Curves", actionId: "image-curves" as const },
          { label: "Hue/Saturation", actionId: "image-hue-sat" as const },
          { label: "Invert", actionId: "image-invert" as const },
          { label: "Channel Mixer...", actionId: "image-channel-mixer" as const },
          { label: "Threshold...", actionId: "image-threshold" as const },
          { label: "Posterize...", actionId: "image-posterize" as const },
          { label: "Selective Color...", actionId: "image-selective-color" as const },
          { label: "Photo Filter...", actionId: "image-photo-filter" as const },
          { label: "Gradient Map...", actionId: "image-gradient-map" as const },
        ],
      },
      {
        title: "Geometry",
        items: [
          { label: "Canvas Size...", shortcut: "Ctrl+Alt+C", actionId: "canvas-size" as const },
        ],
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
          {
            label: "New Layer",
            shortcut: "Ctrl+Shift+N",
            tone: "accent",
            actionId: "layer-new",
          },
          { label: "New Group", actionId: "layer-new-group" },
          { label: "Layer Mask", actionId: "layer-add-mask" },
        ],
      },
      {
        title: "Arrange",
        items: [
          { label: "Duplicate Layer", shortcut: "Ctrl+J", actionId: "layer-duplicate" },
          { label: "Merge Down", shortcut: "Ctrl+E", actionId: "layer-merge-down" },
          { label: "Rasterize", actionId: "layer-rasterize" },
        ],
      },
    ],
  },
  {
    label: "Select",
    caption: "Selection workflows and edge refinement.",
    sections: [
      {
        title: "Global",
        items: [
          { label: "All", shortcut: "Ctrl+A", actionId: "select-all" as const },
          { label: "Deselect", shortcut: "Ctrl+D", actionId: "select-deselect" as const },
          { label: "Reselect", shortcut: "Ctrl+Shift+D", actionId: "select-reselect" as const },
          { label: "Inverse", shortcut: "Ctrl+Shift+I", actionId: "select-invert" as const },
        ],
      },
      {
        title: "Modify",
        items: [
          { label: "Feather...", actionId: "select-feather" as const },
          { label: "Expand...", actionId: "select-expand" as const },
          { label: "Contract...", actionId: "select-contract" as const },
          { label: "Smooth...", actionId: "select-smooth" as const },
          { label: "Border...", actionId: "select-border" as const },
          { label: "Transform Selection", actionId: "select-transform" as const },
          { label: "Color Range...", actionId: "select-color-range" as const },
          { label: "Save Selection...", actionId: "select-save-channel" as const },
          { label: "Load Selection...", actionId: "select-load-channel" as const },
        ],
      },
      {
        title: "Refine",
        items: [{ label: "Select and Mask", actionId: "select-and-mask" as const }],
      },
    ],
  },
  buildFilterMenu(),
  {
    label: "View",
    caption: "Viewport controls that mirror the current chrome.",
    sections: [
      {
        title: "Zoom",
        items: [
          {
            label: "Zoom In",
            shortcut: "Ctrl++",
            tone: "accent",
            actionId: "view-zoom-in",
          },
          { label: "Zoom Out", shortcut: "Ctrl+-", actionId: "view-zoom-out" },
          { label: "Fit on Screen", shortcut: "Ctrl+0", actionId: "view-fit-screen" },
        ],
      },
      {
        title: "Overlays",
        items: [{ label: "Guides", actionId: "view-toggle-guides" }],
      },
    ],
  },
  {
    label: "Window",
    caption: "Open editor panels.",
    align: "right",
    sections: [
      {
        title: "Panels",
        items: [
          { label: "Layers", tone: "accent", actionId: "window-layers" },
          { label: "Navigator", actionId: "window-navigator" },
          { label: "History", actionId: "window-history" },
        ],
      },
    ],
  },
];
