# External verification log

Every row records a non-Agogo tool opening a file Agogo *wrote* (the re-export
leg of the round trip). Reading a fixture correctly proves nothing about the
writer; only a third-party reader accepting Agogo's output does.

The sidecar's `writer.externalVerification` block points here. A fixture that
asserts the `writer` scope must have a row below.

| Fixture id | Tool | Version | Date | Result | Notes |
| --- | --- | --- | --- | --- | --- |
| `rgb8-flat-raw` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | 16x16 RGB/8, one `L1` record, `norm`, four sampled quadrant colours correct. |
| `rgb8-flat-rle` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Reads back identically to `rgb8-flat-raw`; the RAW/RLE difference does not survive re-export, which is correct. |
| `gray8-flat` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | `identify` reports `8-bit Gray`; the grey bands survive. |
| `psb-flat` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Re-export keeps header version 2; psd-tools reports `version=2 (PSB)`. |
| `near-psd-limit` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | 30000x2 survives; both readers report the full width and the correct end-column pixels. |
| `depth16-rejected` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | n/a | Negative fixture: the engine refuses it, so there is no re-export to verify. psd-tools reads the 16-bit original fine, which is what makes the refusal a policy rather than a parse failure. |
| `rgb8-nested-groups` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Two-level nesting and the bounding/open divider pairing survive the round trip. |
| `rgb8-group-passthrough` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | The pass-through group is re-emitted with blend key `pass`; the isolated one stays `norm`. |
| `rgb8-group-closed-folder` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Both folders re-open as groups, and the open/closed flag survives: `GroupLayer.Expanded` carries `lsct` 1 vs 2 through import and export (S.10.3, closed 2026-09-18). Asserted at record scope by the fixture. |
| `rgb8-blend-modes` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | All 27 blend keys survive the write and read back byte-exact, trailing spaces included (`mul `, `idiv`, `dkCl`, `smud`, `fsub`, `fdiv`, `hue `, `sat `, `colr`, `lum `). |
| `rgb8-opacity-hidden` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Opacity 128/64 and the cleared visible flag all survive; `identify` lists each layer at its own offset. |
| `rgb8-fill-opacity` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | partial | The `iOpa` block survives Agogo's write: psd-tools reads the re-export back as fill 128 and fill 0, with the layer that carries no block at 255. Remaining loss is channel order only. |
| `rgb8-layer-mask-offset` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | partial | Re-verified after the S.10.3/S.10.4 writer fix: the layer keeps its full (4,3) 24x18 bounds and the mask now round-trips bit-exact (the sampled blue reads 144, not the 92 this row previously recorded). Remaining loss is the stored `mask.rect`, which import widens to full canvas. |
| `rgb8-mask-disabled` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | The mask-disabled flag survives. Re-verified after the S.10.3/S.10.4 writer fix, which changed this file's raster (the mask is no longer baked into the pixels). |
| `rgb8-mask-inverted` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | partial | Re-verified after the S.10.3/S.10.4 writer fix: the layer keeps its full (4,3) 24x18 bounds instead of being trimmed to (10,7) 12x8, so the mask is non-destructive again. Remaining losses are the stored `mask.rect` and the folded invert flag, both import-side. |
| `rgb8-clipping` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | partial | Re-verified after the S.10.3/S.10.4 writer fix: the clipped layer keeps its full (8,6) 18x14 bounds instead of being cut to (8,9) 11x11, so no pixels are destroyed. Remaining loss is channel order only. |
| `rgb8-group-empty` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | The childless group survives as a group with no children, and the populated group beside it is unaffected. |
| `rgb8-clipping-across-group` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | partial | Nesting and the in-group clip pair survive. The boundary-clipped layer's clipping byte does not: it has no base among its siblings, so the engine clears the flag (knownGaps entry + writer.lossy). |
| `rgb8-mask-larger-than-layer` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | The layer keeps its own (8,6) 14x12 bounds under a 24x20 mask; only the stored mask rect and default fill are widened. |
| `rgb8-group-mask` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | The mask stays on the group's opening record and still attenuates both children; only the stored mask rect and default fill are widened. |
| `rgb8-rle-layers` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-16 | pass | Three RLE-compressed layers re-open with correct bounds and pixels. |
| `rgb8-zip-layers` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | Three ZIP-compressed layer channels decode and re-open with correct bounds and pixels. The re-export is not itself ZIP — `psdexport` picks its own channel compression — so this row verifies the decoder leg, not a ZIP writer. |
| `rgb8-zip-prediction-layers` | psd-tools + ImageMagick | 1.19.0 / 6.9.12-98 | 2026-09-17 | pass | Same three layers under ZIP-with-prediction. The predictor resets at every row boundary; a decoder that carries the running sum across rows disagrees on 736 of 768 bytes of the first channel, including the sampled bottom-row pixels, so this fixture is not vacuous. |

## How these rows were produced

```bash
cd packages/engine-wasm && GOWORK=off go test ./internal/engine/ -run TestPSDFixtureReexport -count=1
cd ../.. && just fixtures-verify
```

The Go test writes each re-export to `internal/io/psdfixture/_dump/` (gitignored, and
uploaded as a CI artifact); `just fixtures-verify` re-reads them with psd-tools and,
via `--identify`, with ImageMagick as a second independent implementation. The last
run reported `22/22 file(s) parsed` - every file Agogo wrote was accepted by a reader
that is not Agogo's own.

That run had ImageMagick installed, so it also retired an earlier caveat: the
2026-09-17 rows previously rested on psd-tools alone, and every one of them has now
been re-read by both implementations. ImageMagick does refuse a *group*-bearing file
at `psd.c/ReadPSDLayersInternal` - but it refuses the pytoshop-authored source
fixtures identically, because a group divider record carries no channels at all, so
that is a limitation of that reader rather than something Agogo emits wrongly. The
`identify` lines in those cases report the error and the psd-tools leg carries the
row.

`partial` means the file opens correctly and its structure is right, but a specific
field is knowingly not reproduced. Each one is listed in that fixture's
`writer.lossy` with a `writer.lossyReason` naming the owning PLAN.md phase, and the
harness fails if such a path ever starts matching, so a fix cannot land unnoticed.

Not verified by Photoshop itself: nobody on this project has a licence. The strongest
claim this corpus makes is "two independent third-party readers accept Agogo's
output", not "Photoshop accepts it".
