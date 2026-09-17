#!/usr/bin/env python3
"""generate_pytoshop.py — generate the LAYERED PSD fixtures with pytoshop.

Phase S.10.1, step 2. See README.md for the corpus rationale.

Independence
------------
The corpus rests on ``generator != deriver != Agogo``:

    pytoshop  ──>  fixture.psd  ──>  psd-tools  ──>  fixture.expected.json
   (writes bytes)                  (reads bytes)         (the sidecar)

So this file is a WRITER and nothing else. It must never import psd-tools, and it
must never import, exec or shell out to anything under ``packages/engine-wasm``.
If a fixture looks wrong, fix the bytes here — never reach for a reader to make a
number agree.

This replaces the GIMP route for layered fixtures. GIMP 3.2.6 is unusable in batch
mode: every ``gimp-file-save`` blocks forever and ignores SIGTERM. pytoshop writes
the same structures directly and deterministically.

Usage
-----
    generate_pytoshop.py --out <dir> [--only <id> ...]

Each fixture is a function named ``fixture_<id_with_underscores>`` so that a
sidecar's ``provenance.generatedBy`` can point at it precisely, e.g.

    "generatedBy": "tools/psdfixtures/generate_pytoshop.py#fixture_rgb8_nested_groups"

Six pytoshop 1.2.1 defects are worked around here. They are defects in the
writer, not in any reader: see ``_encode_unicode_string_no_nul``, ``_PackBits``,
``_AbsentLayerMask``, ``_LayerRecord`` and ``_ZipPrediction`` at the top of this
file, and ``_fill_opacity_block`` further down.
"""

from __future__ import annotations

import argparse
import struct
import sys
import zlib
from collections import OrderedDict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable, Sequence

try:
    import numpy as np
    import pytoshop
    import pytoshop.codecs
    import pytoshop.util
    from pytoshop import core, enums, image_resources, layers, tagged_block
except ImportError as exc:  # pragma: no cover - environment problem, not logic
    sys.exit(f"error: {exc}. Install with: pip install pytoshop")

SCRIPT_NAME = Path(__file__).name

# Fixtures are committed to the repository, so they stay tiny.
SOFT_CAP_BYTES = 24576  # 24 KiB — warn above this
HARD_CAP_BYTES = 131072  # 128 KiB — fail above this


def die(message: str) -> None:
    sys.exit(f"{SCRIPT_NAME}: error: {message}")


def log(message: str) -> None:
    print(f"{SCRIPT_NAME}: {message}", file=sys.stderr)


# ══ pytoshop defect workarounds ═══════════════════════════════════════════════
#
# All six are verified against pytoshop 1.2.1. Each one makes pytoshop emit what
# Photoshop emits; none of them is a concession to any particular reader. Five
# live here; defect 6 is at its only call site, ``_fill_opacity_block``.


# Defect 1 — `luni` layer names carry a NUL and count it.
#
# pytoshop.util.encode_unicode_string packs `len(s) + 1` and appends a UTF-16 NUL,
# so a layer called "Background" declares 11 characters and every reader that
# honours the declared length hands back "Background\x00". Photoshop declares the
# character count WITHOUT a terminator. Patch the writer, not the readers.
def _encode_unicode_string_no_nul(s: str) -> bytes:
    return struct.pack(">L", len(s)) + s.encode("utf_16_be")


pytoshop.util.encode_unicode_string = _encode_unicode_string_no_nul


def _assert_luni_patch_applied() -> None:
    """Fail loudly if the `luni` patch did not take effect.

    tagged_block.py resolves the function through the module (``util.encode_...``)
    rather than binding it at import time, so the patch does stick — but that is an
    implementation detail of a third-party package, and a fixture corpus with NUL-
    terminated layer names is worse than no corpus at all. So: verify, do not hope.
    """
    probe = tagged_block.UnicodeLayerName(name="Xy")
    blob = b""

    class _Sink:
        def write(self, data: bytes) -> None:
            nonlocal blob
            blob += data

    probe.write_data(_Sink(), None)
    expected = struct.pack(">L", 2) + "Xy".encode("utf_16_be")
    if blob != expected:
        die(
            "the pytoshop.util.encode_unicode_string patch did not take effect "
            f"(luni payload was {blob!r}, expected {expected!r}); patch the name "
            "where pytoshop.tagged_block resolves it and try again"
        )


# Defect 2 — pytoshop cannot RLE-compress anything at all.
#
# pytoshop/codecs.py does `from . import packbits` inside a bare `try/except
# ImportError: pass`. `packbits` is a Cython extension that is NOT built in the
# installed wheel, so the name is simply absent and EVERY rle path raises
# `NameError: name 'packbits' is not defined` — both compress_rle (any image) and
# compress_constant_rle (a channel row of one repeated value). The advertised
# "constant images crash under RLE" is the narrow case; the general case is worse.
#
# PackBits is a dozen lines, so supply it rather than dropping RLE coverage. This
# is generator code: psd-tools decoding the result is what proves it correct.
class _PackBits:
    """Minimal Adobe PackBits encoder, standing in for pytoshop's missing C ext."""

    @staticmethod
    def encode(data: bytes | np.ndarray) -> bytes:
        if isinstance(data, np.ndarray):
            data = np.ascontiguousarray(data).tobytes()
        else:
            data = bytes(data)

        out = bytearray()
        total = len(data)
        index = 0
        while index < total:
            # A replicate run: 2..128 identical bytes -> (257 - n, byte).
            run = 1
            while index + run < total and data[index + run] == data[index] and run < 128:
                run += 1
            if run >= 2:
                out.append(257 - run)
                out.append(data[index])
                index += run
                continue

            # A literal run: 1..128 bytes -> (n - 1, bytes). Stops as soon as a
            # replicate run becomes available, which is why the loop below peeks.
            start = index
            literal = 0
            while index < total and literal < 128:
                ahead = 1
                while index + ahead < total and data[index + ahead] == data[index] and ahead < 2:
                    ahead += 1
                if ahead >= 2:
                    break
                index += 1
                literal += 1
            out.append(literal - 1)
            out += data[start : start + literal]
        return bytes(out)


if not hasattr(pytoshop.codecs, "packbits"):
    pytoshop.codecs.packbits = _PackBits  # type: ignore[attr-defined]


# Defect 3 — every layer gets a 36-byte layer-mask block whether or not it has a mask.
#
# LayerRecord.mask is a property that materialises an empty LayerMask on first
# access, and LayerMask.length() is unconditionally 36. So a maskless layer is
# written with a mask rect of (0,0,0,0), and a faithful reader reports "this layer
# has a 0x0 mask". Photoshop writes a four-byte length of 0 instead. Assign one of
# these to `record.mask` to get that.
class _AbsentLayerMask(layers.LayerMask):
    def length(self, header) -> int:  # noqa: ANN001 - mirrors pytoshop's signature
        return 0

    def total_length(self, header) -> int:  # noqa: ANN001
        return 4

    def write(self, fd, header) -> None:  # noqa: ANN001
        pytoshop.util.write_value(fd, "I", 0)


# Defect 4 — LayerRecord.channels re-sorts, and re-binds, the channel dict.
#
# The setter does `self._channels = OrderedDict(sorted(value.items()))`. Two
# consequences, both of which cost a fixture before they were caught:
#
#   * the dict stored on the record is a NEW object, so mutating the dict you
#     passed in (to append the -2 mask channel after construction) silently does
#     nothing and the mask channel never reaches the file;
#   * sorting by channel id puts the user mask (-2) ahead of the transparency
#     channel (-1) and the colour channels. Photoshop writes them in occurrence
#     order: -1, 0, 1, 2, then -2.
#
# Preserve insertion order and keep the caller's dict identity.
class _LayerRecord(layers.LayerRecord):
    @property
    def channels(self):  # type: ignore[override]
        return self._channels

    @channels.setter
    def channels(self, value) -> None:  # noqa: ANN001
        if not isinstance(value, dict):
            raise TypeError("channels must be a dict")
        for key, val in value.items():
            enums.ChannelId(key)
            if not isinstance(val, layers.ChannelImageData):
                raise ValueError("Each channel must be a ChannelImageData instance")
        self._channels = value if isinstance(value, OrderedDict) else OrderedDict(value)


# Defect 5 — ZIP-with-prediction writes UNPREDICTED data, for two separate reasons.
#
# `codecs.compress_zip` needs none of this: it is pure zlib (`zlib.compress` /
# `zlib.compressobj`) and never touches the missing `packbits` extension, so ZIP
# layer channels work out of the box. `compress_zip_prediction` is the broken one,
# and it is broken twice over:
#
#   * it reaches for `packbits.encode_prediction_8bit`, which is absent for the same
#     reason RLE is (defect 2), so the call raises `NameError`;
#   * and even with the extension built it would be a no-op, because the row is
#     passed as `row.flatten()`. `flatten()` ALWAYS copies, the Cython encoder
#     mutates its argument in place, and the copy is then discarded — the row that
#     actually reaches `compressor.compress(row)` is the untouched original. (The
#     decoder, `decompress_zip_prediction`, has the identical `.flatten()` bug.)
#
# So a shimmed encoder alone would still emit a file labelled `zip_prediction`
# carrying plain ZIP bytes — a fixture that silently asserts nothing. Replace the
# compressor outright instead.
#
# The predictor itself is trivial and is specified by the format, not by pytoshop:
# a horizontal uint8 delta, `out[i] = in[i] - in[i-1]` with wraparound, RESET AT
# EVERY ROW BOUNDARY so the first byte of each row is stored verbatim. The reset is
# the whole point of the fixture — a decoder that runs one cumulative sum across the
# entire plane instead of per row decodes row 0 correctly and every later row wrong.
#
# Correctness is proven by round trip, exactly as for defect 2: the same fixture
# written raw and written zip-with-prediction must decode to identical arrays under
# psd-tools, which is a reader this repository does not own.
class _ZipPrediction:
    """The 8-bit horizontal predictor pytoshop cannot apply."""

    @staticmethod
    def encode_row(row: np.ndarray) -> np.ndarray:
        """One row's uint8 deltas, with wraparound; the first byte stays verbatim."""
        row = np.ascontiguousarray(row, dtype=np.uint8)
        out = np.empty_like(row)
        out[0] = row[0]
        if row.shape[0] > 1:
            # uint8 subtraction wraps in numpy, which is the wraparound the format
            # requires; do the whole tail at once rather than byte by byte.
            out[1:] = row[1:] - row[:-1]
        return out

    @staticmethod
    def compress(fd, image, depth, version) -> None:  # noqa: ANN001 - codecs signature
        """Replacement for codecs.compress_zip_prediction: predict per row, then zlib."""
        if depth != 8:
            die(f"the zip-prediction workaround only implements 8-bit, got depth {depth}")
        rows = np.atleast_2d(np.ascontiguousarray(image, dtype=np.uint8))
        compressor = zlib.compressobj()
        for row in rows:
            fd.write(compressor.compress(_ZipPrediction.encode_row(row).tobytes()))
        fd.write(compressor.flush())


# codecs.compress_image dispatches through the `compressors` DICT, not through the
# module-level name, so rebinding `pytoshop.codecs.compress_zip_prediction` alone
# would be silently ignored. Patch the dict entry, and the name too for anything
# that reads it directly.
pytoshop.codecs.compress_zip_prediction = _ZipPrediction.compress  # type: ignore[assignment]
pytoshop.codecs.compressors[enums.Compression.zip_prediction] = _ZipPrediction.compress


# ══ canvas and palette ════════════════════════════════════════════════════════

CANVAS_W = 32
CANVAS_H = 24

# Distinct, memorable colours. Every fixture paints several of them: a fixture in
# one flat colour lets a pixel sample taken from a completely wrong coordinate
# pass, which makes the assertion worthless.
BLUE = (0x33, 0x66, 0xCC)
CRIMSON = (0xCC, 0x33, 0x66)
LIME = (0x66, 0xCC, 0x33)
AMBER = (0xEE, 0xEE, 0x11)
VIOLET = (0x77, 0x33, 0xCC)
TEAL = (0x22, 0xAA, 0xAA)
ORANGE = (0xEE, 0x77, 0x11)
SLATE = (0x33, 0x44, 0x55)
CHALK = (0xF0, 0xF0, 0xE8)


def quadrants(
    width: int,
    height: int,
    top_left: tuple[int, int, int],
    top_right: tuple[int, int, int],
    bottom_left: tuple[int, int, int],
    bottom_right: tuple[int, int, int],
    alpha: int = 255,
) -> np.ndarray:
    """An RGBA plane painted in four distinct quadrants.

    The right-hand column is darkened by one step and its alpha nudged down. That
    is not decoration: it guarantees no channel row is a single repeated value,
    which is what makes a sampled pixel informative — and, incidentally, what keeps
    pytoshop's constant-row RLE path out of the picture entirely.
    """
    plane = np.zeros((height, width, 4), dtype=np.uint8)
    half_x = max(1, width // 2)
    half_y = max(1, height // 2)
    plane[:half_y, :half_x, :3] = top_left
    plane[:half_y, half_x:, :3] = top_right
    plane[half_y:, :half_x, :3] = bottom_left
    plane[half_y:, half_x:, :3] = bottom_right
    plane[:, :, 3] = alpha
    _break_uniform_rows(plane)
    return plane


def bands(
    width: int,
    height: int,
    colors: Sequence[tuple[int, int, int]],
    alpha: int = 255,
) -> np.ndarray:
    """An RGBA plane painted in horizontal bands, one per colour."""
    plane = np.zeros((height, width, 4), dtype=np.uint8)
    step = max(1, height // len(colors))
    for index, color in enumerate(colors):
        top = index * step
        bottom = height if index == len(colors) - 1 else min(height, top + step)
        plane[top:bottom, :, :3] = color
    plane[:, :, 3] = alpha
    _break_uniform_rows(plane)
    return plane


def _break_uniform_rows(plane: np.ndarray) -> None:
    if plane.shape[1] < 2:
        return
    plane[:, -1, :3] = np.clip(plane[:, -1, :3].astype(np.int16) - 0x11, 0, 255).astype(np.uint8)
    plane[:, -1, 3] = np.clip(plane[:, -1, 3].astype(np.int16) - 1, 0, 255).astype(np.uint8)


def mask_ramp(width: int, height: int) -> np.ndarray:
    """A single-channel mask plane: a diagonal ramp, deliberately non-uniform."""
    ys = np.arange(height, dtype=np.int32).reshape(height, 1)
    xs = np.arange(width, dtype=np.int32).reshape(1, width)
    ramp = (16 + (xs * 15) + (ys * 9)) % 256
    return ramp.astype(np.uint8)


# ══ layer model ═══════════════════════════════════════════════════════════════
#
# Nodes are given TOP-TO-BOTTOM, the way a layers palette reads. PSD stores layer
# records bottom-to-top, and the flattener reverses at the end — the same shape as
# pytoshop's own nested_layers, which is the path proven to round-trip.


@dataclass
class MaskSpec:
    """A raster layer mask. Its rect is independent of the layer's rect."""

    top: int
    left: int
    bottom: int
    right: int
    data: np.ndarray  # (height, width) uint8, matching THIS rect, not the layer's
    default_color: bool = False
    disabled: bool = False
    inverted: bool = False

    @property
    def width(self) -> int:
        return self.right - self.left

    @property
    def height(self) -> int:
        return self.bottom - self.top


@dataclass
class ImageNode:
    name: str
    top: int
    left: int
    pixels: np.ndarray  # (height, width, 4) uint8 RGBA
    blend_mode: bytes = enums.BlendMode.normal
    opacity: int = 255
    visible: bool = True
    clipping: bool = False
    mask: MaskSpec | None = None
    # Fill opacity (the iOpa tagged block). 255 writes no block at all, which is
    # what its absence means in the format.
    fill_opacity: int = 255

    @property
    def height(self) -> int:
        return int(self.pixels.shape[0])

    @property
    def width(self) -> int:
        return int(self.pixels.shape[1])

    @property
    def bottom(self) -> int:
        return self.top + self.height

    @property
    def right(self) -> int:
        return self.left + self.width


@dataclass
class AdjustmentNode:
    """A live adjustment layer: tagged blocks and no pixels.

    Photoshop writes an adjustment layer as a layer record with an EMPTY rect and
    no colour channels; the adjustment lives entirely in its tagged blocks. That
    shape is the point of the fixture — an importer that assumes every non-group
    record has a raster fails on it.

    `visible` defaults to False on purpose. The composite stored in a PSD is a
    flattened preview, and this generator computes that preview itself from the
    layer stack; it does not implement Photoshop's adjustment maths. A VISIBLE
    adjustment would therefore produce a preview contradicting the layer stack,
    which is the one thing this generator must never do (see
    _visible_images_bottom_to_top). Hidden, the two agree, and the block, the
    parameters and the record's own flags are still asserted in full.
    """

    name: str
    blocks: list  # [(four-character key, payload bytes)], in write order
    blend_mode: bytes = enums.BlendMode.normal
    opacity: int = 255
    visible: bool = False
    clipping: bool = False
    mask: MaskSpec | None = None
    fill_opacity: int = 255


@dataclass
class GroupNode:
    name: str
    children: list = field(default_factory=list)
    blend_mode: bytes = enums.BlendMode.normal
    opacity: int = 255
    visible: bool = True
    closed: bool = False
    clipping: bool = False
    # Fill opacity (the iOpa tagged block) on the group's OPENING record, never
    # on the bounding divider. Like the group mask it attenuates the whole
    # group's result, so the composite folds it into every descendant's alpha.
    fill_opacity: int = 255
    # A group carries its mask on the OPENING (lsct 1/2) record, never on the
    # bounding divider. The mask attenuates the whole group's result, so the
    # composite folds it into every descendant's alpha.
    mask: MaskSpec | None = None


# ══ flattening to layer records ═══════════════════════════════════════════════


def _channels(pixels: np.ndarray, compression: int) -> "OrderedDict[int, layers.ChannelImageData]":
    """Channel planes in Photoshop's occurrence order: transparency, then R, G, B."""
    channels: OrderedDict[int, layers.ChannelImageData] = OrderedDict()
    for channel_id, plane_index in (
        (enums.ChannelId.transparency, 3),
        (enums.ChannelId.red, 0),
        (enums.ChannelId.green, 1),
        (enums.ChannelId.blue, 2),
    ):
        plane = np.ascontiguousarray(pixels[:, :, plane_index])
        channels[channel_id] = layers.ChannelImageData(image=plane, compression=compression)
    return channels


def _apply_mask(
    name: str,
    spec: "MaskSpec | None",
    channels: "OrderedDict[int, layers.ChannelImageData]",
    compression: int,
) -> layers.LayerMask:
    """Build the mask block and append its -2 channel, for an image or a group.

    Returns the block to assign to record.mask. The channel must be added BEFORE
    the record is constructed — see defect 4 in the README.
    """
    if spec is None:
        return _AbsentLayerMask()
    if spec.data.shape != (spec.height, spec.width):
        die(
            f"{name}: mask channel is {spec.data.shape} but the mask rect is "
            f"{(spec.height, spec.width)} — the -2 channel must match the MASK rect, "
            "not the layer rect"
        )
    channels[enums.ChannelId.user_layer_mask] = layers.ChannelImageData(
        image=np.ascontiguousarray(spec.data), compression=compression
    )
    return layers.LayerMask(
        top=spec.top,
        left=spec.left,
        bottom=spec.bottom,
        right=spec.right,
        default_color=spec.default_color,
        layer_mask_disabled=spec.disabled,
        invert_layer_mask_when_blending=spec.inverted,
    )

# Defect 6 — `GenericTaggedBlock.data`'s setter validates and then never assigns.
#
# `tagged_block.py:183-186` checks `isinstance(val, bytes)` and falls off the end
# without touching `self._data`, so `block.data = payload` silently writes an
# EMPTY block: a fixture that looks right in the generator and asserts nothing in
# the file. Pass the payload to the constructor, and check that it stuck.
def _fill_opacity_block(value: int) -> tagged_block.GenericTaggedBlock:
    """The iOpa block, as psd-tools writes it: the byte plus three pad bytes.

    psd-tools reads iOpa as "B3x" (a byte and three pad bytes) and only falls
    back to a bare byte after logging a read error, and it writes the four-byte
    form itself. Four bytes is therefore the representative shape, and being even
    it also sidesteps the layer-record padding disagreement entirely.
    """
    if not 0 <= value <= 255:
        die(f"fill opacity {value} is not a byte")
    payload = bytes([value, 0, 0, 0])
    block = tagged_block.GenericTaggedBlock(code=b"iOpa", data=payload)
    if block.data != payload:
        die("pytoshop dropped the iOpa payload — see defect 6 above")
    return block


# ══ adjustment payloads ═══════════════════════════════════════════════════════
#
# Hand-built from the format spec, deliberately. psd-tools could produce these
# structures in a couple of lines, but psd-tools is the DERIVER: if it wrote the
# bytes as well as read them, the sidecars would assert nothing but its own
# self-consistency. Writing them here from the spec keeps generator and deriver
# two independent implementations, and a layout error shows up as psd-tools
# refusing to parse rather than as a fixture that agrees with itself.
#
# Several of these structures are padded to a 4-byte boundary, which is what
# Photoshop writes and what psd-tools' own writers emit. Every payload below is
# even-length as a result, so none of them meets the layer-record padding
# disagreement documented in testdata/README.md.


def _pad4(payload: bytes) -> bytes:
    return payload + b"\x00" * (-len(payload) % 4)


def _levl(records: Sequence[tuple[int, int, int, int, int]]) -> bytes:
    """levl: version 2, then exactly 29 level records of five uint16.

    Record order is composite, red, green, blue, then unused. Gamma is a short
    from 10..999 standing for 0.1..9.99, so 120 means 1.2.
    """
    out = struct.pack(">H", 2)
    table = list(records) + [(0, 255, 0, 255, 100)] * (29 - len(records))
    if len(table) != 29:
        die(f"levl needs at most 29 records, got {len(records)}")
    for floor, ceiling, out_floor, out_ceiling, gamma in table:
        out += struct.pack(">5H", floor, ceiling, out_floor, out_ceiling, gamma)
    return _pad4(out)


def _curv(channels: dict[int, Sequence[tuple[int, int]]]) -> bytes:
    """curv: version 1, a channel bitmap, then per channel (y, x) point pairs.

    Version 4 drops the bitmap and with it the channel identity, so version 1 is
    the only form that can carry a per-channel curve. Photoshop follows the
    bitmapped section with a 'Crv ' extra marker repeating each curve with an
    explicit channel id; psd-tools logs "Failed to read CurvesExtraMarker" when
    it is absent, so it is written here too.
    """
    out = struct.pack(">BHI", 0, 1, sum(1 << channel for channel in channels))
    for _, points in sorted(channels.items()):
        out += struct.pack(">H", len(points))
        for y, x in points:
            out += struct.pack(">2H", y, x)
    out += struct.pack(">4sHI", b"Crv ", 4, len(channels))
    for channel, points in sorted(channels.items()):
        out += struct.pack(">2H", channel, len(points))
        for y, x in points:
            out += struct.pack(">2H", y, x)
    return _pad4(out)


def _hue2(
    master: tuple[int, int, int],
    items: Sequence[tuple[tuple[int, int, int, int], tuple[int, int, int]]],
    colorization: tuple[int, int, int] = (0, 0, 0),
    enable: int = 1,
) -> bytes:
    """hue2: version 2, the colorize triple, the master triple, then six ranges.

    Each range is four int16 hue edges followed by its own hue/saturation/
    lightness triple. The six are reds, yellows, greens, cyans, blues, magentas.
    """
    if len(items) != 6:
        die(f"hue2 needs exactly 6 ranges, got {len(items)}")
    out = struct.pack(">HBx", 2, enable)
    out += struct.pack(">3h", *colorization)
    out += struct.pack(">3h", *master)
    for edges, settings in items:
        out += struct.pack(">4h", *edges)
        out += struct.pack(">3h", *settings)
    return _pad4(out)


def _brit(brightness: int, contrast: int, mean: int = 127, lab_only: int = 0) -> bytes:
    """brit: the OBSOLETE brightness/contrast block, written for old readers.

    psd-tools maps its BrightnessContrast layer to CgEd and marks this tag
    obsolete. Photoshop still writes both, so a faithful fixture carries both and
    the live values belong in the descriptor - see _cged.
    """
    return struct.pack(">3HBx", brightness & 0xFFFF, contrast & 0xFFFF, mean, lab_only)


def _blnc(
    shadows: tuple[int, int, int],
    midtones: tuple[int, int, int],
    highlights: tuple[int, int, int],
    luminosity: int = 0,
) -> bytes:
    """blnc: three tone triples of int16 (cyan-red, magenta-green, yellow-blue)."""
    out = struct.pack(">3h", *shadows)
    out += struct.pack(">3h", *midtones)
    out += struct.pack(">3h", *highlights)
    out += struct.pack(">B", luminosity)
    return _pad4(out)


def _mixr(monochrome: int, data: Sequence[int]) -> bytes:
    """mixr: version, the monochrome flag, then five int16 for ONE output row.

    Four of the five are the source weights in percent and the fifth is the
    constant. Photoshop repeats the block per output channel.
    """
    if len(data) != 5:
        die(f"mixr needs 5 values, got {len(data)}")
    return struct.pack(">2H", 1, monochrome) + struct.pack(">5h", *data)


def _selc(method: int, plates: Sequence[tuple[int, int, int, int]]) -> bytes:
    """selc: version, the relative/absolute method, then ten CMYK plates.

    The ten are reds, yellows, greens, cyans, blues, magentas, whites, neutrals,
    blacks - with one unused leading plate.
    """
    if len(plates) != 10:
        die(f"selc needs 10 plates, got {len(plates)}")
    out = struct.pack(">2H", 1, method)
    for plate in plates:
        out += struct.pack(">4h", *plate)
    return out


def _thrs(level: int) -> bytes:
    """thrs: one uint16 level, padded to four bytes."""
    return struct.pack(">H2x", level)


def _post(levels: int) -> bytes:
    """post: one uint16 level count, padded to four bytes."""
    return struct.pack(">H2x", levels)


def _nvrt() -> bytes:
    """nvrt: no payload at all. Invert has nothing to configure."""
    return b""


def _phfl(
    color_space: int,
    components: tuple[int, int, int, int],
    density: int,
    luminosity: int,
) -> bytes:
    """phfl version 2: a colour space, four uint16 components, density, luminosity.

    Version 3 stores XYZ instead. Version 2 is the widely written one and is the
    form psd-tools emits.
    """
    out = struct.pack(">H", 2)
    out += struct.pack(">H4H", color_space, *components)
    out += struct.pack(">IB", density, luminosity)
    return _pad4(out)


# ── descriptor-valued blocks ──────────────────────────────────────────────────


def _descriptor_unicode(text: str) -> bytes:
    """A descriptor unicode string: a character count INCLUDING the NUL, then UTF-16BE."""
    return struct.pack(">I", len(text) + 1) + text.encode("utf-16-be") + b"\x00\x00"


def _length_and_key(key: str) -> bytes:
    """The length-or-4CC form: a zero length means a bare four-character key follows."""
    raw = key.encode("ascii")
    if len(raw) == 4:
        return b"\x00\x00\x00\x00" + raw
    return struct.pack(">I", len(raw)) + raw


def _descriptor(class_id: str, items: Sequence[tuple[str, bytes]], name: str = "") -> bytes:
    out = _descriptor_unicode(name) + _length_and_key(class_id)
    out += struct.pack(">I", len(items))
    for key, value in items:
        out += _length_and_key(key) + value
    return out


def _v_long(value: int) -> bytes:
    return b"long" + struct.pack(">i", value)


def _v_bool(value: bool) -> bytes:
    return b"bool" + struct.pack(">B", 1 if value else 0)


def _descriptor_block(class_id: str, items: Sequence[tuple[str, bytes]]) -> bytes:
    """A descriptor-valued tagged block: uint32 version 16, then the descriptor."""
    return struct.pack(">I", 16) + _descriptor(class_id, items)


def _blwh(
    reds: int, yellows: int, greens: int, cyans: int, blues: int, magentas: int,
    use_tint: bool = False,
) -> bytes:
    """blwh: the black-and-white mix, as a descriptor of six long percentages."""
    return _descriptor_block(
        "null",
        [
            ("Rd  ", _v_long(reds)),
            ("Yllw", _v_long(yellows)),
            ("Grn ", _v_long(greens)),
            ("Cyn ", _v_long(cyans)),
            ("Bl  ", _v_long(blues)),
            ("Mgnt", _v_long(magentas)),
            ("useTint", _v_bool(use_tint)),
        ],
    )


def _cged(
    brightness: int, contrast: int, mean: int = 127,
    lab: bool = False, use_legacy: bool = False,
) -> bytes:
    """CgEd: the LIVE brightness/contrast values, which `brit` no longer carries."""
    return _descriptor_block(
        "null",
        [
            ("Vrsn", _v_long(1)),
            ("Brgh", _v_long(brightness)),
            ("Cntr", _v_long(contrast)),
            ("means", _v_long(mean)),
            ("Lab ", _v_bool(lab)),
            ("useLegacy", _v_bool(use_legacy)),
        ],
    )


def _adjustment_block(code: str, payload: bytes) -> tagged_block.GenericTaggedBlock:
    """One adjustment tagged block, with defect 6's post-construction check."""
    raw = code.encode("ascii")
    if len(raw) != 4:
        die(f"tagged block key {code!r} is not four characters")
    block = tagged_block.GenericTaggedBlock(code=raw, data=payload)
    if block.data != payload:
        die(f"pytoshop dropped the {code} payload — see defect 6 above")
    return block


def _image_record(node: ImageNode, compression: int, layer_id: int) -> layers.LayerRecord:
    channels = _channels(node.pixels, compression)
    mask_block = _apply_mask(node.name, node.mask, channels, compression)

    record = _LayerRecord(
        top=node.top,
        left=node.left,
        bottom=node.bottom,
        right=node.right,
        name=node.name,
        blend_mode=node.blend_mode,
        opacity=node.opacity,
        visible=node.visible,
        clipping=node.clipping,
        channels=channels,
        blocks=[
            tagged_block.UnicodeLayerName(name=node.name),
            tagged_block.LayerId(id=layer_id),
        ],
    )
    if node.fill_opacity != 255:
        record.blocks.insert(1, _fill_opacity_block(node.fill_opacity))
    record.mask = mask_block
    return record


def _adjustment_record(
    node: AdjustmentNode, compression: int, layer_id: int
) -> layers.LayerRecord:
    """An adjustment layer record: empty rect, no colour channels, blocks only."""
    channels: "OrderedDict[int, layers.ChannelImageData]" = OrderedDict()
    mask_block = _apply_mask(node.name, node.mask, channels, compression)

    blocks = [tagged_block.UnicodeLayerName(name=node.name)]
    if node.fill_opacity != 255:
        blocks.append(_fill_opacity_block(node.fill_opacity))
    blocks.append(tagged_block.LayerId(id=layer_id))
    blocks.extend(_adjustment_block(code, payload) for code, payload in node.blocks)

    record = _LayerRecord(
        name=node.name,
        blend_mode=node.blend_mode,
        opacity=node.opacity,
        visible=node.visible,
        clipping=node.clipping,
        channels=channels,
        blocks=blocks,
    )
    record.mask = mask_block
    return record


def _group_records(
    node: GroupNode, compression: int, next_id: Callable[[], int]
) -> list[layers.LayerRecord]:
    """The opening record, the children, and the closing bounding divider.

    Emitted top-to-bottom; the caller reverses the whole list once at the end, which
    puts the bounding divider below the children and the header above them — the
    order Photoshop writes.
    """
    divider = (
        enums.SectionDividerSetting.closed if node.closed else enums.SectionDividerSetting.open
    )
    header_id = next_id()
    header_channels: "OrderedDict[int, layers.ChannelImageData]" = OrderedDict()
    header_mask = _apply_mask(node.name, node.mask, header_channels, compression)
    header = _LayerRecord(
        name=node.name,
        blend_mode=node.blend_mode,
        opacity=node.opacity,
        visible=node.visible,
        clipping=node.clipping,
        pixel_data_irrelevant=True,
        channels=header_channels,
        blocks=[
            tagged_block.UnicodeLayerName(name=node.name),
            tagged_block.SectionDividerSetting(type=divider),
            tagged_block.LayerId(id=header_id),
        ],
    )
    if node.fill_opacity != 255:
        header.blocks.insert(1, _fill_opacity_block(node.fill_opacity))
    header.mask = header_mask

    records = [header]
    records.extend(_flatten(node.children, compression, next_id))

    bounding = _LayerRecord(
        name="</Layer group>",
        pixel_data_irrelevant=True,
        blocks=[
            tagged_block.UnicodeLayerName(name="</Layer group>"),
            tagged_block.SectionDividerSetting(type=enums.SectionDividerSetting.bounding),
            tagged_block.LayerNameSource(id=header_id),
        ],
    )
    bounding.mask = _AbsentLayerMask()
    records.append(bounding)
    return records


def _flatten(
    nodes: Sequence[ImageNode | AdjustmentNode | GroupNode], compression: int, next_id: Callable[[], int]
) -> list[layers.LayerRecord]:
    records: list[layers.LayerRecord] = []
    for node in nodes:
        if isinstance(node, GroupNode):
            records.extend(_group_records(node, compression, next_id))
        elif isinstance(node, AdjustmentNode):
            records.append(_adjustment_record(node, compression, next_id()))
        else:
            records.append(_image_record(node, compression, next_id()))
    return records


# ══ composite ═════════════════════════════════════════════════════════════════


def _visible_images_bottom_to_top(
    nodes: Sequence[ImageNode | AdjustmentNode | GroupNode],
    width: int = CANVAS_W,
    height: int = CANVAS_H,
    inherited: np.ndarray | None = None,
) -> list[tuple[ImageNode, np.ndarray | None]]:
    """Visible image layers bottom-to-top, each paired with its inherited coverage.

    A group contributes three things to everything inside it: its mask, its
    opacity and its fill opacity. All three attenuate the group's whole result,
    so they fold into one inherited coverage plane and multiply into each
    descendant's alpha exactly like the descendant's own mask would. Carrying
    the mask alone would let a group-opacity or group-fill fixture write the
    block while the stored composite ignored it - a preview contradicting the
    layer stack, which is the one thing this generator must never produce.
    """
    out: list[tuple[ImageNode, np.ndarray | None]] = []
    for node in reversed(list(nodes)):
        if isinstance(node, GroupNode):
            if not node.visible:
                continue
            plane = _mask_plane(node.mask, width, height)
            if plane is None:
                combined = inherited
            elif inherited is None:
                combined = plane
            else:
                combined = inherited * plane / 255.0
            factor = (node.opacity / 255.0) * (node.fill_opacity / 255.0)
            if factor != 1.0:
                if combined is None:
                    combined = np.full((height, width), 255.0, dtype=np.float32)
                combined = combined * factor
            out.extend(_visible_images_bottom_to_top(node.children, width, height, combined))
        elif isinstance(node, AdjustmentNode):
            # An adjustment layer contributes no pixels of its own, and this
            # generator does not implement Photoshop's adjustment maths, so it
            # cannot contribute the change it would make to the layers below
            # either. AdjustmentNode.visible therefore defaults to False and
            # this arm refuses the case where it is not - silently compositing
            # around a visible adjustment would store a preview that contradicts
            # the layer stack.
            if node.visible:
                die(
                    f"adjustment layer {node.name!r} is visible; the stored composite "
                    "cannot represent it (see AdjustmentNode's docstring)"
                )
        elif node.visible:
            out.append((node, inherited))
    return out


def _mask_plane(spec: "MaskSpec | None", width: int, height: int) -> np.ndarray | None:
    """A mask spec expanded to a whole-canvas 0..255 coverage plane, or None."""
    if spec is None or spec.disabled:
        return None
    plane = np.full((height, width), 255.0 if spec.default_color else 0.0, dtype=np.float32)
    m_top = max(0, spec.top)
    m_left = max(0, spec.left)
    m_bottom = min(height, spec.bottom)
    m_right = min(width, spec.right)
    if m_bottom > m_top and m_right > m_left:
        plane[m_top:m_bottom, m_left:m_right] = spec.data[
            m_top - spec.top : m_bottom - spec.top, m_left - spec.left : m_right - spec.left
        ]
    if spec.inverted:
        plane = 255.0 - plane
    return plane


def _effective_alpha(
    node: ImageNode, width: int, height: int, inherited: np.ndarray | None = None
) -> np.ndarray:
    """Layer alpha over the whole canvas, with the layer mask and opacity folded in.

    `inherited` is the coverage contributed by the enclosing groups - their
    masks, opacity and fill opacity - already expanded to the canvas. All of it
    attenuates everything inside the group, so it multiplies in exactly like the
    layer's own mask.
    """
    alpha = np.zeros((height, width), dtype=np.float32)
    top = max(0, node.top)
    left = max(0, node.left)
    bottom = min(height, node.bottom)
    right = min(width, node.right)
    if bottom <= top or right <= left:
        return alpha
    alpha[top:bottom, left:right] = (
        node.pixels[top - node.top : bottom - node.top, left - node.left : right - node.left, 3]
        / 255.0
    )

    plane = _mask_plane(node.mask, width, height)
    if plane is not None:
        alpha *= plane / 255.0
    if inherited is not None:
        alpha *= inherited / 255.0

    return alpha * (node.opacity / 255.0) * (node.fill_opacity / 255.0)


def _composite(
    nodes: Sequence[ImageNode | AdjustmentNode | GroupNode], width: int, height: int
) -> np.ndarray:
    """A plain source-over flatten of the visible layers, as the stored preview.

    Blend modes are deliberately NOT applied here. The stored composite is a
    preview; every pixel the corpus actually asserts on is re-derived from the layer
    stack by psd-tools, so a writer that stored a blend-mode-aware composite would
    only be duplicating the reader's job — badly, and with this script's own
    arithmetic, which is exactly the coupling the corpus forbids.
    """
    canvas = np.zeros((height, width, 3), dtype=np.float32)
    for node, inherited in _visible_images_bottom_to_top(nodes, width, height):
        alpha = _effective_alpha(node, width, height, inherited)[:, :, None]
        source = np.zeros((height, width, 3), dtype=np.float32)
        top = max(0, node.top)
        left = max(0, node.left)
        bottom = min(height, node.bottom)
        right = min(width, node.right)
        if bottom <= top or right <= left:
            continue
        source[top:bottom, left:right] = node.pixels[
            top - node.top : bottom - node.top, left - node.left : right - node.left, :3
        ]
        canvas = source * alpha + canvas * (1.0 - alpha)
    return np.clip(np.round(canvas), 0, 255).astype(np.uint8).transpose(2, 0, 1)


# ══ assembly ══════════════════════════════════════════════════════════════════


def build_psd(
    nodes: Sequence[ImageNode | AdjustmentNode | GroupNode],
    *,
    width: int = CANVAS_W,
    height: int = CANVAS_H,
    compression: int = enums.Compression.raw,
    composite_compression: int | None = None,
) -> core.PsdFile:
    """Assemble the file. `compression` is the LAYER channels; the composite is separate.

    Photoshop compresses the merged composite with raw or RLE and never with ZIP, so
    the two are not one knob. `composite_compression` defaults to `compression`,
    which keeps every pre-existing fixture byte-identical; a ZIP fixture passes RLE
    here so the file it produces is shaped the way a real document is.
    """
    if composite_compression is None:
        composite_compression = compression

    counter = {"next": 1}

    def next_id() -> int:
        value = counter["next"]
        counter["next"] += 1
        return value

    records = _flatten(nodes, compression, next_id)
    records.reverse()  # top-to-bottom authoring order -> bottom-to-top file order

    composite = _composite(nodes, width, height)

    return core.PsdFile(
        version=enums.Version.version_1,
        num_channels=3,
        width=width,
        height=height,
        depth=8,
        color_mode=enums.ColorMode.rgb,
        layer_and_mask_info=layers.LayerAndMaskInfo(
            layer_info=layers.LayerInfo(layer_records=records)
        ),
        image_resources=image_resources.ImageResources(
            blocks=[image_resources.LayersGroupInfo(group_ids=[0] * len(records))]
        ),
        image_data=core.ImageData(channels=composite, compression=composite_compression),
        compression=composite_compression,
    )


def write_psd(psd: core.PsdFile, path: Path) -> int:
    """Write atomically: a partial fixture must never be left behind."""
    temp = path.with_suffix(path.suffix + ".partial")
    try:
        with temp.open("wb") as handle:
            psd.write(handle)
        size = temp.stat().st_size
        if size > HARD_CAP_BYTES:
            die(f"{path.name} is {size} bytes, above the hard cap of {HARD_CAP_BYTES}")
        if size > SOFT_CAP_BYTES:
            log(f"warning: {path.name} is {size} bytes, above the soft cap of {SOFT_CAP_BYTES}")
        temp.replace(path)
        return size
    except BaseException:
        temp.unlink(missing_ok=True)
        raise


def background(name: str = "Background") -> ImageNode:
    return ImageNode(
        name=name,
        top=0,
        left=0,
        pixels=quadrants(CANVAS_W, CANVAS_H, BLUE, CRIMSON, LIME, AMBER),
    )


# ══ fixtures ══════════════════════════════════════════════════════════════════


def fixture_rgb8_nested_groups(out: Path) -> Path:
    """A background plus a group whose two children are a sub-group and a layer.

    Exercises the lsct bounding(3) / open(1) pairing and child order through two
    levels of nesting.
    """
    nodes = [
        GroupNode(
            name="Content",
            children=[
                GroupNode(
                    name="Nested",
                    children=[
                        ImageNode(
                            name="Nested Leaf",
                            top=3,
                            left=6,
                            pixels=quadrants(14, 10, TEAL, VIOLET, ORANGE, CHALK),
                        )
                    ],
                ),
                ImageNode(
                    name="Sibling Leaf",
                    top=11,
                    left=14,
                    pixels=quadrants(16, 12, VIOLET, SLATE, CHALK, TEAL),
                ),
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-nested-groups", nodes)


def fixture_rgb8_group_passthrough(out: Path) -> Path:
    """Two sibling groups, one pass-through and one isolated, over a background.

    The `pass` blend key is the only thing that distinguishes them, so an importer
    that normalises unknown group keys to `norm` fails here and nowhere else.
    """
    nodes = [
        GroupNode(
            name="Pass Group",
            blend_mode=enums.BlendMode.pass_through,
            children=[
                ImageNode(
                    name="Pass Child",
                    top=2,
                    left=2,
                    pixels=quadrants(12, 9, TEAL, ORANGE, VIOLET, CHALK),
                )
            ],
        ),
        GroupNode(
            name="Isolated Group",
            blend_mode=enums.BlendMode.normal,
            children=[
                ImageNode(
                    name="Isolated Child",
                    top=12,
                    left=17,
                    pixels=quadrants(13, 10, VIOLET, CHALK, TEAL, SLATE),
                )
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-group-passthrough", nodes)


def fixture_rgb8_group_closed_folder(out: Path) -> Path:
    """One closed folder (lsct type 2) and one open folder (type 1) side by side.

    Both are present on purpose: a file with only a closed folder cannot tell a
    reader that discards the flag apart from one that preserves it.
    """
    nodes = [
        GroupNode(
            name="Closed Folder",
            closed=True,
            children=[
                ImageNode(
                    name="Closed Child",
                    top=2,
                    left=3,
                    pixels=quadrants(12, 9, ORANGE, TEAL, CHALK, VIOLET),
                )
            ],
        ),
        GroupNode(
            name="Open Folder",
            closed=False,
            children=[
                ImageNode(
                    name="Open Child",
                    top=13,
                    left=16,
                    pixels=quadrants(13, 9, CHALK, VIOLET, TEAL, SLATE),
                )
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-group-closed-folder", nodes)


# Every blend mode pytoshop can write, except pass_through — which is a group-only
# key and gets its own fixture. Order is pytoshop's declaration order so that the
# file reads like the enum.
_BLEND_MODES: list[tuple[str, bytes]] = [
    (name, value)
    for name, value in enums.BlendMode.__members__.items()
    if value != enums.BlendMode.pass_through
]


def fixture_rgb8_blend_modes(out: Path) -> Path:
    """One tiny layer per blend mode, tiled across the canvas over a background."""
    tile_w, tile_h = 6, 4
    per_row = CANVAS_W // tile_w  # 5
    swatches = [TEAL, VIOLET, ORANGE, CHALK, SLATE]

    nodes: list[ImageNode | AdjustmentNode | GroupNode] = []
    for index, (name, key) in enumerate(_BLEND_MODES):
        column = index % per_row
        row = index // per_row
        top = (row * tile_h) % (CANVAS_H - tile_h + 1)
        left = column * tile_w
        nodes.append(
            ImageNode(
                name=name,
                top=top,
                left=left,
                pixels=bands(
                    tile_w,
                    tile_h,
                    [swatches[index % len(swatches)], swatches[(index + 2) % len(swatches)]],
                ),
                blend_mode=key,
            )
        )
    nodes.append(background())
    return _emit(out, "rgb8-blend-modes", nodes)


def fixture_rgb8_opacity_hidden(out: Path) -> Path:
    """Partial layer opacity (128, 64) and one layer with the visible flag clear."""
    nodes = [
        ImageNode(
            name="Hidden",
            top=1,
            left=1,
            pixels=quadrants(12, 8, CHALK, ORANGE, VIOLET, TEAL),
            visible=False,
        ),
        ImageNode(
            name="Quarter Opacity",
            top=4,
            left=16,
            pixels=quadrants(14, 9, ORANGE, TEAL, CHALK, VIOLET),
            opacity=64,
        ),
        ImageNode(
            name="Half Opacity",
            top=12,
            left=5,
            pixels=quadrants(15, 10, VIOLET, CHALK, TEAL, ORANGE),
            opacity=128,
        ),
        background(),
    ]
    return _emit(out, "rgb8-opacity-hidden", nodes)


def fixture_rgb8_fill_opacity(out: Path) -> Path:
    """Fill opacity (iOpa) at 128 and at 0, beside a layer carrying no block.

    Layer opacity stays 255 on the partially filled layer so the two opacities
    are distinguishable: a reader that maps iOpa onto plain opacity, or plain
    opacity onto iOpa, fails here rather than passing by coincidence. The
    unblocked layer proves that an absent block reads as 255 and not as 0.

    No group, mask or clip: psd-tools applies a group's fill opacity when
    compositing and Agogo does not, so a group here would fail compositePixels
    on a renderer difference that has nothing to do with the block being read.
    """
    nodes = [
        ImageNode(
            name="No Fill Block",
            top=2,
            left=17,
            pixels=quadrants(13, 9, CHALK, VIOLET, TEAL, ORANGE),
        ),
        ImageNode(
            name="Empty Fill",
            top=13,
            left=4,
            pixels=quadrants(15, 9, ORANGE, TEAL, VIOLET, CHALK),
            fill_opacity=0,
        ),
        ImageNode(
            name="Half Fill",
            top=3,
            left=2,
            pixels=quadrants(12, 10, VIOLET, ORANGE, CHALK, TEAL),
            fill_opacity=128,
        ),
        background(),
    ]
    return _emit(out, "rgb8-fill-opacity", nodes)


def _masked_stack(
    *,
    disabled: bool = False,
    inverted: bool = False,
    layer_name: str = "Masked",
) -> list[ImageNode | AdjustmentNode | GroupNode]:
    """A masked layer whose mask rect is offset from, and smaller than, the layer.

    Layer rect  (3,4)-(21,28)  = 24x18 at (4,3)
    Mask  rect  (7,10)-(15,22) = 12x8  at (10,7)

    The -2 channel is sized to the MASK rect. A reader that sizes it from the layer
    rect reads 24x18 bytes out of a 96-byte channel and either throws or silently
    garbles — which is precisely the failure this fixture exists to catch.
    """
    mask = MaskSpec(
        top=7,
        left=10,
        bottom=15,
        right=22,
        data=mask_ramp(12, 8),
        default_color=True,  # outside the mask rect the layer is fully revealed
        disabled=disabled,
        inverted=inverted,
    )
    return [
        ImageNode(
            name=layer_name,
            top=3,
            left=4,
            pixels=quadrants(24, 18, VIOLET, TEAL, ORANGE, CHALK),
            mask=mask,
        ),
        background(),
    ]


def fixture_rgb8_layer_mask_offset(out: Path) -> Path:
    """A layer mask whose rect is offset from and smaller than the layer rect."""
    return _emit(out, "rgb8-layer-mask-offset", _masked_stack())


def fixture_rgb8_mask_disabled(out: Path) -> Path:
    """The same offset mask, with the mask-disabled flag set."""
    return _emit(
        out, "rgb8-mask-disabled", _masked_stack(disabled=True, layer_name="Masked Disabled")
    )


def fixture_rgb8_mask_inverted(out: Path) -> Path:
    """The same offset mask, with invert-when-blending set."""
    return _emit(
        out, "rgb8-mask-inverted", _masked_stack(inverted=True, layer_name="Masked Inverted")
    )


def fixture_rgb8_clipping(out: Path) -> Path:
    """A clipping layer clipped to the base layer directly below it."""
    nodes = [
        ImageNode(
            name="Clipped",
            top=6,
            left=8,
            pixels=quadrants(18, 14, ORANGE, CHALK, VIOLET, TEAL),
            clipping=True,
        ),
        ImageNode(
            name="Clip Base",
            top=9,
            left=3,
            pixels=quadrants(16, 11, TEAL, VIOLET, CHALK, SLATE),
        ),
        background(),
    ]
    return _emit(out, "rgb8-clipping", nodes)


def fixture_rgb8_group_empty(out: Path) -> Path:
    """A group with no children at all, beside a populated one.

    An empty group is an lsct bounding divider (3) immediately followed by its
    folder record (1), with nothing between them. A stack machine that assumes a
    divider is always followed by at least one child either drops the group or
    swallows the next sibling; the populated group beside it is what makes those
    two failures distinguishable from a correct read.
    """
    nodes = [
        GroupNode(name="Empty Group", children=[]),
        GroupNode(
            name="Populated Group",
            children=[
                ImageNode(
                    name="Populated Child",
                    top=4,
                    left=5,
                    pixels=quadrants(14, 11, TEAL, VIOLET, ORANGE, CHALK),
                )
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-group-empty", nodes)


def fixture_rgb8_clipping_across_group(out: Path) -> Path:
    """A clipped layer at the bottom of a group, whose only candidate base is outside it.

    Clipping does not cross a group boundary in Photoshop: "Boundary Clipped" is the
    bottom-most layer inside its group, so "Outside Base" below the group is NOT its
    clip base. "In-Group Clipped" is the control - its base sits directly beneath it
    inside the same group, so it must clip normally. A reader that resolves the clip
    base by walking the flat record list instead of the tree gets the first one wrong
    and the second one right.
    """
    nodes = [
        GroupNode(
            name="Boundary Group",
            children=[
                ImageNode(
                    name="Boundary Clipped",
                    top=2,
                    left=3,
                    pixels=quadrants(13, 9, ORANGE, CHALK, VIOLET, TEAL),
                    clipping=True,
                )
            ],
        ),
        GroupNode(
            name="Inner Group",
            children=[
                ImageNode(
                    name="In-Group Clipped",
                    top=12,
                    left=16,
                    pixels=quadrants(13, 9, VIOLET, TEAL, CHALK, ORANGE),
                    clipping=True,
                ),
                ImageNode(
                    name="In-Group Base",
                    top=14,
                    left=13,
                    pixels=quadrants(12, 8, TEAL, SLATE, ORANGE, VIOLET),
                ),
            ],
        ),
        ImageNode(
            name="Outside Base",
            top=4,
            left=1,
            pixels=quadrants(11, 8, SLATE, CHALK, TEAL, VIOLET),
        ),
        background(),
    ]
    return _emit(out, "rgb8-clipping-across-group", nodes)


def fixture_rgb8_mask_larger_than_layer(out: Path) -> Path:
    """A layer mask whose rect strictly CONTAINS the layer rect.

    The offset-mask fixture covers a mask smaller than its layer; this is the other
    direction. The -2 channel here is bigger than the layer's colour channels, so a
    reader that sizes the mask from the layer rect under-reads it, and one that blits
    without clipping to the layer writes out of bounds.

    Layer rect  (6,8)-(18,22) = 14x12 at (8,6)
    Mask  rect  (2,4)-(22,28) = 24x20 at (4,2)
    """
    mask = MaskSpec(
        top=2,
        left=4,
        bottom=22,
        right=28,
        data=mask_ramp(24, 20),
        default_color=False,  # outside the mask rect the layer is hidden
    )
    nodes = [
        ImageNode(
            name="Masked Larger",
            top=6,
            left=8,
            pixels=quadrants(14, 12, VIOLET, TEAL, ORANGE, CHALK),
            mask=mask,
        ),
        background(),
    ]
    return _emit(out, "rgb8-mask-larger-than-layer", nodes)


def fixture_rgb8_group_mask(out: Path) -> Path:
    """A raster mask on a GROUP, attenuating both of its children.

    The mask lives on the group's opening (lsct 1) record, never on the bounding
    divider. A reader that only looks for masks on pixel layers loses it silently,
    and one that attaches it to the divider attaches it to a record that is not a
    layer at all. The unmasked sibling layer outside the group is the control.
    """
    mask = MaskSpec(
        top=4,
        left=6,
        bottom=18,
        right=24,
        data=mask_ramp(18, 14),
        default_color=False,
    )
    nodes = [
        GroupNode(
            name="Masked Group",
            mask=mask,
            children=[
                ImageNode(
                    name="Group Child Top",
                    top=3,
                    left=5,
                    pixels=quadrants(12, 9, ORANGE, CHALK, VIOLET, TEAL),
                ),
                ImageNode(
                    name="Group Child Bottom",
                    top=11,
                    left=14,
                    pixels=quadrants(13, 10, VIOLET, SLATE, TEAL, CHALK),
                ),
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-group-mask", nodes)


def fixture_rgb8_rle_layers(out: Path) -> Path:
    """A multi-layer file whose LAYER channels are RLE-compressed, not just the composite.

    The flat RLE fixture only exercises the composite's RLE stream; this one puts
    every layer channel — colour and alpha alike — through PackBits.
    """
    nodes = [
        ImageNode(
            name="RLE Top",
            top=2,
            left=13,
            pixels=bands(17, 11, [ORANGE, CHALK, VIOLET]),
        ),
        ImageNode(
            name="RLE Middle",
            top=10,
            left=2,
            pixels=bands(18, 12, [TEAL, SLATE, ORANGE]),
        ),
        background("RLE Background"),
    ]
    return _emit(out, "rgb8-rle-layers", nodes, compression=enums.Compression.rle)


def _zip_stack(prefix: str) -> list[ImageNode | AdjustmentNode | GroupNode]:
    """The layer stack both ZIP fixtures share, so the two differ ONLY in the codec.

    Same geometry, same pixels: whatever the two sidecars disagree about is the
    prediction step and nothing else.
    """
    return [
        ImageNode(
            name=f"{prefix} Top",
            top=2,
            left=13,
            pixels=quadrants(17, 11, ORANGE, CHALK, VIOLET, TEAL),
        ),
        ImageNode(
            name=f"{prefix} Middle",
            top=10,
            left=2,
            pixels=bands(18, 12, [TEAL, SLATE, ORANGE]),
        ),
        background(f"{prefix} Background"),
    ]


def fixture_rgb8_zip_layers(out: Path) -> Path:
    """Layer channels compressed with ZIP (zlib, no prediction); RLE composite.

    pytoshop's `compress_zip` is plain zlib and needs no workaround, so this is
    reachable without a Photoshop-authored file — which is what the manifest used to
    claim was impossible. The composite deliberately stays RLE: Photoshop writes the
    merged image raw or RLE and never ZIP, so a ZIP composite would be a shape no
    real document has.
    """
    return _emit(
        out,
        "rgb8-zip-layers",
        _zip_stack("Zip"),
        compression=enums.Compression.zip,
        composite_compression=enums.Compression.rle,
    )


def fixture_rgb8_zip_prediction_layers(out: Path) -> Path:
    """Layer channels compressed with ZIP + the 8-bit horizontal predictor.

    Identical pixels to `rgb8-zip-layers`, so the pair isolates the predictor. The
    predictor RESETS at every row boundary (see defect 5), which is the property a
    decoder most easily gets wrong: carry the running sum across the row break and
    row 0 still decodes correctly while every row after it is wrong. The painted
    content is non-uniform in both axes precisely so that such a decoder cannot
    accidentally agree.
    """
    return _emit(
        out,
        "rgb8-zip-prediction-layers",
        _zip_stack("Zip Pred"),
        compression=enums.Compression.zip_prediction,
        composite_compression=enums.Compression.rle,
    )


def _emit(
    out: Path,
    fixture_id: str,
    nodes: Sequence[ImageNode | AdjustmentNode | GroupNode],
    *,
    compression: int = enums.Compression.raw,
    composite_compression: int | None = None,
) -> Path:
    path = out / f"{fixture_id}.psd"
    psd = build_psd(nodes, compression=compression, composite_compression=composite_compression)
    size = write_psd(psd, path)
    print(f"  {path.name:<28} {size:>8} bytes")
    return path


def fixture_rgb8_adjustment_core(out: Path) -> Path:
    """Levels, Curves and Hue/Saturation — the three richest binary blocks.

    Every value is deliberately off its default, so a reader that recognises the
    block but ignores its payload produces identity parameters and fails. The
    Levels layer additionally carries an offset mask, a clipping flag, a non-255
    opacity and a non-normal blend mode: an adjustment layer is a layer record
    like any other, and an importer that special-cases it must not drop the
    fields every other record gets.

    Curves sets a per-channel curve as well as the composite one, because the
    composite-only case cannot distinguish a reader that keeps channel identity
    from one that discards it.
    """
    nodes = [
        AdjustmentNode(
            name="Levels",
            blocks=[
                (
                    "levl",
                    _levl(
                        [
                            (10, 245, 5, 250, 120),  # composite: gamma 1.20
                            (20, 235, 0, 255, 90),  # red:       gamma 0.90
                            (0, 255, 10, 240, 100),  # green
                            (35, 220, 0, 255, 145),  # blue:      gamma 1.45
                        ]
                    ),
                )
            ],
            opacity=160,
            blend_mode=enums.BlendMode.multiply,
            clipping=True,
            mask=MaskSpec(
                top=5, left=6, bottom=17, right=26, data=mask_ramp(20, 12),
                default_color=True,
            ),
        ),
        AdjustmentNode(
            name="Curves",
            blocks=[
                (
                    "curv",
                    _curv(
                        {
                            0: [(0, 0), (90, 128), (255, 255)],  # composite
                            1: [(12, 0), (200, 128), (255, 255)],  # red
                        }
                    ),
                )
            ],
        ),
        AdjustmentNode(
            name="Hue Saturation",
            blocks=[
                (
                    "hue2",
                    _hue2(
                        master=(25, -30, 10),
                        items=[
                            ((315, 345, 15, 45), (12, -5, 3)),  # reds
                            ((15, 45, 75, 105), (-8, 20, 0)),  # yellows
                            ((75, 105, 135, 165), (0, 0, -12)),  # greens
                            ((135, 165, 195, 225), (5, 5, 5)),  # cyans
                            ((195, 225, 255, 285), (-20, 0, 0)),  # blues
                            ((255, 285, 315, 345), (0, -30, 15)),  # magentas
                        ],
                    ),
                )
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-adjustment-core", nodes)


def fixture_rgb8_adjustment_binary(out: Path) -> Path:
    """The remaining binary adjustment blocks, one layer each.

    Threshold, Posterize and Invert are the degenerate shapes worth pinning
    beside the others: two of them are a single padded uint16 and the third has
    no payload at all, so a parser that requires a non-empty payload, or that
    reads a length before a value, breaks here and nowhere else.
    """
    nodes = [
        AdjustmentNode(
            name="Color Balance",
            blocks=[("blnc", _blnc((20, -10, 5), (-15, 25, 0), (0, 5, -30), luminosity=1))],
        ),
        AdjustmentNode(
            name="Channel Mixer",
            blocks=[("mixr", _mixr(0, (80, 10, 10, 0, 5)))],
        ),
        AdjustmentNode(
            name="Selective Color",
            blocks=[
                (
                    "selc",
                    _selc(
                        1,  # absolute
                        [
                            (0, 0, 0, 0),  # unused leading plate
                            (10, -20, 30, -5),  # reds
                            (-15, 25, 0, 10),  # yellows
                            (5, 5, -40, 0),  # greens
                            (0, -10, 20, 15),  # cyans
                            (30, 0, -25, -10),  # blues
                            (-5, 40, 0, 5),  # magentas
                            (0, 0, 0, 20),  # whites
                            (10, 10, 10, -10),  # neutrals
                            (0, 0, 0, 35),  # blacks
                        ],
                    ),
                )
            ],
        ),
        AdjustmentNode(name="Threshold", blocks=[("thrs", _thrs(96))]),
        AdjustmentNode(name="Posterize", blocks=[("post", _post(6))]),
        AdjustmentNode(name="Invert", blocks=[("nvrt", _nvrt())]),
        AdjustmentNode(
            name="Photo Filter",
            # Colour space 0 is RGB; the four components are R, G, B and an
            # unused slot, each 0..65535. This is a warming filter.
            blocks=[("phfl", _phfl(0, (60000, 30000, 10000, 0), 35, 1))],
        ),
        background(),
    ]
    return _emit(out, "rgb8-adjustment-binary", nodes)


def fixture_rgb8_adjustment_descriptor(out: Path) -> Path:
    """The two adjustments Photoshop stores as Action Descriptors.

    Black & White is descriptor-only. Brightness/Contrast is the interesting one:
    psd-tools marks the `brit` tag obsolete and maps its BrightnessContrast layer
    to `CgEd`, so Photoshop writes BOTH — the legacy block for old readers and
    the descriptor with the live values. This layer carries both, with
    DIFFERENT numbers in each, so a reader that takes the legacy block when the
    descriptor is present is distinguishable from one that prefers the
    descriptor. `brit` says +11/-7; `CgEd`, which wins, says +30/-20.
    """
    nodes = [
        AdjustmentNode(
            name="Black and White",
            blocks=[("blwh", _blwh(40, 60, 40, 60, 20, 80))],
        ),
        AdjustmentNode(
            name="Brightness Contrast",
            blocks=[
                ("brit", _brit(11, -7)),
                ("CgEd", _cged(30, -20)),
            ],
        ),
        background(),
    ]
    return _emit(out, "rgb8-adjustment-descriptor", nodes)


FIXTURES: "OrderedDict[str, Callable[[Path], Path]]" = OrderedDict(
    [
        ("rgb8-nested-groups", fixture_rgb8_nested_groups),
        ("rgb8-group-passthrough", fixture_rgb8_group_passthrough),
        ("rgb8-group-closed-folder", fixture_rgb8_group_closed_folder),
        ("rgb8-blend-modes", fixture_rgb8_blend_modes),
        ("rgb8-opacity-hidden", fixture_rgb8_opacity_hidden),
        ("rgb8-fill-opacity", fixture_rgb8_fill_opacity),
        ("rgb8-layer-mask-offset", fixture_rgb8_layer_mask_offset),
        ("rgb8-mask-disabled", fixture_rgb8_mask_disabled),
        ("rgb8-mask-inverted", fixture_rgb8_mask_inverted),
        ("rgb8-clipping", fixture_rgb8_clipping),
        ("rgb8-clipping-across-group", fixture_rgb8_clipping_across_group),
        ("rgb8-group-empty", fixture_rgb8_group_empty),
        ("rgb8-group-mask", fixture_rgb8_group_mask),
        ("rgb8-mask-larger-than-layer", fixture_rgb8_mask_larger_than_layer),
        ("rgb8-rle-layers", fixture_rgb8_rle_layers),
        ("rgb8-zip-layers", fixture_rgb8_zip_layers),
        ("rgb8-zip-prediction-layers", fixture_rgb8_zip_prediction_layers),
        ("rgb8-adjustment-core", fixture_rgb8_adjustment_core),
        ("rgb8-adjustment-binary", fixture_rgb8_adjustment_binary),
        ("rgb8-adjustment-descriptor", fixture_rgb8_adjustment_descriptor),
    ]
)


# ══ CLI ═══════════════════════════════════════════════════════════════════════


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Generate the layered PSD fixtures with pytoshop (writer only).",
    )
    parser.add_argument("--out", type=Path, help="output directory (required unless --list)")
    parser.add_argument(
        "--only",
        action="append",
        default=[],
        metavar="ID",
        help="regenerate just this fixture id, repeatable",
    )
    parser.add_argument(
        "--list", action="store_true", help="print the fixture ids and exit"
    )
    args = parser.parse_args(argv)

    if args.list:
        for fixture_id in FIXTURES:
            print(fixture_id)
        return 0

    if args.out is None:
        parser.error("--out is required unless --list is given")

    _assert_luni_patch_applied()

    selected = list(dict.fromkeys(args.only)) or list(FIXTURES)
    unknown = [fixture_id for fixture_id in selected if fixture_id not in FIXTURES]
    if unknown:
        die(f"unknown fixture id(s): {', '.join(unknown)} (try --list)")

    args.out.mkdir(parents=True, exist_ok=True)
    log(f"pytoshop {pytoshop.__version__} -> {args.out}")
    for fixture_id in selected:
        FIXTURES[fixture_id](args.out)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
