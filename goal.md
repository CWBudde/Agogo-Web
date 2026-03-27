# Implementierungsplan für einen Photoshop/Photopea-Clone mit Go+AGG in Wasm und React+Vite+shadcn/Base UI

## Zielbild und Architekturprinzipien

Du baust einen **komplett im Browser laufenden** Bildeditor nach dem Vorbild von Photoshop/Photopea. Photopea selbst positioniert sich als „Online Photo Editor“ mit **Layering, Masking, Blending** und **voller PSD-Unterstützung (Öffnen & Speichern)** – genau diese „Desktop-Editor“-Erwartungshaltung wird dir deine Zielgruppe entgegenbringen. citeturn3search0turn3search1

**Kernprinzip (von dir vorgegeben):**  
- **Backend = Go (dein privater AGG-Port) als WebAssembly-Modul** im Browser.  
- **Frontend = Vite + React + TypeScript**, UI-Komponenten über **shadcn** + **Tailwind CSS v4** + „Base UI“-Primitives (unstyled/headless).  
- **Wichtig:** *Kein* Zeichnen/Processing in JS/Canvas außer **Blitting** (Pixelbuffer anzeigen). Der Canvas ist „nur ein Display“.

**Warum das architektonisch sauber ist:**  
AGG ist als Render-Engine gedacht, die **Pixelbilder im Speicher** aus Vektordaten erzeugt und **API-unabhängig** ist (kein Zwang zu einer konkreten Grafik-API). Das passt perfekt zu „Wasm-Backend rendert RGBA in Memory“ + „Frontend zeigt nur an“. citeturn2search14turn1search0

**Licensing/Legal-Hinweis (früh klären, sonst lebensgefährlich für das Projekt):**  
AGG existiert historisch in Varianten mit **GPL** (z. B. 2.5) und Varianten mit permissiveren Lizenzen (Forks rund um 2.4/2.6). Außerdem enthält das klassische AGG-Distribution-Paket Komponenten wie **GPC (General Polygon Clipper)**, die laut Lizenztext **nur für nicht-kommerzielle Software kostenlos** sind – das musst du entweder ersetzen oder konsequent entfernen. citeturn2search3turn2search25turn2search29  
→ In deinem Plan sollte es eine **Lizenz-Audit-Phase** geben, bevor du zu tief implementierst.

**Performance-Grundannahme:**  
Ein Editor ist interaktiv. „Alles in Go/Wasm“ ist machbar, aber du brauchst eine klare Strategie, um **UI-Thread-Blockaden** zu vermeiden. Die robuste Zielarchitektur ist meist: **Wasm in einem Web Worker** + Pixel-/State-Streaming zur UI. (Dazu unten: SharedArrayBuffer/COOP/COEP als spätere Optimierung.) citeturn2search0turn2search12

## Funktionsumfang: Menüs, Panels und Tools

Dieses Kapitel ist eine **vollständige Produktspezifikation** dessen, was dein Clone „können soll“. Es ist absichtlich „Photoshop-nah“, aber pragmatisch in Subsysteme zerlegt, damit du es phasenweise implementieren kannst.

### UI-Grundlayout

Orientierung am klassischen Workspace: **Top-Menü**, **Tool-Bar links**, **Options-Bar oben (kontextabhängig je Tool)**, **Canvas/Arbeitsfläche in der Mitte**, **Panels rechts** (Layers, History, Properties …). Photopea beschreibt exakt diese Aufteilung (Toolbar, Sidebar, Working area, Top menu). citeturn3search1

### Menüspezifikation

Du implementierst diese Menüs (deutsch), jeweils mit einer „Command-ID“ (intern), Shortcut-Default, und „Backend-Contract“ (welche Engine-Funktion aufgerufen wird). Für Shortcut-Konzept/Customizing ist es sinnvoll, Photoshop-ähnlich eine vollständige Shortcut-Liste exportierbar zu machen (Photoshop erlaubt Zugriff/Ansicht der kompletten Shortcut-Liste und Customizing über „Edit > Keyboard Shortcuts“). citeturn4search1

#### Datei

**Neu / Öffnen / Speichern**
- Neu… (Dokumentdialog: Presets, Breite/Höhe, DPI/PPU, Farbmodus, Bit-Tiefe, Hintergrund: Transparent/Weiß/Farbe)
- Öffnen… (File Picker, Drag&Drop, „Zuletzt geöffnet“, „Beispiele“)
- Öffnen als… (Format erzwingen)
- Platzieren / Einbetten… (Import als Smart-Objekt-Äquivalent: „eingebettetes Objekt“)
- Schließen / Alle schließen
- Speichern
- Speichern unter…
- Version speichern (Snapshot/Revision)
- Exportieren (siehe unten)
- Drucken… (optional: Browser-Print nur als Export-PDF/PNG vorbereiten)
- Dokument-Informationen/Metadaten…
- Einstellungen/Preferences… (global)
- Beenden (Web: „Tab schließen“-Hinweis; stattdessen „Projekt schließen“)

**Import/Export**
- Import: PNG/JPG/WebP/TIFF/BMP/GIF (Basis), PSD/PSB (Ziel), SVG/PDF (Vektor-/Import-Pipeline), RAW (spät, optional wie Photopea „RAW support“). citeturn3search0turn1search1  
- Export „Quick“: PNG (transparent), JPG (Qualität), WebP (Lossy/Lossless), TIFF (optional), PDF (print), PSD/PSB (Projektformat).
- Export „Assets“ (Slices/Artboards):  
  - Exportieren als… (z. B. mehrere Größen, Formate, Naming-Schema)  
  - Exportieren: Slices → Dateien (Photopea: Slices definieren Rechtecke und exportieren mehrere Dateien). citeturn7search20
  - Exportieren: Artboards → Dateien/ZIP (Photopea: Artboards als „documents inside a document“). citeturn7search0

**Automatisieren**
- Aktionen… (Record/Play, Batch) (Photopea hat Actions/Panel: Window → Actions). citeturn7search28turn7search24  
- Variablen… (Data Sets → Export) (Photopea: Variables/Export-as Pipeline). citeturn7search8turn7search24  
- Skripte… (Photoshop-like Scripting Interface; Photopea: kompatible Script-Idee, „global object app“ etc.). citeturn7search16  
- Stapelverarbeitung… (Batch: Ordner auswählen → Actions/Steps anwenden → Export)

#### Bearbeiten

**Undo/Redo & History**
- Rückgängig / Wiederholen (mehrstufig), Schritt zurück/vor (History)
- Letzten Filter verblassen… (Fade) (als Mischmodus/Opacity-Blend auf „Last Operation Result“)
- Verlauf löschen / Verlaufspalette bereinigen

**Clipboard / Transfer**
- Ausschneiden / Kopieren / Einfügen  
- In Place einfügen  
- Einfügen als: Neue Ebene / Neue Dokument / Smart-Objekt / Maske  
- Copy Merged (sichtbare Komposition kopieren)
- Special Paste: „Paste into Selection“, „Paste as Layer Mask“

**Transform / Geometrie**
- Frei Transformieren (Bounding Box + Handles)
- Skalieren, Drehen, Neigen, Verzerren, Perspektivisch, Warp
- Transformieren wiederholen
- Inhaltssensitives Skalieren (spät/optional)

**Fill/Stroke**
- Fläche füllen… (Farbe/Pattern/Content-Aware optional)
- Kontur… (Stroke) — Hinweis: Photoshop beschreibt „Clear mode“ u. a. im Zusammenhang mit Fill/Stroke/Brush. citeturn3search2  

**Preferences**
- Voreinstellungen…  
  - UI (Theme, Density, Ruler Units)  
  - Performance/Cache  
  - GPU (bei dir: „kein GPU“, aber Worker-/Thread-Optionen)  
  - File Handling (Auto-Save, Recovery)  
  - Color Management (später)

#### Bild

**Geometrie**
- Bildgröße… (Resample-Methoden)
- Arbeitsfläche… (Canvas Size, Anchor)
- Zuschneiden (Crop auf Selection/Layer)
- Zurechtschneiden/Trim (transparent edges, top-left color)

**Modus / Color**
- Modus: RGB, Graustufen, Indexed (spät), CMYK/Lab (spät)  
- Bit-Tiefe: 8/16/32 (spät); PSD/PSB unterstützen laut Spezifikation große Dokumente und viele Features, inkl. Layers/Effects/Filters in PSB. citeturn1search1turn1search13
- Farbprofil zuweisen / konvertieren (ICC, sehr spät)

**Korrekturen (destruktiv, als Alternative zu Adjustment Layers)**
- Auto-Tone/Auto-Contrast/Auto-Color
- Tonwerte (Levels)… (Adobe: Levels Adjustment erklärt Tonwertkorrektur). citeturn3search18  
- Gradationskurven (Curves)… citeturn3search14  
- Hue/Saturation… (auch als Adjustment Layer) citeturn3search3  
- Color Balance, Exposure, Vibrance, Black & White, Channel Mixer, Selective Color, Gradient Map, Invert, Threshold, Posterize

**Variablen/Histogram**
- Histogramm Panel (View-only)
- Messwerte/Info (Pixel unter Cursor)

#### Ebene

**Layer Lifecycle**
- Neu:  
  - Leere Ebene  
  - Ebene aus Hintergrund (wenn Hintergrund-Semantik existiert)  
  - Duplizieren  
  - Löschen  
- Ebenen-Reihenfolge: nach vorn/hinten, nach oben/unten
- Ebenenarten:  
  - Pixel-Layer  
  - Text-Layer  
  - Shape-/Vector-Layer  
  - Group/Folder  
  - Adjustment Layer (nicht-destruktiv; Adobe betont non-destructive edits über Adjustment Layers). citeturn3search34turn3search22  
  - Smart Object / Embedded Object (spät)

**Blending / Opacity**
- Blend Mode Dropdown + Opacity + Fill  
  - Vollständige Blend-Mode-Implementierung: Adobe listet/erklärt Blend Modes als zentrale Ebene-Funktion. citeturn3search2turn3search33

**Masken**
- Ebenenmaske hinzufügen (Reveal All / Hide All / From Selection)  
  (Adobe: Masking ermöglicht Bereiche zu verbergen/enthüllen; Layer Masks add/edit). citeturn7search30turn7search33  
- Vektormaske hinzufügen (für Shape/Path-basiertes Masking). citeturn7search37  
- Maske anwenden / Maske löschen / Maske invertieren / Maske verfeinern  
- Clipping Mask (Schnittmaske): Basislayer definiert Sichtbarkeit der darüberliegenden Inhalte. citeturn7search2turn7search6  

**Layer Styles (Effekte)**
- Ebenenstil… (Dialog)  
  - Drop Shadow, Inner Shadow  
  - Outer Glow, Inner Glow  
  - Bevel & Emboss  
  - Satin  
  - Color Overlay, Gradient Overlay, Pattern Overlay  
  - Stroke  
  - Blend-If / Advanced Blending  
  (Adobe hat eine Übersicht „layer style effects and options“ sowie „add layer styles“). citeturn7search1turn7search36  

**Layer Comps / States**
- Layer Comps Panel (Snapshots der Layer-States: Visibility/Position/Appearance). Photopea beschreibt Layer Comps genau so. citeturn7search4

#### Text

- Text erstellen: Point Text / Area Text / Path Text  
- Absatz-/Zeichenformate (Character/Paragraph Panels)  
- Text in Formen umwandeln (Create Outlines)  
- Rechtschreibung/Find Replace (optional)

#### Auswahl

**Selektion erstellen**
- Alles auswählen / Auswahl aufheben / Auswahl erneut laden
- Farbbereich… (Color Range)
- Motiv auswählen (Subject) (später: heuristisch/ML optional)
- Schnellauswahl / Zauberstab

**Selektion modifizieren**
- Transform Selection
- Expand/Contract
- Feather
- Smooth
- Border
- Inverse

**Select and Mask / Refine Edge**
- „Auswählen und maskieren“ Workspace: Edge Refinement, View Modes etc. Adobe dokumentiert das Refining von Selections/Masks. citeturn7search7turn7search15turn7search27  

#### Filter

Du baust ein Filter-System in Kategorien, ähnlich Photopea („Filters… choose category → filter“). citeturn3search20  
Außerdem: Adobe erklärt, dass manche Filter sofort wirken, andere Dialoge öffnen. citeturn0search31

**Filter-Kategorien (Sollumfang)**
- Blur: Box/Gaussian/Motion/Radial/Surface/Field Tilt-Shift (später)
- Sharpen: Unsharp Mask/Smart Sharpen
- Noise: Add Noise/Reduce Noise/Despeckle/Median
- Distort: Lens Correction/Ripple/Twirl/Displace/Polar Coordinates
- Stylize: Emboss/Find Edges/Oil Paint (spät)
- Render: Clouds/Lighting Effects (spät)
- Other: High Pass/Offset/Minimum/Maximum
- Liquify (sehr spät)
- Vanishing Point (sehr spät; Photopea hat „Vanishing Point“ als Feature-Kategorie). citeturn7search12

#### Ansicht

Wichtig: Da du **keine Canvas-Transforms** nutzen willst, ist „Zoom“ eine Sache des Backends (Render in Zielauflösung) + „Canvas nur Blit“.

- Zoom In/Out, 100%, Fit Screen, Fill Screen
- Rotate View (nur Viewport, nicht Bild)
- Extras anzeigen: Auswahlkanten, Transform Controls, Guides, Grid, Slices (Photopea: View → Slices). citeturn7search20  
- Rulers, Units
- Snap, Snap To (Guides/Grid/Slices/Layers)
- Proof Colors / Gamut Warning (spät)
- Fullscreen Modes

#### Fenster

- Workspace Presets (Essentials, Photography, Typography, Custom)
- Panel toggles (Layers, Channels, Paths, History, Actions, Adjustments, Properties, Character, Paragraph, Brush Settings, Color, Swatches, Navigator, Info, Histogram, Styles, Layer Comps, Slices, Artboards, Variables/Data Sets, Scripts Console)

#### Hilfe

- About, Version
- Shortcuts Reference (exportierbar)
- Diagnostics (Wasm-Version, Memory, Cache)
- Report Bug (optional)

### Tool-Spezifikation (linke Toolbar)

Jedes Tool = **Zustandsmaschine** im Backend (PointerDown/Move/Up, Key modifiers). Die Options-Bar zeigt Tool-spezifische Parameter.

**Navigation/Viewport**
- Move/Hand: Pan (Space = Hand)
- Zoom: click-to-zoom, scrubby zoom, Alt=out
- Rotate View: rotiert nur View

**Auswahl-Tools**
- Marquee: Rect/Ellipse/Single Row/Single Column
- Lasso: Free/Polygon/Magnetic (später)
- Quick Selection / Magic Wand
- Object Selection (später)
- Select & Mask/Refine Edge Brush (im speziellen Workspace)

**Transform/Geometry**
- Move Tool (Auto-select Layer unter Cursor; Photopea beschreibt Auto-select Verhalten). citeturn7search35  
- Crop Tool (Rule of Thirds overlay)
- Perspective Crop (später)
- Slice Tool (Slices definieren Rechtecke; Photopea: Slice Tool). citeturn7search20

**Retouch**
- Spot Healing/Healing Brush (später)
- Patch Tool (später)
- Content-Aware Move (später)
- Red Eye Tool (optional)

**Paint/Draw**
- Brush, Pencil
- Mixer Brush (später)
- Eraser (normal/background/magic)
- Gradient Tool
- Paint Bucket (Fill)
- Dodge/Burn/Sponge (später)
- Blur/Sharpen/Smudge (später)
- Clone Stamp (später)
- History Brush (später)

**Vector**
- Pen Tool (Bezier), Freeform Pen (optional)
- Direct Selection (path points)
- Shape Tools: Rectangle, Rounded, Ellipse, Polygon, Line, Custom Shape
- Path Operations (combine/subtract/intersect)

**Text**
- Type Tool: Horizontal/Vertical, Area Type, Type on Path

**Utilities**
- Eyedropper (Sampler: add points)
- Measure Tool (optional)
- Color Picker / Swatches

### Panels/Model-Features (müssen in deinem Backend existieren)

- Layers (Tree, Groups, Lock, Blend/Opacity/Fill, Effects, Masks)
- Channels (RGB + Alpha + Spot Channels optional)
- Paths (Vector Paths, Shape Paths)
- History (Undo stack)
- Properties (kontextsensitiv: Layer/Mask/Adjustment)
- Adjustments (create adjustment layers)
- Actions/Scripts/Variables (Automation) citeturn7search28turn7search16turn7search8
- Layer Comps citeturn7search4
- Navigator (Mini viewport)
- Histogram/Info (Pixel, color readouts)

## Backend-Engine in Go/AGG: Datenmodell und Rendering-Pipeline

### Dokumentmodell

Ein Photoshop-ähnlicher Editor lebt oder stirbt mit einem klaren „Document Graph“. Vorschlag:

**Core Entities**
- `Document`: Größe, Resolution/DPI, Farbmodus, Color Profile (später), Artboards.
- `LayerNode`: Tree (Group/Layer).  
  - Typen: PixelLayer, VectorLayer, TextLayer, AdjustmentLayer, SmartObject (später)
  - Attribute: Visibility, Opacity, Fill, BlendMode, Lock flags
  - Masken: LayerMask (Raster), VectorMask (Path), ClippingMask-Beziehungen
  - LayerStyleStack (DropShadow etc.)
- `Selection`: als Alpha-Maske in Doc-Space (8/16-bit), plus optional „marching ants“ rendering data.
- `Paths`: Bezier curves (Pen), Shape layers.
- `Channels`: RGBA + extra channels opt.
- `History`: Command-basierte Undo/Redo.

### Rendering-Architektur

**Dein Backend rendert immer „final pixels“ für den aktuellen Viewport.**  
Das bedeutet: Zoom, Rotation, Overlays (Selection border, transform handles, guides, grid, slices, etc.) werden im Backend composited.

**Warum das zu deiner „kein JS-Processing“-Vorgabe passt:**  
Canvas wird nur als Ziel genutzt, um RGBA-Pixel anzuzeigen. `putImageData` malt Pixel aus einem `ImageData`-Objekt auf den Canvas; optional kann man nur ein Dirty-Rectangle aktualisieren. citeturn4search2turn1search3

**AGG-Rolle**  
AGG ist stark bei **Anti-Aliasing, Subpixel-Genauigkeit, Stroke-Rendering**, Gradients, Affine Transforms/Resampling (bilinear/bicubic/spline/sinc etc.). citeturn1search0turn1search8  
→ Nutze AGG als:
- Rasterizer für Vektor (Shapes, Pen, Text-Glyph-Outlines)
- Transform/Resampling Engine für Viewport (scale/rotate)
- Brush-Dab Rasterization (für „stamps“)

**Pixelverarbeitung (nicht-AGG)**
- Adjustment Layers: als Parameterized Ops im RenderGraph.
- Filter: als separate Kernel-Pipeline (CPU in Wasm).
- Compositing: Blend Modes (Adobe beschreibt, wie Blend Modes Pixel beeinflussen; du brauchst exakte Formeln). citeturn3search2turn3search33

### Nicht-destruktive Bearbeitung

Du willst „Photoshop-Feeling“:  
- Adjustment Layers sind nicht-destruktiv (Adobe betont Flexibilität und das Nicht-Permanente). citeturn3search34turn3search22  
- Layer Styles sind nicht-destruktiv (Adobe: add layer styles über Styles/Layer menu). citeturn7search36  
- Clipping Masks: base layer bestimmt Sichtbarkeit darüberliegender Layer. citeturn7search2

Das impliziert im Backend:
- RenderGraph, der „Upstream invalidation“ beherrscht: Änderung an einem Adjustment-Node invalidiert nur betroffene Bereiche.
- Cache-Strategie pro Layer und pro Zoom-Level (Tile Cache).

## Frontend in Vite/React/TS + shadcn/Tailwind: UI-Struktur und Zustandsfluss

### Scaffolding-Grundlagen (Framework)

**Vite**: `create-vite` ist das Standard-Tool zum Scaffolden, mit Templates für Frameworks wie React+TS. citeturn0search0turn0search8  
**shadcn/ui**: verstanden als „Open Code“-Distribution, um Komponenten zu kopieren/zu generieren und im Projekt zu besitzen. citeturn5search0turn5search15  
**Tailwind CSS v4**: bringt eine „CSS-first“ Konfiguration über `@theme` und neue Direktiven/Workflows; Tailwind selbst erklärt, dass `@theme` nicht nur Variablen sind, sondern auch Utility-Generierung steuern. citeturn4search3turn4search19turn4search35  
**shadcn + Tailwind v4**: shadcn dokumentiert explizit Support für Tailwind v4 (inkl. CLI-Init für Tailwind v4/React 19). citeturn0search1turn0search9

### „Base UI“ Einordnung

„Base UI“ (base-ui.com / MUI Base UI) ist eine **unstyled/headless** React-Komponentenbibliothek für **accessible UIs**, mit Fokus auf Composability und DX. citeturn5search31turn5search5turn5search33  
→ In deinem Setup kann Base UI die „Primitives“ liefern (Menu, Popover, Dialog, Tooltip, Listbox), während shadcn die „Opinionated Styling/Composition“ bietet.

### UI-State-Philosophie (wichtig wegen „Backend-only Rendering“)

Frontend-State darf **nur UI- und Interaktionszustand** halten, z. B.:
- aktives Tool + Tool-Optionen (aber die Engine ist Source-of-Truth)
- welche Panels offen sind, Layout, Docking
- Shortcut mapping
- transient UI (Dialog open, text input focus)
- Canvas sizing / devicePixelRatio

Alles, was „Bild“ betrifft, bleibt im Backend:
- Layer-Pixel
- Auswahl-Masken
- Filter results
- Zoom-resampling
- Overlays

Frontend sendet **Intents/Events** (Pointer, Key, Commands), Engine sendet **RenderResult** (Viewport pixels + Meta).

## WebAssembly-Brücke, Protokoll und Performance-Strategie

### Go→Wasm Interop Optionen

Du hast zwei Hauptwege:

**Weg A: klassisch `syscall/js` + `wasm_exec.js`**  
- etabliert, viele Beispiele, aber object-bridging und callback-orchestrierung können overhead haben.  
- Go-Wiki beschreibt den Go/js/wasm Workflow; Support-Files (u. a. `wasm_exec.js`) liegen je nach Go-Version in `misc/wasm` (≤1.23) bzw. `lib/wasm` (neuere Toolchains). citeturn6search4turn6search31turn6search1

**Weg B: `//go:wasmexport` (Go 1.24+) + „C-ähnliche“ Exports**  
Go 1.24 führt `go:wasmexport` als Compiler-Direktive ein. citeturn6search24turn6search0  
Wichtig: Es gibt weiterhin Typ-Limits; u. a. kann man nicht einfach Pointer/komplexe Strukturen übergeben. Das zwingt dich zu einem **Handle/Offset/Length**-ABI (was ohnehin ideal ist). citeturn6search0

**Empfehlung für dein Projekt:**  
Starte mit einem **klaren ABI**, das in beiden Welten funktioniert:
- Engine besitzt Speicher und Objekte.
- Frontend kennt nur: `docHandle`, `layerHandle`, `bufferPtr`, `bufferLen`, `jsonPtr/jsonLen`.
- Für Debug/Iteration kannst du parallel „Dev-Bridge“ mit `syscall/js` behalten.

### Rendering-Transport: „Viewport Pixels“ ohne JS-Processing

**Minimaler Canvas-Code (erlaubt):**  
- `ImageData` ist ein Container für RGBA Pixel in einem `Uint8ClampedArray` (oder Float16Array). citeturn1search3turn1search31  
- `putImageData` malt diese Pixel auf den Canvas; du kannst ein Dirty-Rectangle angeben (spart Bandbreite) und es ist *nicht* von Canvas-Transforms abhängig — perfekt, weil du bewusst keine Canvas-Transforms nutzen willst. citeturn4search2turn4search10

**Protokollvorschlag (RenderResult)**
- `frameId` (monoton)
- `viewport`: x/y (Doc-space), zoom, rotation, outW/outH (in device pixels)
- `dirtyRects[]`: Liste von rectangles (oder 1 full rect)
- `pixelFormat`: RGBA8 premultiplied (fix für MVP)
- `bufferPtr`, `bufferLen` oder „transfered ArrayBuffer“
- `uiMeta`: JSON/Flatbuffers (active layer id, cursor, statusbar text, rulers origin, etc.)

### Worker/Threading und SharedArrayBuffer

Für echte Editor-Responsiveness solltest du mittelfristig **Wasm im Worker** betreiben.  
Wenn du *zero-copy* Pixel-Sharing zwischen Worker und UI willst (statt jedes Frame ein ArrayBuffer zu transferieren), brauchst du typischerweise **SharedArrayBuffer**. Dafür muss die Seite „cross-origin isolated“ sein, was via **COOP/COEP** Headers erreicht wird. citeturn2search0turn2search26turn2search4  
Web.dev beschreibt COOP/COEP explizit als Weg, eine Seite cross-origin isolated zu machen und damit Features wie SharedArrayBuffer zu aktivieren. citeturn2search0

**Pragmatischer Stufenplan**
- MVP: Worker + Transferable ArrayBuffer (kopiert, aber einfach).
- Optimierung: SharedArrayBuffer + Ringbuffer + DirtyRects.
- Später: Wasm Threads (wenn du es wirklich brauchst) – auch hier ist COOP/COEP Voraussetzung. citeturn2search12turn2search8

## Implementierungsplan in Phasen mit Aufgabenlisten

Die Phasen sind so geschnitten, dass jede Phase **ein integriertes, testbares Zwischenergebnis** liefert. Phase 0 ist explizit „Scaffolding“ mit CLI-Tools.

### Phase 0: Scaffolding, Repo-Struktur, Build-Pipeline

**Ziel:** „Hello Editor“ — UI startet, Wasm lädt, Engine liefert einen Test-Viewport (z. B. Schachbrett/Gradient) und wird in Canvas angezeigt.

- [ ] Repo-Layout festlegen (Monorepo empfohlen)
  - [ ] `/apps/editor-web` (Vite + ReactTS)
  - [ ] `/packages/engine-wasm` (Go module für js/wasm)
  - [ ] `/packages/proto` (shared TS types + ABI constants + command IDs)
  - [ ] `/tools` (scripts: build, release, bench)
- [ ] Frontend scaffolden (CLI)
  - [ ] Vite ReactTS Template via `create-vite` (React + TS). citeturn0search0turn0search8
  - [ ] Tailwind CSS v4 Setup:
    - [ ] Tailwind import und CSS-first Tokens via `@theme` (Design Tokens). citeturn4search19turn4search3
    - [ ] Base theme (dark/light) als Token-Sets
  - [ ] shadcn initialisieren:
    - [ ] `shadcn init` (ggf. canary für Tailwind v4, je nach CLI-Stand; shadcn dokumentiert v4 Support). citeturn0search1turn0search21turn0search9
  - [ ] Base UI installieren (Primitives, headless):
    - [ ] `@base-ui/react` integrieren (unstyled + a11y Fokus). citeturn5search16turn5search31
- [ ] Go/Wasm scaffolden
  - [ ] Go module erstellen (engine-wasm)
  - [ ] `cmd/engine/main.go` + minimaler Export: `EngineInit`, `RenderTestPattern`
  - [ ] Wasm build script:
    - [ ] `GOOS=js GOARCH=wasm go build -o dist/engine.wasm` (Go Wiki beschreibt js/wasm build). citeturn0search2turn6search4
    - [ ] `wasm_exec.js` in Frontend assets kopieren (Pfad abhängig von Go-Version; Go Wiki nennt `lib/wasm` und Hinweis für ≤1.23 `misc/wasm`). citeturn6search4turn6search31
- [ ] Dev-Server Integration
  - [ ] Vite: wasm als Asset ausliefern
  - [ ] Loader: wasm laden, Engine starten, „ready“ signal
- [ ] Canvas „Display-only“ Pipeline
  - [ ] `ImageData` aus RGBA buffer erstellen (TypedArray). citeturn1search3
  - [ ] `putImageData` benutzen, zunächst full-frame; später dirty rectangles (unterstützt). citeturn4search2
- [ ] QA-Basics
  - [ ] Prettier/ESLint oder Biome
  - [ ] Go lint/test baseline
  - [ ] CI skeleton (build + unit tests)

**Abnahmekriterium:** `npm/pnpm dev` zeigt UI mit Canvas; Wasm liefert sichtbaren Viewport; keine JS-Pixelmanipulation außer putImageData.

### Phase 1: Engine-Core (Document, Viewport, Pan/Zoom) + UI-Shell

**Ziel:** Neues Dokument, Pan/Zoom/Rotate View, Statusbar, basic Panels.

- [ ] Backend: Document & Viewport
  - [ ] `Document` struct: width/height, resolution, background mode
  - [ ] `ViewportState`: center, zoom, rotation, devicePixelRatio
  - [ ] Renderer: `RenderViewport(doc, viewport) -> RGBA buffer`
  - [ ] Checkerboard-Transparenz (composited im Backend)
- [ ] Frontend: Workspace Shell
  - [ ] Menubar + ToolBar + OptionsBar + Panels Docking (min. Layers/History/Properties)
  - [ ] Canvas resize handling (devicePixelRatio an Engine melden)
  - [ ] Input routing: Pointer/Keyboard → Engine events
- [ ] Command System (Frontend→Backend)
  - [ ] Command IDs + payload schema (z. B. JSON minimal)
  - [ ] Response schema: Viewport + Meta (cursor, tool options)
- [ ] Undo/Redo minimal
  - [ ] Command pattern in Engine: `Apply(cmd)`, `Undo(cmd)`
  - [ ] UI shortcuts: Ctrl+Z/Ctrl+Shift+Z

**Abnahmekriterium:** Du kannst ein leeres Dokument öffnen, navigieren, Zoomstufen wechseln (Engine rendert korrekt), History zeigt Einträge.

### Phase 2: Layer-System (Pixel-Layer, Groups, Blend Modes, Masks) + Layers Panel

**Ziel:** „Photoshop-Grundgerüst“: mehrere Layer, Blend Modes, Masken, Sichtbarkeit.

- [ ] Backend: Layer Tree
  - [ ] LayerNode + GroupNode
  - [ ] Composite: render stack, handling opacity/fill
  - [ ] Blend Modes: implement baseline set (Normal/Multiply/Screen/Overlay/…)
    - [ ] Referenz: Adobe beschreibt Blend Modes als zentrale Pixel-Compositing-Regel. citeturn3search2turn3search33
- [ ] Masks
  - [ ] Raster Layer Mask add/edit/invert (Adobe: Layer masks hide/reveal). citeturn7search30turn7search33
  - [ ] Clipping Mask semantics (base layer defines visible boundaries). citeturn7search2
  - [ ] Vector Mask placeholder (Path→mask) (Adobe: vector masks). citeturn7search37
- [ ] Frontend: Layers Panel v1
  - [ ] Tree UI (groups, collapse)
  - [ ] visibility toggles, lock, opacity, blend-mode dropdown
  - [ ] context menu (duplicate, delete, group, add mask, clip)
- [ ] Save/Load project internal (noch nicht PSD)
  - [ ] eigenes JSON + binary blobs („.mep“ o. ä.) als Zwischenformat

**Abnahmekriterium:** Layer hinzufügen/duplizieren/verschieben; Blend modes sichtbar; Masken beeinflussen Rendering.

### Phase 3: Selektion & Transform (Move, Marquee/Lasso, Free Transform, Crop)

**Ziel:** Interaktion „wie Photoshop“: auswählen, verschieben, transformieren, beschneiden.

- [ ] Backend: Selection Engine
  - [ ] Selection als Mask buffer (doc-space alpha)
  - [ ] Selection ops: add/subtract/intersect via modifiers
  - [ ] Feather/Expand/Contract/Smooth
  - [ ] Render overlay: marching ants (backend gerendert)
- [ ] Tools
  - [ ] Marquee rect/ellipse
  - [ ] Lasso free/polygon (magnetic später)
  - [ ] Move Tool mit optional „Auto-select layer“ Verhalten (Photopea dokumentiert das). citeturn7search35
- [ ] Transform System
  - [ ] Free Transform: bbox handles, rotate, scale
  - [ ] Transform overlay (backend)
- [ ] Crop
  - [ ] Crop tool overlay, commit/cancel
- [ ] Frontend
  - [ ] OptionsBar: selection mode, feather; transform numeric input
  - [ ] Shortcuts: V (move), M (marquee), L (lasso), Ctrl+T (transform), C (crop)

**Abnahmekriterium:** Du kannst Bereiche auswählen, Layer verschieben, transformieren; UI bleibt flüssig.

### Phase 4: Painting-Basics (Brush/Pencil/Eraser/Fill/Gradient) + Brush UI

**Ziel:** „Malen“ und „Grund-Retouch“ Basis, ohne High-End Healing.

- [ ] Backend: Brush Engine v1
  - [ ] Dab rasterization (AGG für AA + subpixel hilfreich). citeturn1search0turn1search8
  - [ ] Spacing, hardness, flow, opacity
  - [ ] Stabilizer (optional)
  - [ ] Pressure API (Pointer events): später
- [ ] Tools
  - [ ] Brush, Pencil
  - [ ] Eraser variants
  - [ ] Fill (Paint Bucket) inkl. tolerance + contiguous
  - [ ] Gradient tool (linear/radial) + dither optional
  - [ ] Eyedropper
- [ ] Frontend Panels
  - [ ] Brush preset list, size slider, hardness
  - [ ] Color picker + swatches

**Abnahmekriterium:** Du kannst auf Pixel-Layern zeichnen; Undo funktioniert; Engine rendert strokes.

### Phase 5: Adjustments & Filter-System (nicht-destruktiv) + Properties/Adjustments Panel

**Ziel:** „Foto-Bearbeitung“ Kern: Tonwerte, Kurven, Hue/Sat etc. plus Filter-Pipeline.

- [ ] Adjustment Layers
  - [ ] Levels, Curves, Hue/Saturation als Adjustment Layer (Adobe dokumentiert diese Adjustments). citeturn3search18turn3search14turn3search3
  - [ ] Non-destructive toggles: hide/delete returns original (Adobe betont das Prinzip). citeturn3search34
- [ ] Filter Framework
  - [ ] Kategorie-Menü, Filter apply (einige instant, einige Dialog – Adobe beschreibt dieses UX-Muster). citeturn0search31
  - [ ] Smart Filter (später) als nicht-destruktiv bei Smart Objects
- [ ] Frontend
  - [ ] Adjustments Panel (create adjustment layer)
  - [ ] Properties Panel (per adjustment type)
  - [ ] Live preview toggles (backend rendert preview)

**Abnahmekriterium:** Adjustment Layers funktionieren non-destructive; Basis-Filter laufen.

### Phase 6: Text & Vector (Pen/Shapes/Type) + Layer Styles v1

**Ziel:** Design-/UI-Workflows: Text, Shapes, vector masks, layer styles.

- [ ] Vector/Paths
  - [ ] Pen Tool (create/edit Bezier)
  - [ ] Shapes + path ops
  - [ ] Rasterization on demand („rasterize layer“)
- [ ] Text Engine
  - [ ] Text layout (basic), font loading
  - [ ] Character/Paragraph Panels
- [ ] Layer Styles v1
  - [ ] Drop Shadow, Stroke, Outer Glow, Color Overlay
  - [ ] UI: Layer Style Dialog
  - [ ] Adobe Überblick zu layer style effects als Referenz. citeturn7search1turn7search36

**Abnahmekriterium:** Text/Shapes sind editierbar; layer styles sichtbar; export als PNG funktioniert.

### Phase 7: PSD/PSB Kompatibilität, Artboards/Slices, Automation

**Ziel:** „Photopea-ähnlicher“ Funktionsumfang: PSD als natives Format, Artboards/Slices/Actions.

- [ ] PSD/PSB I/O
  - [ ] Parser/Writer nach Adobe Spezifikation (PSB marker differences; PSB supports huge dimensions). citeturn1search1turn1search13
  - [ ] Mapping: Layer types, blend modes, masks, layer styles, text (schrittweise)
- [ ] Artboards
  - [ ] Document enthält mehrere Artboards (Photopea Konzept). citeturn7search0
  - [ ] Export: pro Artboard
- [ ] Slices
  - [ ] Slice tool + export pipeline (Photopea: slices definieren export areas). citeturn7search20
- [ ] Automation
  - [ ] Actions: record/play (Photopea beschreibt Actions+Panel). citeturn7search28
  - [ ] Variables/Data sets (Photopea beschreibt Variables→Data Sets→Export). citeturn7search8turn7search24
  - [ ] Scripting: *nicht* JS image processing, aber Scripts als „Command-Makros“; Photopea nutzt Photoshop-ähnliches Script-Model. citeturn7search16

**Abnahmekriterium:** Öffnen/Speichern von PSD (subset) funktioniert; Slices/Artboards exportieren; Actions/Variables laufen rudimentär.

### Phase 8: Performance-Hardening (Worker, Dirty Rects, Caches) + Pro UX

**Ziel:** Editor fühlt sich „professionell“ an: keine Janks, große Dateien, schnelle Tools.

- [ ] Worker-Umzug
  - [ ] Engine läuft in Worker
  - [ ] Message protocol stabilisieren
- [ ] Dirty Rect Rendering
  - [ ] Engine trackt invalid rects (stroke bounding boxes, transform regions)
  - [ ] UI nutzt `putImageData` mit dirty rectangle (MDN beschreibt Möglichkeit). citeturn4search2
- [ ] SharedArrayBuffer optional
  - [ ] COOP/COEP Setup (cross-origin isolation) — web.dev/MDN beschreiben Notwendigkeit für SharedArrayBuffer & COI. citeturn2search0turn2search26turn2search4
  - [ ] Ringbuffer + frame fences
- [ ] Multi-Resolution / Mipmaps
  - [ ] Backend erzeugt Downscale pyramids fürs schnelle Zoom-Out
- [ ] UX Features
  - [ ] Snaplines/guides, ruler origin, preferences
  - [ ] Shortcut Customizer (Export/Import, „Photoshop-like“)

**Abnahmekriterium:** Große Dokumente bleiben navigierbar; brush strokes fühlen sich flüssig an; UI blockiert nicht.

## Qualität, Tests, Build und Deployment

### Teststrategie

- **Go Engine Unit Tests**: Blend modes, selection ops, mask ops, filters (golden images).
- **Deterministische Render-Tests**: snapshot testing mit Hashes pro viewport region.
- **Interop Tests**: ABI stability (TS ↔ Wasm).
- **E2E**: Playwright: „open doc → paint → export → compare“.

Für Wasm-Testing ist es hilfreich, dass Go-Wiki ausdrücklich erwähnt, dass man js/wasm auch über Node ausführen kann (für Tests/Automation), was CI erleichtert. citeturn1search18

### Build/Release

- Vite build (prod) + Go wasm build (prod)
- Brotli/Gzip (server)
- Version stamping (engine+ui)
- Feature flags (Beta features wie Liquify, Smart Objects)

### Deployment & Security Headers (wenn du SAB/Threads willst)

Sobald du SharedArrayBuffer oder Wasm-Threads brauchst, muss die App **cross-origin isolated** sein (COOP/COEP). web.dev und MDN dokumentieren das als Voraussetzung für SharedArrayBuffer. citeturn2search0turn2search26  
→ Plane Deployment so, dass du diese Header setzen kannst (nicht jede Hosting-Plattform erlaubt das ohne Weiteres).

### Lizenz- und Third-Party-Audit

- Prüfe AGG-Version/Fork-Lizenzlage (GPL vs permissiv) und entferne/ersetze GPC, wenn kommerziell relevant. citeturn2search3turn2search25turn2search29  
- Prüfe Fonts (EULA), RAW Decoder (patent/licensing), PSD/PSB Spezifikationsnutzung (Spec ist öffentlich zugänglich). citeturn1search1

