#!/usr/bin/env bash
#
# generate_imagemagick.sh — generate the FLAT (composite-only) PSD/PSB fixtures.
#
# Phase S.10.1, step 2. See README.md for the full rationale.
#
# IMPORTANT: ImageMagick is used ONLY for flat, single-layer fixtures. Its LAYERED
# PSD writer is broken (it emits byte-swapped blend mode keys such as "mron" instead
# of "norm", which psd-tools rejects outright). Layered fixtures come from GIMP —
# see generate_gimp.scm.
#
# Usage:
#   ./generate_imagemagick.sh <output-dir> [fixture-id ...]
#
# With no fixture ids, every fixture is generated. Each fixture is a shell function
# named `fixture_<id-with-underscores>` so that a sidecar's provenance.generatedBy can
# reference it precisely, e.g.
#   "generatedBy": "tools/psdfixtures/generate_imagemagick.sh#fixture_rgb8_flat_rle"
#
set -euo pipefail

CONVERT="${CONVERT:-convert}"

SCRIPT_NAME="$(basename "$0")"

# Hard size budget. Fixtures are committed to the repository, so keep them tiny.
SOFT_CAP_BYTES=24576  # 24 KiB — warn above this
HARD_CAP_BYTES=131072 # 128 KiB — fail above this

die() {
	printf '%s: error: %s\n' "$SCRIPT_NAME" "$*" >&2
	exit 1
}

log() {
	printf '%s: %s\n' "$SCRIPT_NAME" "$*" >&2
}

# check_size <path> — enforce the fixture size budget.
check_size() {
	local path="$1" bytes
	bytes="$(stat -c '%s' "$path")"
	if [ "$bytes" -gt "$HARD_CAP_BYTES" ]; then
		die "$path is $bytes bytes, above the hard cap of $HARD_CAP_BYTES"
	fi
	if [ "$bytes" -gt "$SOFT_CAP_BYTES" ]; then
		log "warning: $path is $bytes bytes, above the soft cap of $SOFT_CAP_BYTES"
	fi
	printf '  %-24s %8s bytes\n' "$(basename "$path")" "$bytes"
}

# ── Fixtures ──────────────────────────────────────────────────────────────────
#
# Every fixture paints several distinct solid colour blocks. A single uniform
# colour would make every pixel sample pass trivially, including a sample taken
# from a completely wrong coordinate, so it is deliberately avoided.

# 16x16 RGB/8, four 8x8 quadrants, uncompressed (raw) image data.
#
#   (0,0) #3366cc   (8,0) #cc3366
#   (0,8) #66cc33   (8,8) #eeee11
fixture_rgb8_flat_raw() {
	local out="$1/rgb8-flat-raw.psd"
	"$CONVERT" \
		\( -size 8x8 xc:'#3366cc' -size 8x8 xc:'#cc3366' +append \) \
		\( -size 8x8 xc:'#66cc33' -size 8x8 xc:'#eeee11' +append \) \
		-append \
		-colorspace sRGB -type TrueColor -depth 8 -compress None \
		"$out"
	check_size "$out"
}

# Same geometry and colours as rgb8-flat-raw, but RLE (PackBits) compressed, so the
# two exercise the engine's two image-data code paths against identical pixels.
fixture_rgb8_flat_rle() {
	local out="$1/rgb8-flat-rle.psd"
	"$CONVERT" \
		\( -size 8x8 xc:'#3366cc' -size 8x8 xc:'#cc3366' +append \) \
		\( -size 8x8 xc:'#66cc33' -size 8x8 xc:'#eeee11' +append \) \
		-append \
		-colorspace sRGB -type TrueColor -depth 8 -compress RLE \
		"$out"
	check_size "$out"
}

# 16x16 Grayscale/8, four 8x8 quadrants of distinct luminance (0x20/0x60/0xa0/0xe0).
#
# This is also the ONLY fixture carrying an explicit resolution. ImageMagick writes
# no image resource block at all unless -density is given, so every other fixture
# exercises the "no 0x03ED resource -> fall back to 72 dpi" path, and this one
# exercises actually parsing resource 1005. Verified: -density 144 does produce a
# RESOLUTION_INFO resource.
fixture_gray8_flat() {
	local out="$1/gray8-flat.psd"
	"$CONVERT" \
		\( -size 8x8 xc:'gray(32)' -size 8x8 xc:'gray(96)' +append \) \
		\( -size 8x8 xc:'gray(160)' -size 8x8 xc:'gray(224)' +append \) \
		-append \
		-colorspace Gray -type Grayscale -depth 8 -compress RLE \
		-units PixelsPerInch -density 144 \
		"$out"
	check_size "$out"
}

# 16x16 RGB/8 written as a PSB (large document format). Same pixels as the PSD
# variants, so a PSD/PSB round-trip difference is attributable to the container.
fixture_psb_flat() {
	local out="$1/psb-flat.psb"
	"$CONVERT" \
		\( -size 8x8 xc:'#3366cc' -size 8x8 xc:'#cc3366' +append \) \
		\( -size 8x8 xc:'#66cc33' -size 8x8 xc:'#eeee11' +append \) \
		-append \
		-colorspace sRGB -type TrueColor -depth 8 -compress RLE \
		"$out"
	check_size "$out"
}

# 30000x2 RGB/8 — a degenerate aspect ratio that lands exactly on the engine's
# PSDMaxDimension guard (30000) without allocating a 30000x30000 surface. Two solid
# 15000x2 halves, so it RLE-packs down to a few KiB.
#
# Do NOT raise the second dimension: 30000x30000 would be a 3.6 GB decode.
fixture_near_psd_limit() {
	local out="$1/near-psd-limit.psd"
	"$CONVERT" \
		-size 15000x2 xc:'#3366cc' \
		-size 15000x2 xc:'#cc3366' \
		+append \
		-colorspace sRGB -type TrueColor -depth 8 -compress RLE \
		"$out"
	check_size "$out"
}

# 16x16 RGB/16 — a NEGATIVE fixture. The engine must reject 16-bit PSDs with an
# actionable error rather than mis-decoding them, so this file exists to be refused.
# ImageMagick at Q16 writes 16 bits per channel whenever -depth 8 is omitted.
fixture_depth16_rejected() {
	local out="$1/depth16-rejected.psd"
	"$CONVERT" \
		\( -size 8x8 xc:'#3366cc' -size 8x8 xc:'#cc3366' +append \) \
		\( -size 8x8 xc:'#66cc33' -size 8x8 xc:'#eeee11' +append \) \
		-append \
		-colorspace sRGB -type TrueColor -depth 16 -compress RLE \
		"$out"
	check_size "$out"
}

ALL_FIXTURES=(
	rgb8-flat-raw
	rgb8-flat-rle
	gray8-flat
	psb-flat
	near-psd-limit
	depth16-rejected
)

usage() {
	cat >&2 <<EOF
usage: $SCRIPT_NAME <output-dir> [fixture-id ...]

Fixtures: ${ALL_FIXTURES[*]}
EOF
	exit 2
}

main() {
	[ $# -ge 1 ] || usage
	local outdir="$1"
	shift

	command -v "$CONVERT" >/dev/null 2>&1 || die "'$CONVERT' not found on PATH"

	mkdir -p "$outdir"

	local wanted=()
	if [ $# -eq 0 ]; then
		wanted=("${ALL_FIXTURES[@]}")
	else
		wanted=("$@")
	fi

	log "ImageMagick: $("$CONVERT" -version | head -n1)"

	local id fn
	for id in "${wanted[@]}"; do
		fn="fixture_${id//-/_}"
		if ! declare -F "$fn" >/dev/null; then
			die "unknown fixture '$id' (no function $fn)"
		fi
		log "generating $id"
		"$fn" "$outdir"
	done

	log "done; wrote ${#wanted[@]} fixture(s) to $outdir"
}

main "$@"
