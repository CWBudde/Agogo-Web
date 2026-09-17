#!/usr/bin/env bash
#
# generate_gimp.sh — driver for generate_gimp.scm (the LAYERED PSD fixtures).
#
# Phase S.10.1, step 2. See README.md.
#
# WARNING: on the machine this was authored on, every gimp-file-save call hangs
# indefinitely and does not respond to SIGTERM, so this driver has NOT been seen to
# produce a file. It is written to the documented GIMP 3 batch form; the one
# unverified call is psdfix-save in generate_gimp.scm.
#
# Because a hung GIMP ignores SIGTERM, `timeout` is always used with -k so that a
# SIGKILL follows. Never run GIMP here without it.
#
# Usage:
#   ./generate_gimp.sh <output-dir> [scheme-function ...]
#
# With no function names, psdfix-generate-all runs. Otherwise each named function
# is called with the output directory, e.g.
#   ./generate_gimp.sh /tmp/fx psdfix-mask-offset
#
set -euo pipefail

GIMP="${GIMP:-gimp}"
GIMP_TIMEOUT="${GIMP_TIMEOUT:-300}"
GIMP_KILL_AFTER="${GIMP_KILL_AFTER:-15}"

SCRIPT_NAME="$(basename "$0")"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SCM="$SCRIPT_DIR/generate_gimp.scm"

die() {
	printf '%s: error: %s\n' "$SCRIPT_NAME" "$*" >&2
	exit 1
}

log() {
	printf '%s: %s\n' "$SCRIPT_NAME" "$*" >&2
}

usage() {
	cat >&2 <<EOF
usage: $SCRIPT_NAME <output-dir> [scheme-function ...]

Runs generate_gimp.scm under GIMP 3 in batch mode. With no function names,
psdfix-generate-all is called.

Environment:
  GIMP             gimp executable (default: gimp)
  GIMP_TIMEOUT     seconds before SIGTERM (default: 300)
  GIMP_KILL_AFTER  seconds after SIGTERM before SIGKILL (default: 15)
EOF
	exit 2
}

main() {
	[ $# -ge 1 ] || usage
	local outdir="$1"
	shift

	command -v "$GIMP" >/dev/null 2>&1 || die "'$GIMP' not found on PATH"
	[ -f "$SCM" ] || die "$SCM not found"

	mkdir -p "$outdir"
	# GIMP resolves the path itself, so hand it an absolute one.
	outdir="$(cd -- "$outdir" && pwd)"

	local calls=()
	if [ $# -eq 0 ]; then
		calls=("(psdfix-generate-all \"$outdir\")")
	else
		local fn
		for fn in "$@"; do
			calls+=("($fn \"$outdir\")")
		done
	fi

	# Script-Fu is fed as one expression: load the library, then run the calls.
	local script
	script="(begin (load \"$SCM\") ${calls[*]})"

	log "GIMP: $("$GIMP" --version 2>/dev/null | head -n1 || echo unknown)"
	log "running: ${calls[*]}"

	# --batch-interpreter=plug-in-script-fu-eval is REQUIRED: without it, GIMP 3
	# expects a script name rather than an expression. A Script-Fu error makes GIMP
	# hang instead of exiting, hence timeout -k.
	if ! timeout -k "$GIMP_KILL_AFTER" "$GIMP_TIMEOUT" \
		"$GIMP" -i --batch-interpreter=plug-in-script-fu-eval \
		-b "$script" \
		-b '(gimp-quit 0)'; then
		die "GIMP exited non-zero or timed out after ${GIMP_TIMEOUT}s. \
A Script-Fu error or a blocking export both look like this; \
see the UNVERIFIED banner in generate_gimp.scm."
	fi

	log "done; output in $outdir"
	ls -l "$outdir"
}

main "$@"
