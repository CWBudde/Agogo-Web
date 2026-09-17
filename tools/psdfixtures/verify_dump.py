#!/usr/bin/env python3
"""Re-read Agogo-written PSD/PSB files with a reader that is not Agogo's.

Phase S.10.1, step 2. This is the external half of the round-trip claim
"PSDs Agogo writes open in other software": the Go side re-exports each fixture
into ``packages/engine-wasm/internal/io/psdfixture/_dump/``, and this script opens
every file there with psd-tools (and optionally ImageMagick's ``identify``, a third
independent implementation) and prints what they see.

A file that psd-tools cannot parse AT ALL is a hard failure — it means Agogo
emitted something no other reader accepts. Structural differences from the source
fixture are NOT judged here; that is the sidecar's ``writer`` scope. This script
reports, a human (or the Go harness) decides.

Exit codes:
    0  every file parsed
    1  at least one file failed to parse
    2  the dump directory is missing or empty
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any

try:
    import psd_tools
    from psd_tools import PSDImage
    from psd_tools.constants import Tag
except ImportError as exc:  # pragma: no cover
    sys.exit(f"error: {exc}. Install with: pip install psd-tools")

DEFAULT_DUMP_DIR = "packages/engine-wasm/internal/io/psdfixture/_dump"


def _blend_key(obj: Any) -> str:
    mode = getattr(obj, "blend_mode", None)
    raw = getattr(mode, "value", mode)
    if isinstance(raw, bytes):
        return raw.decode("ascii", "replace")
    return str(raw)


def _describe_mask(layer: Any) -> str:
    mask = getattr(layer, "mask", None)
    if mask is None:
        return "mask=none"
    disabled = getattr(getattr(mask, "flags", None), "mask_disabled", None)
    if disabled is None:
        disabled = getattr(mask, "disabled", False)
    rect = (
        getattr(mask, "left", 0),
        getattr(mask, "top", 0),
        getattr(mask, "right", 0),
        getattr(mask, "bottom", 0),
    )
    background = getattr(mask, "background_color", None)
    return (
        f"mask=present enabled={not bool(disabled)} rect={rect} "
        f"defaultFill={background}"
    )


def _fill_opacity(layer: Any) -> int:
    """Fill opacity byte from the iOpa tagged block; 255 when absent.

    Nothing is caught here on purpose. An unreadable block, or a psd-tools that
    no longer knows this tag, must fail the run loudly - swallowing it would
    print fill=255 for a file whose fill opacity was never verified at all,
    which is indistinguishable from the answer this script exists to check.
    """
    blocks = getattr(layer, "tagged_blocks", None)
    if blocks is None:
        return 255
    value = blocks.get_data(Tag.BLEND_FILL_OPACITY, None)
    return 255 if value is None else int(value)


def print_tree(layer: Any, depth: int = 0) -> None:
    pad = "  " * (depth + 1)
    kind = "group" if layer.is_group() else str(getattr(layer, "kind", "?"))
    print(
        f"{pad}- {layer.name!r} kind={kind} blend={_blend_key(layer)!r} "
        f"opacity={layer.opacity} fill={_fill_opacity(layer)} visible={layer.visible} "
        f"bounds={(layer.left, layer.top, layer.right, layer.bottom)} "
        f"{_describe_mask(layer)}"
    )
    if layer.is_group():
        # Bottom-to-top, the order the records are stored in.
        for child in layer:
            print_tree(child, depth + 1)


def sample_composite(psd: PSDImage, count: int = 5) -> str:
    try:
        image = psd.composite()
    except Exception as exc:
        return f"  composite: UNAVAILABLE ({type(exc).__name__}: {exc})"
    if image is None:
        return "  composite: UNAVAILABLE (None)"
    width, height = image.size
    last_x, last_y = max(0, width - 1), max(0, height - 1)
    points = [(0, 0), (last_x, 0), (0, last_y), (last_x, last_y), (width // 2, height // 2)]
    seen: list[tuple[int, int]] = []
    for point in points[:count]:
        if point not in seen:
            seen.append(point)
    parts = [f"{p}={image.getpixel(p)}" for p in seen]
    return f"  composite: mode={image.mode} size={image.size} " + " ".join(parts)


def run_identify(path: Path) -> str | None:
    """ImageMagick's view — a third implementation, purely informational."""
    identify = shutil.which("identify")
    if identify is None:
        return None
    try:
        result = subprocess.run(
            [identify, "-quiet", str(path)],
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return "  identify: TIMEOUT"
    output = (result.stdout or result.stderr).strip()
    if not output:
        return f"  identify: no output (exit {result.returncode})"
    lines = output.splitlines()
    shown = "\n".join(f"  identify: {line}" for line in lines[:8])
    if len(lines) > 8:
        shown += f"\n  identify: ... ({len(lines) - 8} more)"
    return shown


def verify_file(path: Path, *, use_identify: bool) -> bool:
    print(f"\n=== {path.name} ({path.stat().st_size} bytes)")
    try:
        psd = PSDImage.open(path)
    except Exception as exc:
        print(f"  FAIL: psd-tools could not parse this file: {type(exc).__name__}: {exc}")
        return False

    header = psd._record.header
    print(
        f"  document: {psd.width}x{psd.height} colorMode={psd.color_mode} "
        f"depth={header.depth} channels={header.channels} "
        f"version={header.version} ({'PSB' if header.version == 2 else 'PSD'})"
    )

    layer_info = psd._record.layer_and_mask_information.layer_info
    record_count = len(layer_info.layer_records) if layer_info else 0
    # PSD stores layer_count negative when the first alpha channel holds the
    # merged-result transparency, so the sign carries meaning, not the count.
    raw_count = layer_info.layer_count if layer_info else 0
    print(f"  layer records: {record_count} (raw layer_count={raw_count})")

    print("  layer tree (bottom-to-top):")
    if len(psd) == 0:
        print("    (no API layers)")
    for layer in psd:
        print_tree(layer)

    print(sample_composite(psd))

    if use_identify:
        identify_output = run_identify(path)
        if identify_output:
            print(identify_output)

    return True


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "dump_dir",
        nargs="?",
        default=DEFAULT_DUMP_DIR,
        type=Path,
        help=f"directory of Agogo-written PSD/PSB files (default: {DEFAULT_DUMP_DIR})",
    )
    parser.add_argument(
        "--identify",
        action="store_true",
        help="also run ImageMagick's identify, a third independent implementation",
    )
    args = parser.parse_args(argv)

    if not args.dump_dir.is_dir():
        print(
            f"error: {args.dump_dir} does not exist.\n"
            "Run the Go writer round-trip test first; it populates the _dump directory.",
            file=sys.stderr,
        )
        return 2

    files = sorted(
        p for p in args.dump_dir.iterdir() if p.suffix.lower() in (".psd", ".psb")
    )
    if not files:
        print(f"error: no .psd/.psb files in {args.dump_dir}", file=sys.stderr)
        return 2

    print(f"verify_dump.py — psd-tools {psd_tools.__version__}")
    print(f"reading {len(files)} file(s) from {args.dump_dir}")

    failures = [p.name for p in files if not verify_file(p, use_identify=args.identify)]

    print(f"\n--- {len(files) - len(failures)}/{len(files)} file(s) parsed")
    if failures:
        print("FAILED: " + ", ".join(failures))
        return 1
    print("all files parsed by an independent reader")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
