# PSD fixture tooling

Generation and expectation-derivation tooling for the PSD import/export test corpus
(Phase S.10.1). Nothing here is part of the build: `just ci` and `go test` stay
Go-only and must never acquire a Python dependency.

The design rests on one property — **independence**. A fixture binary is written by
one third-party tool, and the numbers it is judged against are derived by a
*different* third-party tool. Agogo appears nowhere in that loop, so a test failure
means Agogo disagrees with the rest of the world rather than with itself.

```
ImageMagick / GIMP  ──>  fixture.psd  ──>  psd-tools  ──>  fixture.expected.json
   (writes bytes)                        (reads bytes)         (the sidecar)
                                                                     │
                                          Agogo is compared against ─┘
```

## Tool versions

Verified on 2026-09-16, on the machine this was authored on:

| Tool | Version | Path | Role |
| ---- | ------- | ---- | ---- |
| psd-tools | 1.19.0 | `requirements.txt` | reads fixtures, derives expectations |
| Python | 3.12.4 (anaconda) | `/home/christian/anaconda3/bin/python3` | runs the scripts |
| pytoshop | 1.2.1 | `requirements.txt` | writes layered fixtures |
| ImageMagick | 6.9.12-98 Q16 | `/usr/bin/convert` | writes flat fixtures |
| GIMP | 3.2.6 (snap 561) | `/snap/bin/gimp` | **abandoned** as a writer, see below |

Re-verified on 2026-09-17 on CPython 3.11.15 (GCC 13.3.0), where the whole pinned
set installs cleanly and **regenerates every committed layered fixture byte for
byte**. That reproduction is the real check on this table: if the versions drift in
a way that matters, the bytes move.

## Setting the toolchain up

```bash
just fixtures-setup                    # -> .venv-psdfixtures/ (gitignored)
.venv-psdfixtures/bin/python tools/psdfixtures/generate_pytoshop.py --out .fixtures-out
```

`requirements.txt` pins the whole tree, transitives included, because a corpus is
only reproducible if its writer and its deriver are. Nothing in `just ci`,
`just test` or `go test` touches any of it — the fixtures and sidecars are
committed, so this is a maintenance toolchain, not a build dependency.

Use a **virtualenv**, not the system interpreter: pytoshop has no wheel and builds
from its sdist, and that build fails against a Debian-patched system `setuptools`
with `AttributeError: install_layout`. It also has an undeclared dependency on
`six`, which is why `requirements.txt` lists it explicitly.

## Files

| File | Purpose |
| ---- | ------- |
| `generate_imagemagick.sh` | flat (single-image) PSD/PSB fixtures — **working** |
| `generate_pytoshop.py` | layered fixtures: groups, masks, blend modes, clipping — **working** |
| `generate_gimp.scm` | superseded by `generate_pytoshop.py`; kept only as a GIMP reference |
| `generate_gimp.sh` | driver for the above — superseded |
| `derive_expectations.py` | reads a fixture with psd-tools, emits `<id>.expected.json` |
| `requirements.txt` | the pinned Python toolchain; `just fixtures-setup` installs it |
| `verify_dump.py` | re-reads Agogo-*written* PSDs with psd-tools (+ optional `identify`) |

## ImageMagick: the layered-writer defect

ImageMagick 6.9.12-98's PSD writer **byte-swaps blend mode keys whenever it writes
more than one layer**. It emits `mron` instead of `norm`, and psd-tools refuses the
file outright:

```
ValueError: b'mron' is not a valid BlendMode
```

Agogo's own importer survives it but warns
`unknown PSD blend mode key "mron"; imported as Normal`, which is the correct
response to a corrupt file — and precisely why such a file must never become a
fixture. A fixture has to be *valid* input, or the test is asserting on garbage.

The trigger is the **number of images ImageMagick ends up writing**, nothing else;
`-alpha set` and `-set label` make no difference:

| Command shape | Result |
| ------------- | ------ |
| `convert -size 8x8 xc:red -depth 8 out.psd` | `8BIMnorm` — valid |
| `convert -size 8x8 xc:red \( -size 8x8 xc:blue \) -depth 8 out.psd` | `8BIMmron` — corrupt |

**So: ImageMagick is used here only for fixtures that resolve to exactly one
image.** That is not as limiting as it sounds — `+append` and `-append` *collapse* a
multi-image sequence into a single image before the PSD writer ever sees it, so the
fixtures in `generate_imagemagick.sh` still carry several distinct colour blocks
while writing exactly one layer. Every generated file was checked with
`xxd f.psd | grep -o '8BIM....'` and all report `8BIMnorm`.

Two more things worth knowing about ImageMagick's PSD output:

- "Flat" here means **one layer record**, not *no* layer records. ImageMagick always
  writes a layer-and-mask section containing a single layer named `L1`. That is the
  name the sidecars assert, and the name pixel samples are keyed by.
- It writes **no image resource block at all** unless `-density` is given, so there
  is no resolution resource and every reader falls back to 72 dpi. `gray8-flat` is
  the one fixture that passes `-units PixelsPerInch -density 144`, so the corpus
  covers both parsing resource 1005 and defaulting when it is absent.

## GIMP: UNVERIFIED, and why

`generate_gimp.scm` and `generate_gimp.sh` are **written but never seen to produce a
file.** Keep them; they are the starting point for whoever unblocks GIMP.

What is confirmed to work in GIMP 3.2.6 batch mode:

```bash
timeout -k 15 300 gimp -i --batch-interpreter=plug-in-script-fu-eval \
    -b '<scheme expression>' \
    -b '(gimp-quit 0)'
```

`--batch-interpreter=plug-in-script-fu-eval` is required — without it GIMP 3 expects
a *script name* rather than an expression. Confirmed working procedures:
`gimp-image-new`, `gimp-layer-new`, `gimp-image-insert-layer`,
`gimp-context-set-foreground`, `gimp-drawable-edit-fill`, `gimp-image-get-width`,
`gimp-message`, and Script-Fu's `catch`. `gimp-file-load` takes **one** path argument
in GIMP 3, where GIMP 2 took two.

**The blocker:** every call to `gimp-file-save` hangs indefinitely — for PSD and for
PNG alike, whether the destination is under `/mnt` or under `$HOME`. It does not
respond to SIGTERM, so a plain `timeout` does not reclaim the process, and hung
instances accumulate (18 of them during this work, unkillable even unsandboxed).
Script-Fu's `catch` does not help, because the call **blocks rather than raising**.
So the `gimp-file-save` argument list could not be bisected; the three-argument
GIMP 3 form is what the script uses, with the alternatives to try documented inline
at `psdfix-save` — the single place to fix.

Two hazards to respect when someone picks this up:

1. **A Script-Fu error makes GIMP hang instead of exiting.** Build scripts
   incrementally, one procedure at a time.
2. **Always use `timeout -k`.** Plain `timeout` sends SIGTERM, which a hung GIMP
   ignores; `-k` follows up with SIGKILL. Clear stale instances with
   `pkill -9 -f snap/gimp` before a run.

GIMP is therefore **not** the layered writer. `generate_pytoshop.py` is — see below.
The scripts stay in the tree only for whoever wants to revisit GIMP.

### Do not hand-write expected blend modes

GIMP's PSD exporter maps only part of its blend-mode vocabulary and silently writes
`norm` for the rest. Do not predict the mapping: generate the fixture, run
`derive_expectations.py`, and read `psdBlendKey` out of the resulting sidecar. Any
mode that comes back as `norm` was not mapped and should be dropped from the
generator with a comment, rather than left in to produce a fixture that asserts the
wrong thing. (pytoshop does not have this problem — it writes the four-character key
it is given — but the rule stands for any writer.)

## pytoshop: the layered writer

```bash
.venv-psdfixtures/bin/python tools/psdfixtures/generate_pytoshop.py --out /tmp/fx
.venv-psdfixtures/bin/python tools/psdfixtures/generate_pytoshop.py --out /tmp/fx --only rgb8-clipping
.venv-psdfixtures/bin/python tools/psdfixtures/generate_pytoshop.py --list
```

pytoshop writes layer records directly, so it reaches the structures GIMP's exporter
hides: `lsct` section dividers, `pass` group blending, an offset layer-mask rect with
its own `-2` channel, the mask-disabled and invert flags, and the clipping flag.
Output is deterministic — regenerating produces byte-identical files.

`GroupNode` also takes a `mask`, which is emitted on the group's **opening** (`lsct`
1/2) record and never on the bounding divider, and the script's stored composite
folds a group mask into every descendant's alpha. An empty `children` list produces a
genuinely childless group: a bounding divider immediately followed by its folder
record.

**Layer and composite compression are separate knobs.** `build_psd` takes
`compression` for the layer channels and `composite_compression` for
`core.ImageData`, defaulting the second to the first. They are split because
Photoshop writes the merged composite raw or RLE and **never** ZIP, so the ZIP
fixtures pass `composite_compression=rle` and end up shaped like a real document
instead of like a synthetic one.

Each fixture is a function `fixture_<id_with_underscores>`, which is what
`provenance.generatedBy` points at.

### Six pytoshop 1.2.1 defects, all worked around in the script

These are defects in the *writer*. Each workaround makes pytoshop emit what Photoshop
emits; none of them is a concession to a particular reader. They are numbered in the
script and commented where they are patched.

1. **`luni` names carry a NUL and count it.** `util.encode_unicode_string` packs
   `len(s) + 1` and appends a UTF-16 terminator, so `Background` is declared as 11
   characters and every reader that honours the declared length hands back
   `"Background\x00"`. Patched, and the patch is *verified at startup* rather than
   assumed — the script refuses to write anything if the probe comes back wrong.
2. **RLE is entirely unavailable, not just broken for constant rows.**
   `codecs.py` does `from . import packbits` inside a bare `try/except ImportError:
   pass`; the Cython extension is not built in the wheel, so the name is simply
   absent and *every* RLE path raises `NameError: name 'packbits' is not defined` —
   `compress_rle` for any image, not only `compress_constant_rle`. The script
   supplies a small pure-Python PackBits encoder. It is proven by round trip: a
   fixture written twice, once raw and once RLE, decodes to identical arrays under
   psd-tools.
3. **Every layer gets a 36-byte mask block whether or not it has a mask.**
   `LayerRecord.mask` materialises an empty `LayerMask` on first access and
   `LayerMask.length()` is unconditionally 36, so maskless layers ship a `(0,0,0,0)`
   mask that a faithful reader reports as a real 0x0 mask. The script writes a
   four-byte length of 0 instead, as Photoshop does.
4. **`LayerRecord.channels` re-sorts and re-binds the channel dict.** The setter
   stores `OrderedDict(sorted(...))`, which (a) makes the record hold a *different*
   dict, so appending the `-2` mask channel after construction silently does nothing
   and the mask channel never reaches the file, and (b) orders the mask channel ahead
   of transparency and colour. The script preserves insertion order and builds the
   channel dict complete before constructing the record.
5. **ZIP-with-prediction writes unpredicted data, twice over.** `compress_zip` is
   fine — pure `zlib`, no `packbits`, so ZIP layer channels work out of the box and
   the corpus's old claim that "pytoshop offers raw and RLE only" was simply false.
   `compress_zip_prediction` is the broken one: it calls the absent
   `packbits.encode_prediction_8bit` (defect 2 again, so `NameError`), *and* passes
   the row as `row.flatten()` — a copy — while the encoder mutates in place, so even
   a built extension would write the untouched row. Shimming the encoder alone would
   therefore produce a file labelled `zip_prediction` carrying plain ZIP bytes, which
   asserts nothing; the script replaces the compressor outright. It patches
   `codecs.compressors[...]`, not the module-level name, because `compress_image`
   dispatches through the dict. The predictor is a per-row uint8 delta with
   wraparound, **reset at every row boundary**. (`decompress_zip_prediction` has the
   same `.flatten()` bug on the way back, which is one more reason the round trip is
   judged by psd-tools and not by pytoshop.)

6. **`GenericTaggedBlock.data`'s setter validates and then never assigns.** It
   checks `isinstance(val, bytes)` and falls off the end without touching
   `self._data`, so `block.data = payload` leaves the block *empty* — a fixture that
   looks right in the generator and asserts nothing in the file, the same shape of
   trap as defect 4. The payload goes through the constructor instead, and
   `_fill_opacity_block` fails the run if it did not stick.

Defects 2 and 5 are both proven by round trip rather than by inspection: the same
layer stack written raw, RLE, ZIP and ZIP-with-prediction must decode to identical
arrays under psd-tools, a reader this repository does not own.

Fixtures are painted with non-uniform content on purpose — the right-hand column is
darkened a step and its alpha nudged down — so that no channel row is a single
repeated value. That is what makes a sampled pixel informative, and it keeps the
constant-row code path out of the picture as a side effect.

## Adding a new fixture, end to end

1. **Write the generator.** Add a named function — `fixture_<id>` in
   `generate_imagemagick.sh` (flat fixtures) or `fixture_<id>` in
   `generate_pytoshop.py` (layered fixtures). The name is what
   `provenance.generatedBy` points at, so it has to be stable.

   Paint **several distinct colour regions**. A fixture in one flat colour lets a
   pixel sample taken from entirely the wrong coordinate pass, which makes the
   assertion worthless.

   Keep it small: under 24 KiB where possible, 128 KiB hard cap. The generator
   enforces both.

2. **Generate into a scratch directory** — never straight into the committed corpus:

   ```bash
   PY=.venv-psdfixtures/bin/python                 # see "Setting the toolchain up"
   just fixtures-generate /tmp/fx "$PY"
   # or: tools/psdfixtures/generate_imagemagick.sh /tmp/fx rgb8-flat-rle
   # or: "$PY" tools/psdfixtures/generate_pytoshop.py --out /tmp/fx --only rgb8-clipping
   ```

3. **Confirm the file is valid** before deriving anything from it:

   ```bash
   xxd /tmp/fx/my-fixture.psd | grep -o '8BIM....'   # expect norm, never mron
   "$PY" -c "from psd_tools import PSDImage; print(PSDImage.open('/tmp/fx/my-fixture.psd'))"
   ```

4. **Derive the sidecar:**

   ```bash
   "$PY" tools/psdfixtures/derive_expectations.py /tmp/fx/my-fixture.psd \
       --id my-fixture --out /tmp/fx \
       --source-tool ImageMagick --source-tool-version 6.9.12-98 \
       --generated-by 'tools/psdfixtures/generate_imagemagick.sh#fixture_my_fixture' \
       --description '16x16 RGB/8, four distinct colour quadrants, RLE.'
   ```

   Use `--out -` to print to stdout while iterating.

5. **Review the sidecar by hand.** Two fields cannot be derived and are a human
   decision:

   - **`warnings`** is always emitted as `[]`, meaning "must import clean".
     psd-tools cannot know which diagnostics *Agogo* ought to raise. Import the
     fixture and confirm the real warning set, then edit the field.
   - **`knownGaps`** is empty by default. Add entries with
     `--known-gap 'path=...;want=...;actual=...;reason=...'` for places where Agogo
     is *knowingly* wrong. The harness asserts `actual` and fails when the value
     reaches `want` — that failure means the gap closed and the sidecar is stale.

6. **Move the fixture and its sidecar into the corpus** under
   `packages/engine-wasm/internal/io/psdfixture/`, and register it in the manifest.

## Sidecar notes

- **`assert` and the data blocks are derived together, never written by hand.** The
  Go loader rejects both directions: a scope asserted without data, and data present
  without its scope asserted. `--assert` therefore *restricts which optional scopes
  are collected*; it cannot produce an inconsistent pairing. An optional scope with
  nothing in it is omitted entirely rather than written as `[]`.
- **Unknown JSON keys are a hard error** — the Go side decodes with
  `DisallowUnknownFields`. Emit exactly the contract's field names, nothing extra.
- **Layer order is bottom-to-top, and is NOT reversed.** PSD stores layer records
  bottom-to-top; psd-tools appends its API layers in raw record order; the engine
  stores children the same way. Verified empirically rather than assumed: in a file
  with two opaque full-canvas layers, the layer at record index 1 wins the
  composite, so index 1 is *above* index 0. `psd[0]` is the bottom layer.
- **`opacity255` is the raw byte**, 0..255, exactly as a non-Agogo reader printed it.
  The Go side converts at compare time (`float64(x)/255`).
- **Boundless nodes omit `bounds`** rather than emitting a zero rect, which would
  read as a real mismatch instead of "nothing asserted".
- **Output is Biome-stable.** Biome formats every `*.json` in this repo and
  `just ci` runs `treefmt --fail-on-change`, so `derive_expectations.py` emits what
  Biome would emit — scalar arrays inlined when they fit 100 columns. Plain
  `json.dumps(indent=2)` does **not** satisfy this and would break CI.
- `writer.externalVerification.result` is emitted as `"pending"`. It is a claim about
  an external check that has not run yet; promote it to `"pass"` only after
  `just fixtures-verify` actually passes.

## Verifying Agogo's own output

`verify_dump.py` closes the round trip from the outside. The Go side re-exports
fixtures into `packages/engine-wasm/internal/io/psdfixture/_dump/`, and this reads
every file there with psd-tools — and, with `--identify`, with ImageMagick as a third
independent implementation:

```bash
just fixtures-verify                                      # add a second argument
just fixtures-verify <dir> .venv-psdfixtures/bin/python   # to use the pinned venv
# or: .venv-psdfixtures/bin/python tools/psdfixtures/verify_dump.py <dir> --identify
```

It exits non-zero if psd-tools cannot parse a file at all, which would mean Agogo
emitted something no other reader accepts. It does **not** judge structural
differences against the source fixture — that is the sidecar's `writer` scope. This
script reports; the harness and a human decide.

## Licensing

Every fixture is **self-authored from scratch** and contains **no third-party
artwork**: solid colour blocks drawn by the generator scripts in this directory,
nothing else. They are published under **CC0-1.0** so they can be redistributed with
the repository without encumbering it, and each sidecar records that basis in its
`provenance` block (`license`, `licenseNote`, `author`, `redistributable: true`).

Do not add a fixture sourced from a real-world PSD, a stock asset, or anyone else's
document. Beyond the licensing problem, such a file cannot be regenerated, which
defeats the point of keeping the generators under version control.
