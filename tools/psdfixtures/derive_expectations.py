#!/usr/bin/env python3
"""Derive a PSD fixture expectation sidecar (``<id>.expected.json``) with psd-tools.

Phase S.10.1, step 2. See README.md and the sidecar contract for the schema.

The whole point of this script is INDEPENDENCE: the numbers a fixture is judged
against must come from an implementation that is not Agogo's. Therefore:

    HARD RULE — zero Agogo code in the loop.

This file must never import, exec, or shell out to anything under
``packages/engine-wasm``. The only PSD knowledge here comes from psd-tools. If you
ever find yourself wanting to "just check what the engine does" to make a value
agree, stop: a disagreement is the finding, not a bug in this script.

Usage
-----
    derive_expectations.py <fixture.psd> --id <id> --out <dir>

Writes ``<dir>/<id>.expected.json``. ``--out -`` prints to stdout instead.
"""

from __future__ import annotations

import argparse
import datetime as _dt
import json
import sys
from pathlib import Path
from typing import Any

try:
    import numpy as np
    import psd_tools
    from psd_tools import PSDImage
except ImportError as exc:  # pragma: no cover - environment problem, not logic
    sys.exit(f"error: {exc}. Install with: pip install psd-tools")

# Repo-relative path recorded in expectationSource.script.
SCRIPT_REPO_PATH = "tools/psdfixtures/derive_expectations.py"

SCHEMA_VERSION = 1

ALL_SCOPES = (
    "document",
    "tree",
    "warnings",
    "psdRecords",
    "layerPixels",
    "maskSamples",
    "compositePixels",
    "writer",
    "import",
)

# Scopes the contract requires every sidecar to assert.
REQUIRED_SCOPES = ("document", "tree", "warnings")

# PSD four-character blend keys -> engine model.BlendMode strings.
#
# Mirrors psdBlendModeMappings in internal/io/psd/helpers.go. We key off the raw
# four bytes rather than psd-tools' enum *names* so that the mapping is a
# statement about the FILE FORMAT, not about either library's vocabulary.
# Trailing spaces are significant.
BLEND_KEY_TO_MODE = {
    "norm": "normal",
    "diss": "dissolve",
    "mul ": "multiply",
    "idiv": "color-burn",
    "lbrn": "linear-burn",
    "dark": "darken",
    "dkCl": "darker-color",
    "scrn": "screen",
    "div ": "color-dodge",
    "lddg": "linear-dodge",
    "lite": "lighten",
    "lgCl": "lighter-color",
    "over": "overlay",
    "sLit": "soft-light",
    "hLit": "hard-light",
    "vLit": "vivid-light",
    "lLit": "linear-light",
    "pLit": "pin-light",
    "hMix": "hard-mix",
    "diff": "difference",
    "smud": "exclusion",
    "fsub": "subtract",
    "fdiv": "divide",
    "hue ": "hue",
    "sat ": "saturation",
    "colr": "color",
    "lum ": "luminosity",
    # Pass-through has no distinct model.BlendMode; the engine imports it as
    # Normal and records the pass-through-ness separately (group isolation).
    "pass": "normal",
}

# psd-tools ColorMode value -> engine Document.ColorMode string.
COLOR_MODE_TO_ENGINE = {1: "gray", 3: "rgb"}


# ── small helpers ─────────────────────────────────────────────────────────────


def _blend_key(obj: Any) -> str | None:
    """Return the raw four-character PSD blend key of a record or API layer."""
    mode = getattr(obj, "blend_mode", None)
    if mode is None:
        return None
    raw = getattr(mode, "value", mode)
    if isinstance(raw, bytes):
        return raw.decode("ascii", "replace")
    return str(raw)


def _blend_mode(obj: Any) -> str | None:
    key = _blend_key(obj)
    if key is None:
        return None
    # An unknown key is NOT silently normalised to "normal": that would hide the
    # very defect (ImageMagick's byte-swapped "mron") this corpus exists to catch.
    return BLEND_KEY_TO_MODE.get(key)


def _bounds(obj: Any) -> dict[str, int]:
    left = int(getattr(obj, "left", 0) or 0)
    top = int(getattr(obj, "top", 0) or 0)
    right = int(getattr(obj, "right", 0) or 0)
    bottom = int(getattr(obj, "bottom", 0) or 0)
    return {"x": left, "y": top, "w": right - left, "h": bottom - top}


def _flag(obj: Any, *names: str) -> bool | None:
    """First present boolean attribute out of ``names``, or None."""
    for name in names:
        value = getattr(obj, name, None)
        if value is not None:
            return bool(value)
    return None


def _to_u8(value: Any) -> int:
    """Clamp a psd-tools channel sample to a 0..255 int.

    The scale is decided by the sample's TYPE, never by its value. An integer
    sample is already a 0..255 code, so ``np.uint8(1)`` stays 1; a float sample
    from ``layer.numpy()`` is normalised, so ``1.0`` becomes 255. Deciding by
    value instead would silently promote the integer 1 to 255 and bake a false
    expectation into the sidecar.
    """
    if isinstance(value, (bool, int, np.integer)):
        return max(0, min(255, int(value)))
    number = float(value)
    if 0.0 <= number <= 1.0:
        number *= 255.0
    return max(0, min(255, int(round(number))))


def _rgba_from_pixel(pixel: Any, mode: str) -> list[int]:
    """Normalise a PIL pixel to a 4-element RGBA list of 0..255 ints."""
    if isinstance(pixel, (int, float)):
        pixel = (pixel,)
    values = [int(round(float(component))) for component in pixel]
    if mode in ("L", "I", "F"):
        gray = values[0]
        return [gray, gray, gray, 255]
    if mode == "LA":
        return [values[0], values[0], values[0], values[1]]
    if mode == "RGB":
        return values[:3] + [255]
    if mode == "RGBA":
        return values[:4]
    # Fall back: pad or truncate to RGBA.
    values = (values + [255, 255, 255, 255])[:4]
    return values


# ── informative sample-point selection ────────────────────────────────────────


def _candidate_points(width: int, height: int) -> list[tuple[int, int]]:
    """Corners first, then a coarse interior grid.

    Corners catch off-by-one and origin errors; interior points catch region
    mix-ups. Order matters: earlier candidates are preferred.
    """
    if width <= 0 or height <= 0:
        return []
    last_x, last_y = width - 1, height - 1
    points = [(0, 0), (last_x, 0), (0, last_y), (last_x, last_y)]
    for fy in (0.25, 0.5, 0.75):
        for fx in (0.25, 0.5, 0.75):
            points.append((min(last_x, int(width * fx)), min(last_y, int(height * fy))))
    # De-duplicate, preserving order.
    seen: set[tuple[int, int]] = set()
    unique = []
    for point in points:
        if point not in seen:
            seen.add(point)
            unique.append(point)
    return unique


def _pick_distinct(
    sampler: Any, width: int, height: int, budget: int
) -> list[tuple[int, int, list[int]]]:
    """Pick up to ``budget`` sample points with pairwise-distinct colours.

    A fixture painted in one flat colour would let a sample taken from entirely
    the wrong coordinate pass, so distinct colours are the whole value of a pixel
    assertion. Distinct points are taken first; if the image genuinely has fewer
    colours than the budget, we stop rather than pad with duplicates.
    """
    if budget <= 0:
        return []
    chosen: list[tuple[int, int, list[int]]] = []
    seen_colours: set[tuple[int, ...]] = set()
    for x, y in _candidate_points(width, height):
        rgba = sampler(x, y)
        if rgba is None:
            continue
        key = tuple(rgba)
        if key in seen_colours:
            continue
        seen_colours.add(key)
        chosen.append((x, y, rgba))
        if len(chosen) >= budget:
            break
    return chosen


# ── document / tree / records ─────────────────────────────────────────────────


def build_document(psd: PSDImage) -> dict[str, Any]:
    header = psd._record.header
    mode_value = int(getattr(psd.color_mode, "value", psd.color_mode))
    color_mode = COLOR_MODE_TO_ENGINE.get(mode_value)
    if color_mode is None:
        sys.exit(
            f"error: colour mode {mode_value} is not one the engine models "
            f"(expected 1=grayscale or 3=RGB)"
        )
    return {
        "width": int(psd.width),
        "height": int(psd.height),
        "colorMode": color_mode,
        "bitDepth": int(header.depth),
        "resolution": _resolution(psd),
        "isPSB": int(header.version) == 2,
    }


def _resolution(psd: PSDImage) -> float:
    """Horizontal resolution in DPI, or 72.0 when the file carries no 1005 resource.

    72 is what every PSD reader falls back to, including the engine. ImageMagick
    writes no image resources at all unless -density is passed, so most flat
    fixtures legitimately land on the default.
    """
    try:
        from psd_tools.constants import Resource

        info = psd._record.image_resources.get_data(Resource.RESOLUTION_INFO)
    except Exception:
        return 72.0
    if info is None:
        return 72.0
    horizontal = getattr(info, "horizontal", None)
    if horizontal is None:
        return 72.0
    value = float(horizontal)
    # psd-tools may hand back the raw 16.16 fixed-point integer.
    if value >= 65536.0:
        value /= 65536.0
    # Deliberately NOT rounded. PSD stores resolution as a 16.16 fixed-point
    # value, and a generator that is one LSB off writes a DPI that is genuinely
    # not the round number it meant. ImageMagick does exactly this: asked for
    # 144 dpi it writes 0x00900001, i.e. 144 + 1/65536. Rounding here would make
    # the sidecar assert a value the file does not contain, and would hide the
    # engine reading the fixed-point correctly.
    return value


def _layer_type(layer: Any) -> str:
    """Map a psd-tools layer to a model.LayerType string."""
    if layer.is_group():
        return "group"
    kind = str(getattr(layer, "kind", "") or "")
    if kind in ("type",):
        return "text"
    if kind in ("shape",):
        return "vector"
    # psd-tools names every adjustment/fill kind individually
    # (brightnesscontrast, curves, solidcolor, gradientfill, ...). Anything that
    # is not a plain pixel or smart-object layer is an adjustment to the engine.
    if kind in ("pixel", "smartobject", "psdimage", ""):
        return "pixel"
    return "adjustment"


# ── adjustment parameters ─────────────────────────────────────────────────────
#
# The mapping from a parsed PSD adjustment onto the engine's JSON parameter
# vocabulary, written here from the format spec and the engine's documented
# field names. It is deliberately a SECOND implementation of the same mapping
# the Go importer performs: the hard rule at the top of this file applies, so
# nothing below may consult Agogo. When the two disagree, that is the finding.
#
# Ranges follow the engine's own conventions (internal/engine/adjustments_core.go):
# levels in 0..255 with a real-valued gamma, hue shifts in degrees, saturation
# and lightness in -100..100, channel-mixer weights and selective-colour plates
# in percent, photo-filter density in percent.

_HUE_BUCKETS = ("reds", "yellows", "greens", "cyans", "blues", "magentas")

# selc stores ten plates; the first is unused and the rest are these, in order.
_SELECTIVE_BUCKETS = (
    "reds", "yellows", "greens", "cyans", "blues", "magentas",
    "whites", "neutrals", "blacks",
)

# The curv channel bitmap: bit 0 is the composite curve, 1..3 are R, G and B.
_CURVE_KEYS = {0: "points", 1: "redPoints", 2: "greenPoints", 3: "bluePoints"}


def _points(raw: Any) -> list[dict[str, int]]:
    """PSD stores a curve point as (output, input); the engine stores {x, y}."""
    return [{"x": int(x), "y": int(y)} for y, x in raw]


def _adjustment(layer: Any) -> tuple[str, dict[str, Any]] | None:
    """(engine adjustment kind, engine params) for an adjustment layer, or None."""
    kind = str(getattr(layer, "kind", "") or "")
    data = getattr(layer, "_data", None)

    if kind == "levels":
        # The engine carries ONE set of level values and a channel selector; PSD
        # carries 29 records, composite first. Only the composite record has an
        # engine home, which is why the importer warns about the rest.
        record = data[0]
        return "levels", {
            "channel": "rgb",
            "inputBlack": int(record.input_floor),
            "inputWhite": int(record.input_ceiling),
            "outputBlack": int(record.output_floor),
            "outputWhite": int(record.output_ceiling),
            # A short from 10..999 standing for 0.1..9.99.
            "gamma": round(int(record.gamma) / 100.0, 4),
        }

    if kind == "curves":
        params: dict[str, Any] = {}
        channels = [index for index in range(4) if data.count_map & (1 << index)]
        for slot, points in zip(channels, data.data):
            params[_CURVE_KEYS[slot]] = _points(points)
        return "curves", params

    if kind == "huesaturation":
        hue, saturation, lightness = data.master
        params = {
            "hueShift": int(hue),
            "saturation": int(saturation),
            "lightness": int(lightness),
            "colorize": bool(data.enable == 0),
        }
        for name, item in zip(_HUE_BUCKETS, data.items):
            bucket_hue, bucket_sat, bucket_light = item[1]
            if bucket_hue or bucket_sat or bucket_light:
                params[name] = {
                    "hueShift": int(bucket_hue),
                    "saturation": int(bucket_sat),
                    "lightness": int(bucket_light),
                }
        return "huesat", params

    if kind == "colorbalance":
        tone = lambda values: dict(  # noqa: E731 - a local, three-key shape
            zip(("cyanRed", "magentaGreen", "yellowBlue"), (int(v) for v in values))
        )
        return "color-balance", {
            "shadows": tone(data.shadows),
            "midtones": tone(data.midtones),
            "highlights": tone(data.highlights),
            "preserveLuminosity": bool(data.luminosity),
        }

    if kind == "channelmixer":
        # psd-tools surfaces five int16 for ONE output row - four source weights
        # in percent and a constant - and hands the rest back as `unknown`. The
        # engine has a row per output channel and no constant at all, so only
        # the red row maps and the constant is reported as lost.
        red = [int(v) for v in data.data[:3]]
        return "channel-mixer", {
            "monochrome": bool(data.monochrome),
            "red": red,
        }

    if kind == "selectivecolor":
        params = {"mode": "absolute" if int(data.method) else "relative"}
        for name, plate in zip(_SELECTIVE_BUCKETS, data.data[1:]):
            cyan, magenta, yellow, black = (int(v) for v in plate)
            if cyan or magenta or yellow or black:
                params[name] = {
                    "cyanRed": cyan,
                    "magentaGreen": magenta,
                    "yellowBlue": yellow,
                    "black": black,
                }
        return "selective-color", params

    if kind == "threshold":
        return "threshold", {"threshold": int(data)}

    if kind == "posterize":
        return "posterize", {"levels": int(data)}

    if kind == "invert":
        return "invert", {}

    if kind == "photofilter":
        # Colour space 0 is RGB and its components are 16-bit; the engine wants
        # straight 8-bit RGBA. Density is already a percentage.
        components = tuple(int(v) for v in (data.color_components or ()))
        colour = [component // 257 for component in components[:3]] + [255]
        return "photo-filter", {
            "color": colour,
            "density": int(data.density),
            "preserveLuminosity": bool(data.luminosity),
        }

    if kind == "blackandwhite":
        get = lambda key, default=0: int(data.get(key.encode("ascii"), default))  # noqa: E731
        return "black-white", {
            "reds": get("Rd  "),
            "yellows": get("Yllw"),
            "greens": get("Grn "),
            "cyans": get("Cyn "),
            "blues": get("Bl  "),
            "magentas": get("Mgnt"),
            "tint": bool(data.get(b"useTint", False)),
        }

    if kind == "brightnesscontrast":
        # psd-tools reads these off CgEd, the descriptor block; the legacy
        # `brit` block is obsolete and psd-tools ignores it. The engine agrees.
        return "brightness-contrast", {
            "brightness": int(layer.brightness),
            "contrast": int(layer.contrast),
            "legacy": bool(layer.use_legacy),
        }

    return None


def _mask_summary(layer_or_record: Any, *, record: bool) -> dict[str, Any]:
    """Mask description.

    The model tree gets only present/enabled, because psdimport.buildLayerMask
    rasterises the mask to a document-sized plane and discards the rect. The rect,
    default fill and inverted flag are asserted in the psdRecords scope instead.
    """
    mask = getattr(layer_or_record, "mask_data" if record else "mask", None)
    if mask is None:
        return {"present": False}
    # PSD stores "disabled"; the engine models "enabled".
    disabled = _flag(getattr(mask, "flags", None), "mask_disabled")
    if disabled is None:
        disabled = _flag(mask, "disabled")
    summary: dict[str, Any] = {"present": True, "enabled": not bool(disabled)}
    if record:
        summary["rect"] = _bounds(mask)
        background = getattr(mask, "background_color", None)
        summary["defaultFill"] = int(background) if background is not None else 0
        inverted = _flag(getattr(mask, "flags", None), "invert_mask", "inverted")
        summary["inverted"] = bool(inverted)
    return summary


def build_layer_node(layer: Any, parent_path: str) -> dict[str, Any]:
    name = str(layer.name)
    path = f"{parent_path}/{name}" if parent_path else name
    is_group = layer.is_group()
    blend_key = _blend_key(layer)

    node: dict[str, Any] = {
        "path": path,
        "name": name,
        "type": _layer_type(layer),
    }

    # A boundless kind (a group, an adjustment) must OMIT bounds rather than carry
    # a zero rect: the comparison renders "no bounds" as "-", so {0,0,0,0} would
    # read as a real mismatch instead of "nothing asserted here".
    bounds = _bounds(layer)
    if not is_group and bounds["w"] > 0 and bounds["h"] > 0:
        node["bounds"] = bounds

    node |= {
        "blendMode": _blend_mode(layer),
        "psdBlendKey": blend_key,
        "opacity255": int(layer.opacity),
        "fillOpacity255": _fill_opacity(layer),
        "visible": bool(layer.visible),
        # psd-tools >=1.19 renamed `clipping_layer` to `clipping`; the old name
        # still works but warns. Prefer the new one, fall back for older versions.
        "clipToBelow": bool(
            getattr(layer, "clipping", None)
            if hasattr(layer, "clipping")
            else getattr(layer, "clipping_layer", False)
        ),
    }

    if is_group:
        # A PSD group blending as "pass" is a pass-through group; anything else
        # (normally "norm") is an isolated group.
        node["isolated"] = blend_key != "pass"
    else:
        adjustment = _adjustment(layer)
        if adjustment is not None:
            node["adjustmentKind"], node["adjustmentParams"] = adjustment

    node["mask"] = _mask_summary(layer, record=False)

    if is_group:
        # psd-tools appends children in raw layer-record order, and PSD stores
        # layer records BOTTOM-TO-TOP — the same order the engine stores children.
        # Verified empirically: with two opaque full-canvas layers, the layer at
        # record index 1 wins the composite, i.e. index 1 is above index 0.
        # So this is a straight pass-through; do NOT reverse it.
        node["children"] = [build_layer_node(child, path) for child in layer]

    return node


def _fill_opacity(layer: Any) -> int:
    """Fill opacity byte from the iOpa tagged block; 255 when absent."""
    try:
        from psd_tools.constants import Tag

        blocks = getattr(layer, "tagged_blocks", None)
        if blocks is None:
            return 255
        value = blocks.get_data(Tag.BLEND_FILL_OPACITY, None)
    except Exception:
        return 255
    return 255 if value is None else int(value)


def build_layers(psd: PSDImage) -> list[dict[str, Any]]:
    # Same bottom-to-top reasoning as build_layer_node: no reversal.
    return [build_layer_node(layer, "") for layer in psd]


def _section_type(record: Any) -> int:
    """lsct section type: 0 normal, 1 open folder, 2 closed folder, 3 divider."""
    try:
        from psd_tools.constants import Tag

        blocks = record.tagged_blocks
        if blocks is None:
            return 0
        divider = blocks.get_data(Tag.SECTION_DIVIDER_SETTING, None)
        divider = blocks.get_data(Tag.NESTED_SECTION_DIVIDER_SETTING, divider)
    except Exception:
        return 0
    if divider is None:
        return 0
    kind = getattr(divider, "kind", divider)
    return int(getattr(kind, "value", kind))


def build_psd_records(psd: PSDImage) -> list[dict[str, Any]]:
    """Flat layer records in FILE order.

    Deliberately read straight off the parsed record structure rather than the
    API tree: the engine model is lossy for mask rects, section dividers and
    channel IDs, so these have to be asserted at record level.
    """
    layer_info = psd._record.layer_and_mask_information.layer_info
    if layer_info is None or not layer_info.layer_records:
        return []

    records = []
    for index, record in enumerate(layer_info.layer_records):
        blend_key = _blend_key(record)
        section_type = _section_type(record)
        clipping = getattr(record, "clipping", 0)
        entry: dict[str, Any] = {
            "index": index,
            "name": str(record.name),
            "sectionType": section_type,
            "bounds": _bounds(record),
            "channelIds": [
                int(getattr(channel.id, "value", channel.id))
                for channel in record.channel_info
            ],
            "opacity255": int(record.opacity),
            "visible": bool(_flag(record.flags, "visible")),
            "clipping": bool(int(getattr(clipping, "value", clipping))),
            "blendMode": _blend_mode(record),
            "psdBlendKey": blend_key,
        }
        if section_type in (1, 2):
            entry["passThrough"] = blend_key == "pass"
        mask = _mask_summary(record, record=True)
        entry["mask"] = mask if mask.get("present") else None
        records.append(entry)
    return records


# ── pixel sampling ────────────────────────────────────────────────────────────


def build_composite_samples(psd: PSDImage, budget: int) -> tuple[list[dict], str | None]:
    """Sample the flattened composite. Returns (samples, note-on-failure)."""
    try:
        image = psd.composite()
    except Exception as exc:
        # Known case: psd-tools raises IndexError compositing extreme aspect
        # ratios (e.g. the 30000x2 near-limit fixture). Degrade, don't crash.
        return [], f"psd-tools could not composite this file: {type(exc).__name__}: {exc}"
    if image is None:
        return [], "psd-tools returned no composite"

    mode = image.mode
    width, height = image.size

    def sampler(x: int, y: int) -> list[int] | None:
        return _rgba_from_pixel(image.getpixel((x, y)), mode)

    samples = [
        {"x": x, "y": y, "rgba": rgba, "tolerance": 1}
        for x, y, rgba in _pick_distinct(sampler, width, height, budget)
    ]
    return samples, None


def _iter_pixel_layers(layers: list[Any]) -> list[Any]:
    out = []
    for layer in layers:
        if layer.is_group():
            out.extend(_iter_pixel_layers(list(layer)))
        elif layer.width > 0 and layer.height > 0:
            out.append(layer)
    return out


def build_layer_samples(psd: PSDImage, budget: int) -> list[dict[str, Any]]:
    """Sample individual layers in LAYER-LOCAL coordinates."""
    pixel_layers = _iter_pixel_layers(list(psd))
    if not pixel_layers or budget <= 0:
        return []

    per_layer = max(1, budget // len(pixel_layers))
    samples: list[dict[str, Any]] = []

    for layer in pixel_layers:
        if len(samples) >= budget:
            break
        try:
            array = layer.numpy()
        except Exception:
            continue
        if array is None or array.size == 0:
            continue
        array = np.asarray(array)
        height, width = array.shape[0], array.shape[1]
        channels = array.shape[2] if array.ndim == 3 else 1

        def sampler(x: int, y: int, _a=array, _c=channels) -> list[int] | None:
            pixel = _a[y, x]
            values = [_to_u8(v) for v in np.atleast_1d(pixel)]
            if _c == 1:
                return [values[0], values[0], values[0], 255]
            if _c == 2:
                return [values[0], values[0], values[0], values[1]]
            if _c == 3:
                return values[:3] + [255]
            return values[:4]

        room = min(per_layer, budget - len(samples))
        path = _layer_path(layer)
        for x, y, rgba in _pick_distinct(sampler, width, height, room):
            samples.append(
                {
                    "layer": path,
                    "space": "layer",
                    "x": x,
                    "y": y,
                    "rgba": rgba,
                    "tolerance": 0,
                }
            )
    return samples


def _layer_path(layer: Any) -> str:
    parts = [str(layer.name)]
    parent = getattr(layer, "parent", None)
    while parent is not None and not isinstance(parent, PSDImage):
        parts.append(str(parent.name))
        parent = getattr(parent, "parent", None)
    return "/".join(reversed(parts))


# ── known gaps ────────────────────────────────────────────────────────────────


def parse_known_gap(spec: str) -> dict[str, Any]:
    """Parse ``path=...;want=...;actual=...;reason=...`` into a knownGaps entry."""
    fields: dict[str, str] = {}
    for chunk in spec.split(";"):
        chunk = chunk.strip()
        if not chunk:
            continue
        if "=" not in chunk:
            sys.exit(f"error: malformed --known-gap segment {chunk!r} (expected key=value)")
        key, _, value = chunk.partition("=")
        fields[key.strip()] = value.strip()

    missing = [k for k in ("path", "want", "actual", "reason") if k not in fields]
    if missing:
        sys.exit(f"error: --known-gap {spec!r} is missing: {', '.join(missing)}")

    def coerce(text: str) -> Any:
        try:
            return json.loads(text)
        except ValueError:
            return text

    return {
        "path": fields["path"],
        "want": coerce(fields["want"]),
        "actual": coerce(fields["actual"]),
        "reason": fields["reason"],
    }


# ── main ──────────────────────────────────────────────────────────────────────


def build_provenance(args: argparse.Namespace, today: str) -> dict[str, Any]:
    return {
        "sourceTool": args.source_tool,
        "sourceToolVersion": args.source_tool_version,
        "generatedBy": args.generated_by,
        "generatedOn": today,
        "author": args.author,
        "license": args.license,
        "licenseNote": args.license_note,
        "redistributable": True,
    }


def build_rejection_sidecar(
    args: argparse.Namespace, psd: PSDImage, filename: str, today: str
) -> dict[str, Any]:
    """Sidecar for a NEGATIVE fixture: a valid PSD that Agogo must refuse.

    Everything else this script derives — the document header, the layer tree,
    the records, the samples, the re-export — describes a successful import. None
    of it exists behind a rejection, so emitting any of it would be asserting
    facts the harness can never reach. The only expectation is the error itself,
    pinned to a substring so that a panic turned into a generic message cannot
    satisfy it.
    """
    return {
        "schemaVersion": SCHEMA_VERSION,
        "id": args.id,
        "file": filename,
        "description": args.description or "",
        "provenance": build_provenance(args, today),
        "expectationSource": {
            "tool": "psd-tools",
            "toolVersion": psd_tools.__version__,
            "script": SCRIPT_REPO_PATH,
            "derivedOn": today,
            "method": (
                "psd-tools PSDImage.open() confirms the container is valid and "
                "readable by a third-party implementation; the expectation is that "
                "Agogo refuses it anyway. No Agogo code in the loop."
            ),
            "manualChecks": list(args.manual_check),
        },
        "assert": ["import"],
        "import": {
            "expect": "error",
            "errorContains": args.expect_import_error,
        },
        **({"fuzz": {"seed": True}} if args.fuzz_seed else {}),
        "knownGaps": [parse_known_gap(spec) for spec in args.known_gap],
    }


def build_sidecar(args: argparse.Namespace, psd: PSDImage, filename: str) -> dict[str, Any]:
    today = args.date or _dt.date.today().isoformat()

    if args.expect_import_error is not None:
        return build_rejection_sidecar(args, psd, filename, today)

    composite_budget = max(0, args.max_samples // 2)
    layer_budget = args.max_samples - composite_budget

    composite_samples, composite_note = build_composite_samples(psd, composite_budget)
    layer_samples = build_layer_samples(psd, layer_budget)
    layers = build_layers(psd)
    records = build_psd_records(psd)

    # The `assert` list and the data blocks must agree BOTH WAYS: the Go loader
    # rejects a scope that is asserted but empty, and equally rejects data that is
    # present but unasserted. So we never write the two independently — `assert` is
    # derived from what actually got emitted, and --assert only restricts which
    # optional scopes are collected at all. That makes an inconsistent sidecar
    # unrepresentable rather than merely discouraged.
    requested = list(dict.fromkeys(args.assert_scopes))
    unknown = [s for s in requested if s not in ALL_SCOPES]
    if unknown:
        sys.exit(f"error: unknown assertion scope(s): {', '.join(unknown)}")

    def wanted(scope: str) -> bool:
        return not requested or scope in requested

    if not wanted("psdRecords"):
        records = []
    if not wanted("layerPixels"):
        layer_samples = []
    if not wanted("compositePixels"):
        composite_samples = []

    # An empty optional scope asserts nothing, so it is omitted entirely rather
    # than written as [] — that keeps "absent" and "asserted to be empty" distinct.
    optional_data: dict[str, Any] = {}
    if records:
        optional_data["psdRecords"] = records
    if layer_samples:
        optional_data["layerPixels"] = layer_samples
    if composite_samples:
        optional_data["compositePixels"] = composite_samples

    scopes = list(REQUIRED_SCOPES) + list(optional_data)
    if args.writer_format:
        scopes.append("writer")

    missing = [s for s in requested if s not in scopes and s != "writer"]
    if missing:
        sys.exit(
            f"error: --assert named {', '.join(missing)}, but this fixture yields no "
            f"data for those scope(s); drop the flag or pick a different fixture"
        )

    manual_checks: list[str] = list(args.manual_check)
    if composite_note:
        manual_checks.append(composite_note)

    sidecar: dict[str, Any] = {
        "schemaVersion": SCHEMA_VERSION,
        "id": args.id,
        "file": filename,
        "description": args.description or "",
        "provenance": build_provenance(args, today),
        "expectationSource": {
            "tool": "psd-tools",
            "toolVersion": psd_tools.__version__,
            "script": SCRIPT_REPO_PATH,
            "derivedOn": today,
            "method": (
                "psd-tools PSDImage.open() for structure and layer records; "
                "composite() for compositePixels; layer.numpy() for layerPixels. "
                "No Agogo code in the loop."
            ),
            "manualChecks": manual_checks,
        },
        "assert": scopes,
        "document": build_document(psd),
        # psd-tools cannot know which diagnostics the engine SHOULD raise, so the
        # golden warning set is a human decision. [] means "must import clean";
        # a reviewer must confirm that against an actual import before this
        # fixture is added to the corpus.
        #
        # --warning supplies the non-empty case. It is a flag rather than a hand
        # edit of the generated JSON so that regenerating a fixture cannot
        # silently drop the reviewed warnings and turn a fixture that asserts a
        # diagnostic into one that asserts clean import.
        "warnings": list(args.warning),
        "layers": layers,
    }
    sidecar |= optional_data

    if args.writer_format:
        if args.lossy and not args.lossy_reason:
            sys.exit("error: --lossy requires --lossy-reason (a PLAN.md reference)")
        sidecar["writer"] = {
            "format": args.writer_format,
            "expect": args.writer_expect,
            # Like `warnings`, the allowlist is a reviewed human decision that
            # psd-tools cannot derive - it is a statement about Agogo's WRITER,
            # which this script may not look at. Supplying it on the command
            # line keeps regeneration from silently emptying it.
            "lossy": list(args.lossy),
            **({"lossyReason": args.lossy_reason} if args.lossy_reason else {}),
            "externalVerification": {
                "tool": "psd-tools",
                "toolVersion": psd_tools.__version__,
                "verifiedOn": today,
                # "pending" is the honest default: it is a claim about a check
                # that has not run. Promote it with --verification-result only
                # after `just fixtures-verify` actually passed, and record the
                # outcome in VERIFICATION.md at the same time.
                "result": args.verification_result,
                "notes": args.verification_notes
                or "run `just fixtures-verify` (tools/psdfixtures/verify_dump.py)",
            },
        }

    if args.fuzz_seed:
        sidecar["fuzz"] = {"seed": True}

    sidecar["knownGaps"] = [parse_known_gap(spec) for spec in args.known_gap]
    return sidecar


# ── serialisation ─────────────────────────────────────────────────────────────

# Biome formats every *.json in this repo (see treefmt.toml + biome.json), and
# `just ci` runs `treefmt --fail-on-change`. A sidecar emitted by plain
# json.dumps(indent=2) is NOT Biome-stable: Biome collapses short scalar arrays
# onto one line, so CI would fail the moment a sidecar is committed. Rather than
# carving the fixture directory out of the formatter, we emit what Biome would.
#
# The two rules that matter, both matching Biome's defaults here:
#   - objects stay expanded (they are expanded in the source we produce)
#   - an array of scalars is inlined when the resulting line fits lineWidth
BIOME_LINE_WIDTH = 100
BIOME_INDENT = 2


def _is_scalar(value: Any) -> bool:
    return value is None or isinstance(value, (bool, int, float, str))


def biome_dumps(value: Any, indent: int = 0, prefix_len: int = 0) -> str:
    """Serialise to JSON the way Biome would format it.

    ``prefix_len`` is the column the value starts at (indent plus any ``"key": ``),
    which is what decides whether an inlined array still fits the line.
    """
    pad = " " * indent
    inner_pad = " " * (indent + BIOME_INDENT)

    if isinstance(value, dict):
        if not value:
            return "{}"
        parts = []
        for key, item in value.items():
            key_text = json.dumps(key, ensure_ascii=False)
            child_prefix = indent + BIOME_INDENT + len(key_text) + 2
            rendered = biome_dumps(item, indent + BIOME_INDENT, child_prefix)
            parts.append(f"{inner_pad}{key_text}: {rendered}")
        return "{\n" + ",\n".join(parts) + f"\n{pad}}}"

    if isinstance(value, list):
        if not value:
            return "[]"
        if all(_is_scalar(item) for item in value):
            inline = "[" + ", ".join(json.dumps(i, ensure_ascii=False) for i in value) + "]"
            if prefix_len + len(inline) + 1 <= BIOME_LINE_WIDTH:
                return inline
        parts = [
            f"{inner_pad}{biome_dumps(item, indent + BIOME_INDENT, indent + BIOME_INDENT)}"
            for item in value
        ]
        return "[\n" + ",\n".join(parts) + f"\n{pad}]"

    return json.dumps(value, ensure_ascii=False)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Derive a PSD fixture expectation sidecar using psd-tools only.",
    )
    parser.add_argument("fixture", type=Path, help="path to the .psd/.psb fixture")
    parser.add_argument("--id", required=True, help="fixture id (sidecar basename)")
    parser.add_argument(
        "--out",
        required=True,
        help="output directory, or '-' to print the JSON to stdout",
    )
    parser.add_argument("--description", default="", help="one-line fixture description")

    group = parser.add_argument_group("provenance (who made the binary)")
    group.add_argument("--source-tool", required=True, help="e.g. ImageMagick, GIMP")
    group.add_argument("--source-tool-version", required=True, help="e.g. 6.9.12-98")
    group.add_argument(
        "--generated-by",
        required=True,
        help="repo-relative generator reference, e.g. "
        "tools/psdfixtures/generate_imagemagick.sh#fixture_rgb8_flat_rle",
    )
    group.add_argument(
        "--author",
        default="Christian Budde <christian.wilhelm.budde@meko.de>",
    )
    group.add_argument("--license", default="CC0-1.0", help="SPDX identifier")
    group.add_argument(
        "--license-note",
        default=(
            "Self-authored from scratch; no third-party artwork; "
            "CC0-1.0 for redistribution in this repository."
        ),
    )

    parser.add_argument(
        "--assert",
        dest="assert_scopes",
        action="append",
        default=[],
        metavar="SCOPE",
        help=f"assertion scope, repeatable. One of: {', '.join(ALL_SCOPES)}. "
        "Defaults to the scopes that actually have data.",
    )
    parser.add_argument(
        "--max-samples",
        type=int,
        default=16,
        help="total pixel samples (layerPixels + compositePixels), default 16",
    )
    parser.add_argument(
        "--known-gap",
        action="append",
        default=[],
        metavar="SPEC",
        help="'path=...;want=...;actual=...;reason=...', repeatable",
    )
    parser.add_argument(
        "--manual-check",
        action="append",
        default=[],
        metavar="TEXT",
        help="note for expectationSource.manualChecks, repeatable",
    )
    parser.add_argument(
        "--verification-result",
        default="pending",
        choices=("pending", "pass", "partial", "fail"),
        help=(
            "writer.externalVerification.result. Leave at the default until "
            "`just fixtures-verify` has actually run against this fixture"
        ),
    )
    parser.add_argument(
        "--verification-notes",
        default=None,
        metavar="TEXT",
        help="what the external reader saw, for writer.externalVerification.notes",
    )
    parser.add_argument(
        "--lossy",
        action="append",
        default=[],
        metavar="PATH",
        help="a writer.lossy allowlist path, repeatable; requires --lossy-reason",
    )
    parser.add_argument(
        "--lossy-reason",
        default=None,
        metavar="TEXT",
        help="why the --lossy paths differ, with a PLAN.md reference",
    )
    parser.add_argument(
        "--warning",
        action="append",
        default=[],
        metavar="TEXT",
        help=(
            "an import warning this fixture must produce, repeatable. psd-tools "
            "cannot derive these, so they are a reviewed human decision; the "
            "default of none means the fixture must import clean"
        ),
    )
    parser.add_argument(
        "--expect-import-error",
        default=None,
        metavar="SUBSTRING",
        help="mark this as a NEGATIVE fixture: Agogo must refuse the file with an "
        "error containing SUBSTRING. The sidecar then asserts the 'import' scope "
        "only, and carries no document, layer, record, pixel or writer data, "
        "because none of those exist for a file that never imports.",
    )
    parser.add_argument(
        "--fuzz-seed",
        action="store_true",
        help=(
            "emit \"fuzz\": {\"seed\": true}, putting this fixture into the Go fuzz "
            "seed corpus. Opt-in: a sidecar without a fuzz block seeds nothing, so "
            "forgetting this flag makes the seeding silently vacuous."
        ),
    )
    parser.add_argument("--writer-format", choices=("psd", "psb"), default=None)
    parser.add_argument("--writer-expect", choices=("match", "lossy"), default="lossy")
    parser.add_argument("--date", default=None, help="override the ISO date (testing)")

    args = parser.parse_args(argv)
    if args.max_samples < 0:
        parser.error("--max-samples must not be negative")
    if args.expect_import_error is not None:
        if not args.expect_import_error.strip():
            parser.error("--expect-import-error needs a non-empty substring; "
                         "'any error will do' is not an assertion")
        if args.writer_format:
            parser.error("--expect-import-error and --writer-format are mutually "
                         "exclusive: a file that never imports has no re-export")
        extra = [s for s in args.assert_scopes if s != "import"]
        if extra:
            parser.error(
                f"--expect-import-error cannot be combined with --assert "
                f"{', '.join(extra)}; a rejection fixture asserts 'import' only"
            )
    return args


def main(argv: list[str]) -> int:
    args = parse_args(argv)

    if not args.fixture.is_file():
        sys.exit(f"error: {args.fixture} is not a file")

    try:
        psd = PSDImage.open(args.fixture)
    except Exception as exc:
        sys.exit(f"error: psd-tools could not open {args.fixture}: {type(exc).__name__}: {exc}")

    sidecar = build_sidecar(args, psd, args.fixture.name)

    # Key order comes from insertion order above and stays stable between runs,
    # so a regenerated sidecar produces a reviewable diff instead of a reshuffle.
    text = biome_dumps(sidecar) + "\n"

    if args.out == "-":
        sys.stdout.write(text)
        return 0

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / f"{args.id}.expected.json"
    out_path.write_text(text, encoding="utf-8")
    print(f"wrote {out_path}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
