; generate_gimp.scm — LAYERED PSD fixtures for the Agogo PSD corpus (Phase S.10.1).
;
; GIMP 3 Script-Fu. Invoked by generate_gimp.sh; see README.md for the batch form
; and the hang hazard.
;
; ┌─────────────────────────────────────────────────────────────────────────────┐
; │ UNVERIFIED — READ THIS FIRST                                                │
; │                                                                             │
; │ Every generator below is written but NOT confirmed to run end to end. On the │
; │ machine this was authored on (GIMP 3.2.6, snap 561), image construction      │
; │ works fine — gimp-image-new, gimp-layer-new, gimp-image-insert-layer,        │
; │ gimp-drawable-edit-fill and gimp-message all succeed — but EVERY call to     │
; │ gimp-file-save hangs indefinitely and ignores SIGTERM, for PSD and for PNG   │
; │ alike, and whether the destination is under /mnt or under $HOME. So the      │
; │ argument list of gimp-file-save could not be bisected: the process never     │
; │ returns, and Script-Fu's `catch` does not help because the call BLOCKS       │
; │ rather than raising.                                                         │
; │                                                                             │
; │ There is exactly one place to fix when that is resolved: psdfix-save.        │
; └─────────────────────────────────────────────────────────────────────────────┘
;
; Blend-mode note: do NOT treat the LAYER-MODE-* constants below as a prediction of
; the PSD key that ends up in the file. GIMP's PSD exporter maps only part of its
; mode vocabulary and silently writes "norm" for the rest. That is fine, because
; derive_expectations.py READS THE RESULT BACK and records whatever key was really
; written. Generate, then derive, then read the sidecar to learn the true mapping —
; never hand-write the expected blend mode.

; ── save helper ───────────────────────────────────────────────────────────────

; psdfix-save — write IMG to FILENAME.
;
; THE ONE UNVERIFIED CALL. GIMP 3 dropped the GIMP 2 "raw filename" second path
; argument, so the three-argument form below is the documented GIMP 3 signature:
;
;     (gimp-file-save run-mode image file)
;
; If it turns out to be wrong, the alternatives to try, in order, are:
;   1. (gimp-file-save RUN-NONINTERACTIVE img filename filename)  ; GIMP 2 form
;   2. (file-psd-export RUN-NONINTERACTIVE img filename)          ; the plug-in
;      procedure directly — `strings` on the snap's file-psd plug-in confirms
;      "file-psd-export" is the registered name in GIMP 3 (GIMP 2 had
;      "file-psd-save").
;   3. GIMP 3.0 export options objects, via gimp-export-options-*.
(define (psdfix-save img filename)
  (gimp-file-save RUN-NONINTERACTIVE img filename)
  (gimp-message (string-append "wrote " filename)))

; ── small builders ────────────────────────────────────────────────────────────

; psdfix-add-layer — create a filled layer and insert it into PARENT.
;
; Layers are inserted at position 0, which is the TOP of the stack, so calling
; this repeatedly builds the stack bottom-first: the last layer added is topmost.
; That is deliberate — it matches the bottom-to-top order PSD stores records in
; and the order the engine stores children, so the source here reads in the same
; direction as the resulting sidecar.
(define (psdfix-add-layer img parent name x y w h colour opacity mode)
  (let ((layer (car (gimp-layer-new img w h RGBA-IMAGE name opacity mode))))
    (gimp-image-insert-layer img layer parent 0)
    (gimp-layer-set-offsets layer x y)
    (gimp-context-set-foreground colour)
    (gimp-drawable-edit-fill layer FILL-FOREGROUND)
    layer))

; psdfix-add-group — create a group layer and insert it into PARENT.
(define (psdfix-add-group img parent name mode)
  (let ((grp (car (gimp-group-layer-new img name))))
    (gimp-image-insert-layer img grp parent 0)
    (gimp-layer-set-mode grp mode)
    grp))

; psdfix-paint-rect — paint a rectangle of COLOUR inside LAYER (document coords).
; Used to give a layer several distinct colour regions, so that a pixel sample
; taken from the wrong coordinate cannot accidentally pass.
(define (psdfix-paint-rect img layer x y w h colour)
  (gimp-image-select-rectangle img CHANNEL-OP-REPLACE x y w h)
  (gimp-context-set-foreground colour)
  (gimp-drawable-edit-fill layer FILL-FOREGROUND)
  (gimp-selection-none img))

; ── fixtures ──────────────────────────────────────────────────────────────────
;
; Each fixture is a named function taking the output directory, so that a
; sidecar's provenance.generatedBy can point at it precisely, e.g.
;   "generatedBy": "tools/psdfixtures/generate_gimp.scm#psdfix-nested-groups"

; rgb8-nested-groups — 64x48. Group nesting plus group isolation.
;
;   Background                     (pixel, full canvas, two colour regions)
;   Outer                          (group, Normal   -> isolated)
;     Inner                        (group, Pass through -> not isolated)
;       Inner Fill                 (pixel, Multiply)
;     Outer Fill                   (pixel, Normal)
(define (psdfix-nested-groups outdir)
  (let* ((img (car (gimp-image-new 64 48 RGB)))
         (bg (psdfix-add-layer img 0 "Background" 0 0 64 48
                               '(32 48 96) 100.0 LAYER-MODE-NORMAL)))
    (psdfix-paint-rect img bg 0 0 32 48 '(210 60 60))
    (let* ((outer (psdfix-add-group img 0 "Outer" LAYER-MODE-NORMAL))
           (inner (psdfix-add-group img outer "Inner" LAYER-MODE-PASS-THROUGH)))
      (psdfix-add-layer img inner "Inner Fill" 8 8 24 24
                        '(240 200 40) 100.0 LAYER-MODE-MULTIPLY)
      (psdfix-add-layer img outer "Outer Fill" 36 12 20 20
                        '(60 200 120) 100.0 LAYER-MODE-NORMAL))
    (psdfix-save img (string-append outdir "/rgb8-nested-groups.psd"))
    (gimp-image-delete img)))

; rgb8-mask-offset — 64x48. A layer mask whose rect does NOT start at the origin.
;
; A GIMP layer mask always covers exactly its layer, so the way to get a non-zero
; mask offset in the exported PSD is to offset the LAYER. "Masked" sits at (12,10)
; and is 32x24, so the mask rect should come out as (12,10)-(44,34). The mask is
; painted black in its left half, which makes the boundary detectable in a sample.
(define (psdfix-mask-offset outdir)
  (let* ((img (car (gimp-image-new 64 48 RGB))))
    (psdfix-add-layer img 0 "Background" 0 0 64 48
                      '(32 48 96) 100.0 LAYER-MODE-NORMAL)
    (let* ((masked (psdfix-add-layer img 0 "Masked" 12 10 32 24
                                     '(230 90 40) 100.0 LAYER-MODE-NORMAL))
           (mask (car (gimp-layer-create-mask masked ADD-MASK-WHITE))))
      (gimp-layer-add-mask masked mask)
      ; Black out the mask's left half -> that half of the layer is transparent.
      (psdfix-paint-rect img mask 12 10 16 24 '(0 0 0)))
    (psdfix-save img (string-append outdir "/rgb8-mask-offset.psd"))
    (gimp-image-delete img)))

; rgb8-mask-disabled — 64x48. Same shape as rgb8-mask-offset, but the mask is
; switched OFF. The mask data is still written; only its enabled flag differs, so
; the pair isolates "is the mask flag honoured" from "is the mask data read".
(define (psdfix-mask-disabled outdir)
  (let* ((img (car (gimp-image-new 64 48 RGB))))
    (psdfix-add-layer img 0 "Background" 0 0 64 48
                      '(32 48 96) 100.0 LAYER-MODE-NORMAL)
    (let* ((masked (psdfix-add-layer img 0 "Masked" 12 10 32 24
                                     '(230 90 40) 100.0 LAYER-MODE-NORMAL))
           (mask (car (gimp-layer-create-mask masked ADD-MASK-WHITE))))
      (gimp-layer-add-mask masked mask)
      (psdfix-paint-rect img mask 12 10 16 24 '(0 0 0))
      (gimp-layer-set-apply-mask masked FALSE))
    (psdfix-save img (string-append outdir "/rgb8-mask-disabled.psd"))
    (gimp-image-delete img)))

; rgb8-visibility-opacity — 64x48. A hidden layer and a partly transparent one.
;
;   Background   visible, opacity 100
;   Hidden       NOT visible, opacity 100   -> must not contribute to the composite
;   Half         visible, opacity 50        -> opacity255 should land on 127 or 128
(define (psdfix-visibility-opacity outdir)
  (let* ((img (car (gimp-image-new 64 48 RGB))))
    (psdfix-add-layer img 0 "Background" 0 0 64 48
                      '(32 48 96) 100.0 LAYER-MODE-NORMAL)
    (let ((hidden (psdfix-add-layer img 0 "Hidden" 4 4 24 24
                                    '(255 0 255) 100.0 LAYER-MODE-NORMAL)))
      (gimp-item-set-visible hidden FALSE))
    (psdfix-add-layer img 0 "Half" 32 16 24 24
                      '(250 250 250) 50.0 LAYER-MODE-NORMAL)
    (psdfix-save img (string-append outdir "/rgb8-visibility-opacity.psd"))
    (gimp-image-delete img)))

; rgb8-blend-modes — 96x96. One small opaque patch per blend mode, in a grid.
;
; The modes listed are the ones GIMP's PSD exporter is EXPECTED to map to a
; distinct PSD key. Do not trust that expectation: generate, run
; derive_expectations.py, and read psdBlendKey out of the sidecar. Any mode that
; comes back as "norm" was not mapped and should be dropped from this list with a
; comment, rather than left in to produce a misleading fixture.
(define psdfix-blend-modes-list
  (list
    (list "Multiply" LAYER-MODE-MULTIPLY)
    (list "Screen" LAYER-MODE-SCREEN)
    (list "Overlay" LAYER-MODE-OVERLAY)
    (list "Difference" LAYER-MODE-DIFFERENCE)
    (list "Darken" LAYER-MODE-DARKEN-ONLY)
    (list "Lighten" LAYER-MODE-LIGHTEN-ONLY)
    (list "Hue" LAYER-MODE-HSV-HUE)
    (list "Saturation" LAYER-MODE-HSV-SATURATION)
    (list "Color" LAYER-MODE-HSL-COLOR)
    (list "Luminosity" LAYER-MODE-HSV-VALUE)
    (list "ColorDodge" LAYER-MODE-DODGE)
    (list "ColorBurn" LAYER-MODE-BURN)
    (list "HardLight" LAYER-MODE-HARDLIGHT)
    (list "SoftLight" LAYER-MODE-SOFTLIGHT)
    (list "Exclusion" LAYER-MODE-EXCLUSION)
    (list "LinearBurn" LAYER-MODE-LINEAR-BURN)
    (list "VividLight" LAYER-MODE-VIVID-LIGHT)
    (list "PinLight" LAYER-MODE-PIN-LIGHT)
    (list "LinearLight" LAYER-MODE-LINEAR-LIGHT)
    (list "HardMix" LAYER-MODE-HARD-MIX)))

(define (psdfix-blend-modes outdir)
  (let* ((img (car (gimp-image-new 96 96 RGB))))
    (psdfix-add-layer img 0 "Background" 0 0 96 96
                      '(96 128 160) 100.0 LAYER-MODE-NORMAL)
    (let loop ((modes psdfix-blend-modes-list) (i 0))
      (if (not (null? modes))
          (let* ((entry (car modes))
                 (name (car entry))
                 (mode (cadr entry))
                 (x (* 16 (modulo i 6)))
                 (y (* 16 (quotient i 6))))
            (psdfix-add-layer img 0 name x y 16 16
                              '(200 140 60) 100.0 mode)
            (loop (cdr modes) (+ i 1)))))
    (psdfix-save img (string-append outdir "/rgb8-blend-modes.psd"))
    (gimp-image-delete img)))

; psdfix-generate-all — every fixture, for the driver's default mode.
(define (psdfix-generate-all outdir)
  (psdfix-nested-groups outdir)
  (psdfix-mask-offset outdir)
  (psdfix-mask-disabled outdir)
  (psdfix-visibility-opacity outdir)
  (psdfix-blend-modes outdir)
  (gimp-message "psdfix: all fixtures generated"))
