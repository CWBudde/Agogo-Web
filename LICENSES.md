# License Audit

This document records the license decisions for Agogo-Web and its dependencies.

## Agogo-Web

All original code in this repository (apps/editor-web, packages/proto, packages/engine-wasm entry points) is proprietary — MeKo-Tech, all rights reserved — unless a separate license file is added.

---

## Key Dependency: agg_go (`github.com/CWBudde/agg_go`)

- **What it is:** A Go port of the Anti-Grain Geometry (AGG) 2.6 rendering library, providing the 2D rasterization pipeline used by the WebAssembly engine.
- **Author:** MeKo-Christian (same GitHub organization as this project).
- **License status:** No LICENSE file present in the repository as of Phase 0 (Mar 2026). Being an in-house dependency, usage is permitted for this project.
- **Action required:** A license file should be added to `agg_go` before any public release of Agogo-Web. Recommended: MIT or Apache-2.0.

### Upstream AGG License History (for reference)

| AGG Version | License |
|-------------|---------|
| ≤ 2.4       | Anti-Grain Geometry Public License (Modified BSD — permissive) |
| 2.5+        | Changed by Maxim Shemanarev — consult upstream source |

`agg_go` is a clean Go reimplementation and is not a verbatim copy of the C++ sources.

---

## GPC (General Polygon Clipper)

- **Location:** `internal/gpc/` within `agg_go`.
- **Original author:** Alan Murta, Advanced Interfaces Group, University of Manchester.
- **Version ported:** 2.32 (December 2004).
- **License:** **Non-commercial use only.** The original GPC library explicitly prohibits commercial use without a separate commercial license.
- **Impact:** Any commercial release of Agogo-Web must replace GPC with a permissively-licensed alternative.

### Decision

- **Phase 0–1:** GPC is acceptable for development and non-commercial use.
- **Before commercial release:** Replace with [Clipper2](https://github.com/AngusJohnson/Clipper2) (Boost Software License 1.0 — permissive) or equivalent. The `agg_go` codebase notes Clipper2 as the recommended replacement.
- **Owner:** To be tracked as a pre-release blocker.

---

## Frontend Dependencies (apps/editor-web)

All frontend dependencies use permissive open-source licenses (MIT / Apache-2.0):

| Package | License |
|---------|---------|
| react, react-dom | MIT |
| @vitejs/plugin-react, vite | MIT |
| tailwindcss, @tailwindcss/vite | MIT |
| @base-ui-components/react | MIT |
| @biomejs/biome | MIT / Apache-2.0 |
| lefthook | MIT |
| typescript | Apache-2.0 |

No copyleft (GPL/LGPL) frontend dependencies are present.
