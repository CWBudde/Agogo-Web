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

Four pytoshop 1.2.1 defects are worked around at the top of this file. They are
defects in the writer, not in any reader: see ``_encode_unicode_string_no_nul``,
``_PackBits``, ``_AbsentLayerMask`` and ``_LayerRecord``.
"""

from __future__ import annotations

import argparse
import struct
import sys
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
# All four are verified against pytoshop 1.2.1. Each one makes pytoshop emit what
# Photoshop emits; none of them is a concession to any particular reader.


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
class GroupNode:
    name: str
    children: list = field(default_factory=list)
    blend_mode: bytes = enums.BlendMode.normal
    opacity: int = 255
    visible: bool = True
    closed: bool = False
    clipping: bool = False


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


def _image_record(node: ImageNode, compression: int, layer_id: int) -> layers.LayerRecord:
    channels = _channels(node.pixels, compression)
    mask_block: layers.LayerMask = _AbsentLayerMask()

    spec = node.mask
    if spec is not None:
        if spec.data.shape != (spec.height, spec.width):
            die(
                f"{node.name}: mask channel is {spec.data.shape} but the mask rect is "
                f"{(spec.height, spec.width)} — the -2 channel must match the MASK rect, "
                "not the layer rect"
            )
        mask_block = layers.LayerMask(
            top=spec.top,
            left=spec.left,
            bottom=spec.bottom,
            right=spec.right,
            default_color=spec.default_color,
            layer_mask_disabled=spec.disabled,
            invert_layer_mask_when_blending=spec.inverted,
        )
        # The mask channel goes last, after the colour channels, and must be added
        # BEFORE the record is constructed — see defect 4.
        channels[enums.ChannelId.user_layer_mask] = layers.ChannelImageData(
            image=np.ascontiguousarray(spec.data), compression=compression
        )

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
    header = _LayerRecord(
        name=node.name,
        blend_mode=node.blend_mode,
        opacity=node.opacity,
        visible=node.visible,
        clipping=node.clipping,
        pixel_data_irrelevant=True,
        blocks=[
            tagged_block.UnicodeLayerName(name=node.name),
            tagged_block.SectionDividerSetting(type=divider),
            tagged_block.LayerId(id=header_id),
        ],
    )
    header.mask = _AbsentLayerMask()

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
    nodes: Sequence[ImageNode | GroupNode], compression: int, next_id: Callable[[], int]
) -> list[layers.LayerRecord]:
    records: list[layers.LayerRecord] = []
    for node in nodes:
        if isinstance(node, GroupNode):
            records.extend(_group_records(node, compression, next_id))
        else:
            records.append(_image_record(node, compression, next_id()))
    return records


# ══ composite ═════════════════════════════════════════════════════════════════


def _visible_images_bottom_to_top(nodes: Sequence[ImageNode | GroupNode]) -> list[ImageNode]:
    out: list[ImageNode] = []
    for node in reversed(list(nodes)):
        if isinstance(node, GroupNode):
            if node.visible:
                out.extend(_visible_images_bottom_to_top(node.children))
        elif node.visible:
            out.append(node)
    return out


def _effective_alpha(node: ImageNode, width: int, height: int) -> np.ndarray:
    """Layer alpha over the whole canvas, with the layer mask and opacity folded in."""
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

    spec = node.mask
    if spec is not None and not spec.disabled:
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
        alpha *= plane / 255.0

    return alpha * (node.opacity / 255.0)


def _composite(
    nodes: Sequence[ImageNode | GroupNode], width: int, height: int
) -> np.ndarray:
    """A plain source-over flatten of the visible layers, as the stored preview.

    Blend modes are deliberately NOT applied here. The stored composite is a
    preview; every pixel the corpus actually asserts on is re-derived from the layer
    stack by psd-tools, so a writer that stored a blend-mode-aware composite would
    only be duplicating the reader's job — badly, and with this script's own
    arithmetic, which is exactly the coupling the corpus forbids.
    """
    canvas = np.zeros((height, width, 3), dtype=np.float32)
    for node in _visible_images_bottom_to_top(nodes):
        alpha = _effective_alpha(node, width, height)[:, :, None]
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
    nodes: Sequence[ImageNode | GroupNode],
    *,
    width: int = CANVAS_W,
    height: int = CANVAS_H,
    compression: int = enums.Compression.raw,
) -> core.PsdFile:
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
        image_data=core.ImageData(channels=composite, compression=compression),
        compression=compression,
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

    nodes: list[ImageNode | GroupNode] = []
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


def _masked_stack(
    *,
    disabled: bool = False,
    inverted: bool = False,
    layer_name: str = "Masked",
) -> list[ImageNode | GroupNode]:
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


def _emit(
    out: Path,
    fixture_id: str,
    nodes: Sequence[ImageNode | GroupNode],
    *,
    compression: int = enums.Compression.raw,
) -> Path:
    path = out / f"{fixture_id}.psd"
    psd = build_psd(nodes, compression=compression)
    size = write_psd(psd, path)
    print(f"  {path.name:<28} {size:>8} bytes")
    return path


FIXTURES: "OrderedDict[str, Callable[[Path], Path]]" = OrderedDict(
    [
        ("rgb8-nested-groups", fixture_rgb8_nested_groups),
        ("rgb8-group-passthrough", fixture_rgb8_group_passthrough),
        ("rgb8-group-closed-folder", fixture_rgb8_group_closed_folder),
        ("rgb8-blend-modes", fixture_rgb8_blend_modes),
        ("rgb8-opacity-hidden", fixture_rgb8_opacity_hidden),
        ("rgb8-layer-mask-offset", fixture_rgb8_layer_mask_offset),
        ("rgb8-mask-disabled", fixture_rgb8_mask_disabled),
        ("rgb8-mask-inverted", fixture_rgb8_mask_inverted),
        ("rgb8-clipping", fixture_rgb8_clipping),
        ("rgb8-rle-layers", fixture_rgb8_rle_layers),
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
