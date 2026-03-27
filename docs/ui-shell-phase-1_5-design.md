# UI Shell Design: Dense Photoshop/Photopea-Inspired Workspace

## Purpose

Define the target desktop editor shell for Agogo-Web between the current rounded prototype and a denser Photoshop/Photopea-like workspace. This document focuses on shell, docking, panel chrome, spacing, and interaction density. Pixel rendering remains owned by the Go/Wasm engine.

This is a UI density and information architecture pass, not a feature-expansion pass.

## What The Reference UIs Show

Both screenshots present the same core model:

- A dark, low-chroma professional workspace with very little empty space.
- A top application bar with product title and compact text menus.
- A second compact horizontal strip for tool or document controls.
- A narrow left vertical toolbar with tightly stacked icon buttons.
- A large central canvas stage surrounded by a darker work area.
- A right dock made of stacked utility panels such as Layers, Properties, and Adjustments.
- Dense panel chrome: small headers, thin separators, compact rows, minimal card styling.
- Panels behave like software furniture, not standalone marketing cards.
- Almost no large radii, very limited outer padding, and minimal spacing between adjacent regions.

The key takeaway is density:

- Outer shell margins are small.
- Panel internal padding is small.
- Toolbar buttons are compact.
- Tab headers are short and low-height.
- The canvas gets the majority of the screen.

## Target Design Direction For Agogo-Web

Agogo-Web should move from a soft, spacious, rounded dashboard look to a dense editor workspace.

Target qualities:

- Desktop-app feel, not web-app card layout.
- Flush, contiguous regions instead of separated floating panels.
- Compact controls with strong hierarchy.
- Clear contrast between app chrome, panel chrome, and canvas stage.
- Small radii only where needed.
- Minimal margin and padding by default.

Avoid:

- Oversized rounded containers.
- Large gaps between toolbar, canvas, and dock.
- Hero-style headers.
- Excessive gradients, blur, or elevated-card shadows.
- Large pill buttons for primary shell actions.

## Layout Specification

### 1. Global Frame

Use a full-height application frame with near-edge-to-edge content.

- App background: neutral charcoal gradient or flat dark gray.
- Outer page padding: `4px` to `8px`.
- Gap between major shell regions: `1px` to `6px`.
- Region borders should do most of the separation work.

Recommended shell rows:

1. Title/menu bar
2. Context/options bar
3. Main workspace row
4. Status bar

### 2. Title/Menu Bar

Purpose: app identity, menu access, document title, lightweight global controls.

Characteristics:

- Height: `32px` to `38px`
- Compact typography
- Flat dark chrome
- Horizontal text menus with small gaps

Contents:

- Product mark and name
- Top-level menus: `File`, `Edit`, `Image`, `Layer`, `Select`, `Filter`, `View`, `Window`, `Help`
- Optional lightweight document title near the left-center
- Utility icons or window controls on the far right

Rules:

- No oversized logo treatment.
- No large button group in this row.
- Menu items should read like desktop menus, not pill buttons.

### 3. Options Bar

Purpose: show controls for the active tool or active context.

Characteristics:

- Height: `34px` to `42px`
- Slightly different background from title bar
- Dense inline controls

Examples:

- Tool mode toggles
- Numeric fields
- Blend mode dropdown
- Opacity field
- Checkboxes
- Tool hints

Rules:

- Controls should be inline and compact.
- Empty states should still preserve the row height.
- This row should not look like a card.

### 4. Left Toolbar

Purpose: persistent primary tool selection.

Characteristics:

- Width: `40px` to `52px`
- Vertical stack of compact icon buttons
- Mostly monochrome inactive icons
- Strong active state

Rules:

- Buttons should be close together: `2px` to `4px` gap.
- Button size: about `28px` to `34px`.
- Labels should not be permanently visible.
- Tooltips can provide names and shortcuts.
- Reserve space for future grouped tools.

### 5. Canvas Stage

Purpose: maximize document focus while preserving workspace context.

Characteristics:

- Largest region in the app
- Dark stage around the document
- Document centered with visible work area around it
- Thin frame or shadow around the document surface

Structure:

- Stage background darker than panels
- Inner stage for centering canvas
- Optional rulers at top and left in a later pass

Rules:

- Canvas container should not read as a rounded card.
- The stage should feel embedded in the workspace.
- The document area can retain subtle separation from the stage.

### 6. Right Dock

Purpose: house Layers, Properties, History, Navigator, Adjustments, and future panels.

Characteristics:

- Width: `280px` to `360px` initially
- Built from stacked dock sections
- Panels can be tabbed or vertically stacked
- Headers are compact and low-height

Recommended initial stack:

1. Properties / Adjustments area
2. Layers panel

Optional near-term additions:

- History
- Navigator

Rules:

- Prefer stacked dock sections over a single large rounded container.
- Panels should share borders and feel snapped together.
- Resizing should happen at dock boundaries, not via large draggable affordances.

### 7. Status Bar

Purpose: always-visible secondary document information.

Characteristics:

- Height: `24px` to `30px`
- Very compact
- Low visual weight

Contents:

- Zoom
- Document size
- Cursor coordinates
- Engine/runtime status
- Optional color/profile info later

Rules:

- No large padding.
- Information should be segmented lightly, not boxed heavily.

## Density And Spacing System

This project should explicitly adopt dense shell spacing tokens.

### Density tokens

- `--ui-gap-1: 2px`
- `--ui-gap-2: 4px`
- `--ui-gap-3: 6px`
- `--ui-gap-4: 8px`
- `--ui-gap-5: 10px`
- `--ui-gap-6: 12px`

### Control heights

- `--ui-h-xs: 24px`
- `--ui-h-sm: 28px`
- `--ui-h-md: 32px`
- `--ui-h-lg: 36px`

### Radii

- `--ui-radius-sm: 3px`
- `--ui-radius-md: 5px`
- `--ui-radius-lg: 8px`

Use `sm` and `md` as defaults. Large radii should be rare in shell chrome.

### Border strategy

- Prefer `1px` borders and separators over large empty gaps.
- Use subtle value changes to distinguish rows and dock sections.
- Avoid thick outlines except for active or focused controls.

## Color And Surface Rules

Move toward a flatter neutral-dark palette.

Recommended surface ladder:

- App background: darkest base
- Menu/toolbar chrome: slightly raised
- Panel chrome: slightly raised from menu bar
- Panel content: nearly same as panel chrome, separated by borders
- Canvas stage: darker and quieter than dock panels

Guidance:

- Use cyan or blue sparingly for active state and focus.
- Avoid heavy glassmorphism.
- Avoid large soft shadows on every panel.
- Keep highlight contrast subtle and professional.

## Typography

The screenshots use small, efficient typography. Agogo-Web should do the same.

Targets:

- Menus and panel tabs: `12px` to `13px`
- Panel body rows: `12px` to `13px`
- Secondary metadata: `11px` to `12px`
- Status text: `11px` to `12px`

Rules:

- Prefer one compact UI font stack for shell consistency.
- Use uppercase sparingly; mostly for tiny metadata or panel labels.
- Remove oversized marketing-style headings from workspace chrome.

## Panel Chrome And Behavior

### Panel headers

Each dock panel should support:

- Title
- Optional tabs
- Small action icons
- Collapse affordance later if needed

Header guidelines:

- Height: `26px` to `32px`
- Tight horizontal padding: `6px` to `10px`
- Bottom border separator

### Panel body

Guidelines:

- Typical panel padding: `6px` to `10px`
- Dense row gaps: `2px` to `6px`
- Scrolling should happen inside panel bodies

### Tabs

Tabs should behave like software tabs:

- Compact
- Adjacent
- Low-height
- Clear active state

Avoid:

- Large rounded pill tabs
- Oversized inactive padding

## Layers Panel Specific Guidance

This panel is the clearest anchor for the reference look and should be treated as a high-priority shell refinement.

Target row anatomy:

- Visibility icon
- Layer thumbnail area
- Optional mask thumbnail
- Layer name
- Optional badges/icons on the right

Row guidelines:

- Height: `28px` to `36px`
- Horizontal padding: `4px` to `8px`
- Tiny gaps between sub-elements
- Active row should be obvious but not neon-heavy

Toolbar guidelines:

- Small icon actions at the bottom or top of panel
- Minimal border separation
- No large textual buttons for common layer actions in the final dense UI

Near-term implementation note:

- The current large action buttons can remain temporarily, but the design target is an icon-first strip.

## Properties / Adjustments Guidance

The reference UIs show that control-heavy panels should be dense and vertically efficient.

Guidelines:

- Stack compact labeled controls
- Use small section headers
- Prefer dropdown + slider + numeric field patterns
- Keep charts/wheels visually contained inside panel boundaries

Implementation note:

- Initial Agogo-Web Phase 1.5 only needs the shell and layout conventions, not full Photoshop-grade controls.

## Interaction Model

### Resizing

- Right dock remains horizontally resizable.
- Resize handle should be thin and understated.
- Left toolbar remains fixed-width.
- Canvas stage absorbs most free space.

### Responsiveness

This shell is desktop-first.

Rules:

- Preserve dense layout on widths `>= 1280px`.
- For narrower widths, collapse secondary dock content before increasing padding.
- Do not switch to spacious mobile-card patterns.

### Focus states

- Keyboard focus must remain clear on menus, tabs, layer rows, and toolbar buttons.
- Focus ring can use accent color but should remain thin.

## Architecture Mapping To This Repo

### Frontend areas likely affected

- [apps/editor-web/src/App.tsx](/mnt/projekte/Code/Agogo-Web/apps/editor-web/src/App.tsx)
- [apps/editor-web/src/styles.css](/mnt/projekte/Code/Agogo-Web/apps/editor-web/src/styles.css)
- [apps/editor-web/src/components/layers-panel.tsx](/mnt/projekte/Code/Agogo-Web/apps/editor-web/src/components/layers-panel.tsx)
- [apps/editor-web/src/components/editor-canvas.tsx](/mnt/projekte/Code/Agogo-Web/apps/editor-web/src/components/editor-canvas.tsx)
- shared UI primitives under [apps/editor-web/src/components/ui](/mnt/projekte/Code/Agogo-Web/apps/editor-web/src/components/ui)

### Required implementation shift

The current shell uses:

- large rounded surfaces
- large gaps
- card-like segmentation
- prominent shadows and gradients

Phase 1.5 should refactor that into:

- dense app-frame layout
- shared chrome tokens
- smaller radii
- smaller control heights
- contiguous dock sections
- compact panel tabs and rows

### Non-goals for Phase 1.5

- No JS-side pixel processing
- No engine rendering ownership changes
- No full menu command system yet
- No final iconography pass required to unblock the shell
- No Photoshop-complete properties widgets yet

## Acceptance Criteria For Phase 1.5

- Workspace reads as a dense desktop editor instead of a card-based dashboard.
- Global outer padding is visually minimal.
- Top bars are compact and contiguous.
- Left toolbar is narrow and icon-dense.
- Right dock feels like stacked software panels, not a single padded card.
- Layers panel rows become materially denser.
- Canvas stage visually dominates the screen.
- The current engine integration, canvas blitting model, and existing Phase 1 features still work.

## Implementation Notes

Recommended order:

1. Introduce density tokens and flatter surface tokens in CSS.
2. Refactor `App.tsx` shell layout into contiguous bars + dock.
3. Tighten toolbar, tabs, panel headers, and status bar.
4. Densify the Layers panel row structure and action strip.
5. Verify the canvas remains the primary focus region at common desktop widths.

## Summary

The references are not mainly about extra features. They are about density, hierarchy, and professional editor chrome. Agogo-Web should adopt a compact, low-padding, dock-based shell where the canvas is dominant and utility panels are information-dense. Phase 1.5 should deliver that visual and structural shift before later feature phases continue building on top of the current UI.
