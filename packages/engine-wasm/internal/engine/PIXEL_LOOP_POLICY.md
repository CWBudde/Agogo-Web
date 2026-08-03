# Engine Pixel-Loop Policy

Status: reviewed after the Phase S.9 compositor, viewport, gradient, and crop migrations (2026-08-02).

Rendering, blending, gradient generation, and affine image resampling belong in `agg_go`. Use `CompositeImage` for rect/mask/opacity/comp-op work, `DrawImageAffine` for transformed or filtered images, and `RenderGradient` with `GradientLUT` for gradients. A direct engine pixel loop is allowed only when it is an exact data remap, an adapter into or out of an AGG operation, a pixel-domain algorithm for which AGG has no equivalent, or a pixel-crisp UI overlay. “It was easier” is not a reason.

`pixel_loop_policy_test.go` is the executable per-function allowlist. Its rationale strings are the most precise reason of record; the groups below explain the policy decisions and the migration tickets.

## Keep manual

| Class | Functions | Reason of record |
| --- | --- | --- |
| Exact discrete remaps | `flipPixelsH`, `flipPixelsV`, `rotatePixels90CW`, `rotatePixels90CCW`, `rotatePixels180`, `compositeViewportIdentity` (opaque row-copy branch) | Phase X.7 established that bit-exact index/row rearrangements need no interpolation. The measured identity row copy avoids per-pixel compositor setup; translucent viewport pixels still use `CompositeImage`. |
| Exact selection/bounds remaps | `extractSelectionContent`, `clearSelectionContent`, `extractSelectionFromSurface`, `cropSurfaceBounds`, `trimPixelLayerToBounds`, `mergePixelLayerOnto`, `suggestMagneticPath` | These discover bounds, lift/clear selected bytes, place an existing surface, or extract an AGG Sobel input. They do not rasterize or resample; `mergePixelLayerOnto` delegates its actual source-over step to `CompositeImage`. |
| AGG input/output adapters | `maskRegionToRGBA`, `writeMaskRegionFromRGBA`, `paintSampledBrushTip`, `renderBrushResourceThumbnail`, `clearPixelLayerSelection`, `fillRasterWithMask`, `premultipliedDocumentSurface` | These convert one-channel masks/tips or copy dirty rows into AGG-compatible source/mask buffers. AGG performs the transform, premultiplication, DstOut, or final composite. The cut path restores hidden straight-alpha RGB deliberately after AGG changes alpha. |
| Destination-dependent decisions | `eraseBackgroundDabResource` (`EraseBackgroundDab`), `applyBlendIfChannelsClipped` | Tolerance erase must inspect the existing destination colour before deciding whether to erase. BlendIf channel gating conditionally restores individual channels and is not an AGG comp-op. |
| Pixel-domain solvers and filters | `diffuseCropExpansion`, `filterBoxBlur`, `filterSurfaceBlur`, `applyMedian`, `filterMinimum`, `filterMaximum`, `filterReduceNoise` | Diffusion, edge-aware/order-statistic/morphological filtering, and the product's exact box filter are neighbourhood algorithms, not rasterization or affine resampling. No byte-compatible public AGG primitive exists; `filterReduceNoise` already uses AGG for its blur substep. |
| Content-aware crop bookkeeping | `buildContentAwareCropFillLayer` | `DrawImageAffine` now performs crop resampling. The remaining loops classify known/unknown cells, run the diffusion solver, and clear pixels outside the expansion mask. |
| Pixel-crisp editor overlays | `RenderTransformHandlesOverlay`, `RenderCropOverlay`, `RenderSelectionOverlay` | Phase X.7 keeps transform handles manual because AGG antialiasing softens screen-space handles and setup dominates this small UI operation. Crop handles/grid and marching ants use the same structural UI policy. |

## Ticketed migrations

These loops are allowed temporarily, not designated permanent exceptions.

| Ticket | Functions | Required migration |
| --- | --- | --- |
| `S9-MASK-COMPOSITE` | `applyAdjustmentLayerRectToSurface`, `applyFilteredWithMask`, `applyFilteredRGBAWithMask`, `filterGaussianBlur`, `paintDabClippedToSelection`, `applySelectionMaskToDocBuffer` | Keep pixel-domain result generation where needed, but apply selection/layer coverage through `AlphaMask` + `CompositeImage`. Preserve dirty rectangles, premultiplied interpolation, hidden RGB, and stroke accumulation semantics. |
| `S9-BRUSH-SAMPLING` | `cloneStampDabResource` | Replace the private bilinear clone-source sampler with a public AGG filtered-image/span path; retain product-specific dab and load masks. |
| `S9-IMAGE-SCALE` | `scaleRGBA`, `scaleGrayToRGBA` | Use `DrawImageAffine` for nearest scaling. Add a grayscale adapter before removing `scaleGrayToRGBA`. |
| `S9-FILTER-DISTORT` | `filterLensCorrection` | Expose/port a nonlinear AGG span interpolator; an affine draw cannot express radial distortion and chromatic displacement. |
| `S9-ALPHA-CONVERSION` | `premultiplyRGBA`, `unpremultiplyRGBA` | Attach filter buffers safely to `agg_go.Image` and use `Premultiply`/`Demultiply`; prove byte parity before replacing the helpers. |
| `S9-STYLE-SPANS` | `renderMaskedLinearGradientLUT`, `applyGradientDitherMasked`, `renderMaskedPatternSurface` | Move style gradients to `RenderGradient`/`GradientLUT`, including masked dithering, and add a public repeating-pattern span for pattern styles. |

## Review and enforcement

`TestManualPixelLoopAllowlist` parses production Go files in this package and fails when a function containing a loop directly writes three or four adjacent RGBA channels (including short `copy`/`clear` spans) without an allowlist entry. It also fails when an allowlisted function is removed or renamed, prompting cleanup of the policy.

The guard is intentionally syntax-only. During review, also search for channel-variable loops, alpha-only writes, writes hidden in closures/helpers, unsafe/pointer writes, and new pixel buffers passed to non-AGG compositors. `diffuseCropExpansion`, `eraseBackgroundDabResource`, and the adapter functions are explicitly listed because some of those shapes are outside the guard's adjacent-channel heuristic.

For any new finding, choose one outcome in the same change:

1. migrate it to the public AGG API;
2. add the exact `file:function` allowlist entry and a structural or measured reason here; or
3. assign a named migration ticket with the missing AGG primitive and required parity contract.
