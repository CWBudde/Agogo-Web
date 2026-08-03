# ABR compatibility and synthetic fixture provenance

The ABR fixtures in this package are generated in `parser_test.go`; they are
not copied from Photoshop, a public brush library, or another parser project.
They contain only synthetic UUIDs, descriptor values, dimensions, and pixels.

The builders cover the parser compatibility matrix:

- ABR major versions 6 and 7
- descriptor subversions 1 and 2
- sampled 8-bit tips using raw and row-scoped PackBits/RLE storage
- computed-brush metadata represented by a version-16 Action Descriptor
- multiple and unknown `8BIM` sections

## Supported matrix

| Area | Supported | Not supported / behavior |
| --- | --- | --- |
| Container | ABR major 6 and 7; subversion 1 and 2 | Other versions fail with `ErrUnsupported` |
| Sampled tips | 8-bit masks; raw and row-scoped PackBits | 16-bit masks and other compression modes fail explicitly |
| Descriptor values | Object, list, double, unit float, text, enum, integer, Boolean, class, alias/path/data | Unknown descriptor value types fail explicitly |
| Tip metadata | `Dmtr`/`diameter` in pixels; `Hrdn`/`hardness`, `Spcn`/`spacing`, and `Rndn`/`roundness` as percentages; `Angl`/`angle` in degrees | Values are range-checked and clamped with a warning; unsupported units are ignored with a warning |
| Dynamics | Nested `sizeDynamics`/`SzDn`, `opacityDynamics`/`OpDn`, and `flowDynamics`/`FlDn`; `Jitr`/`jitter` percentage, `Cntrl`/`control`, and `FadD`/`fadeDabs` | Controls are limited to off, pen pressure, pen tilt, and fade. The current UI has one shared control/fade length, so differing per-channel values use the first and emit a warning |
| Other brush behavior | Preset name (`Nm  ` or `name`) | Texture, dual-brush, scattering descriptors, color dynamics, wet-media behavior, and every other unmapped descriptor leaf are preserved only as explicit per-preset warnings; they are not approximated |

Sampled records and descriptors are paired by their order in the synthetic
fixtures. That association, the four-character metadata aliases above, inverse
versus direct sampled-mask polarity, and behavior across Photoshop-produced
files still require validation against a licensed, redistributable external
fixture set. This repository currently has no such fixture, so it does not
claim real-world Photoshop compatibility beyond the bounds-checked synthetic
matrix above.

No external/public fixture is currently included, so no third-party fixture
license or attribution applies.
