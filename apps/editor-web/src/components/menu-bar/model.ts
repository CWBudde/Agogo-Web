export type MenuPreviewTone = "default" | "accent" | "muted";

export type MenuActionId =
  | "new-document"
  | "open-project"
  | "open-recent"
  | "save-project"
  | "save-psd"
  | "save-psb"
  | "export-project"
  | "generate-assets"
  | "canvas-size"
  | "transform-free"
  | "transform-scale"
  | "transform-rotate"
  | "transform-skew"
  | "transform-distort"
  | "transform-perspective"
  | "transform-warp"
  | "transform-flip-h"
  | "transform-flip-v"
  | "transform-rotate-cw"
  | "transform-rotate-ccw"
  | "transform-rotate-180"
  | "edit-fill"
  | "image-invert"
  | "image-channel-mixer"
  | "image-threshold"
  | "image-posterize"
  | "image-selective-color"
  | "image-photo-filter"
  | "image-gradient-map"
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
  | "view-toggle-guides";

export type MenuPreviewItem = {
  label: string;
  shortcut?: string;
  tone?: MenuPreviewTone;
  actionId?: MenuActionId;
  disabled?: boolean;
  checked?: boolean;
};

export type MenuPreviewMenu = {
  label: string;
  caption: string;
  align?: "left" | "right";
  sections: { title: string; items: MenuPreviewItem[] }[];
};

export const menuItems: MenuPreviewMenu[] = [
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
          { label: "Save Archive", shortcut: "Ctrl+S", actionId: "save-project" },
          { label: "Save as PSD", actionId: "save-psd" },
          { label: "Save as PSB", actionId: "save-psb" },
          {
            label: "Export As...",
            shortcut: "Ctrl+Shift+E",
            actionId: "export-project",
          },
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
          { label: "Scale", actionId: "transform-scale" as const },
          { label: "Rotate", actionId: "transform-rotate" as const },
          { label: "Skew", actionId: "transform-skew" as const },
          { label: "Distort", actionId: "transform-distort" as const },
          { label: "Perspective", actionId: "transform-perspective" as const },
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
    caption: "Canvas-wide operations and color management preview.",
    sections: [
      {
        title: "Adjustments",
        items: [
          { label: "Levels..." },
          { label: "Curves..." },
          { label: "Hue/Saturation..." },
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
          { label: "Image Size..." },
          { label: "Canvas Size...", shortcut: "Ctrl+Alt+C", actionId: "canvas-size" as const },
          { label: "Trim" },
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
        items: [
          { label: "Pixel Grid" },
          { label: "Rulers" },
          { label: "Guides", actionId: "view-toggle-guides" },
        ],
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
