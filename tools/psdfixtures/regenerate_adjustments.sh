#!/usr/bin/env bash
#
# Regenerate the three adjustment fixtures and their sidecars end to end.
#
# These three have it as a script rather than as a README command because their
# sidecars carry ~30 writer.lossy paths each, plus the reviewed warnings. All of
# that is passed on the command line on purpose - see the "Sidecar notes"
# section of README.md - and retyping it by hand is exactly the silent drop
# those flags exist to prevent.
#
# Usage:  tools/psdfixtures/regenerate_adjustments.sh [scratch-dir]
#         PY=.venv-psdfixtures/bin/python tools/psdfixtures/regenerate_adjustments.sh
#
# Afterwards, re-run the corpus tests and `just fixtures-verify`, and update
# testdata/VERIFICATION.md if what the external reader sees has changed.
set -euo pipefail
cd "$(dirname "$0")/../.."

FX="${1:-.fixtures-out}"
CORPUS=packages/engine-wasm/internal/io/psdfixture/testdata/corpus
PY="${PY:-python3}"

LOSSY_REASON="PLAN.md S.10.7: psdexport cannot yet write a native adjustment block. It emits the private AgAJ JSON block instead, so a re-exported adjustment layer reads back as a pixel layer with no adjustmentKind and no parameters, and the import warnings change accordingly. Native adjustment EXPORT is the open half of S.10.7; this batch did the read side. The channelIds and mask.rect entries are the long-standing S.10.2 writer differences: psdexport emits channels in its own fixed order and rasterizes a mask to document bounds."

rm -rf "$FX"
mkdir -p "$FX"
$PY tools/psdfixtures/generate_pytoshop.py --out "$FX" \
	--only rgb8-adjustment-core --only rgb8-adjustment-binary --only rgb8-adjustment-descriptor

common=(--source-tool pytoshop --source-tool-version 1.2.1 --writer-format psd --writer-expect lossy --fuzz-seed
	--lossy-reason "$LOSSY_REASON" --verification-result partial)

$PY tools/psdfixtures/derive_expectations.py "$FX/rgb8-adjustment-core.psd" \
	--id rgb8-adjustment-core --out "$CORPUS" "${common[@]}" \
	--generated-by "tools/psdfixtures/generate_pytoshop.py#fixture_rgb8_adjustment_core" \
	--verification-notes 'psd-tools reads every re-exported record back: the Levels layer keeps opacity 160, blend key `mul `, its mask and the cleared visible flag. The adjustments themselves read as kind=pixel, because psdexport writes AgAJ rather than levl/curv/hue2 (PLAN.md S.10.7, native export still open). See VERIFICATION.md.' \
	--description "32x24 RGB/8, Levels, Curves and Hue/Saturation adjustment layers with non-default parameters; Levels also carries a mask, clipping, opacity 160 and Multiply." \
	--warning 'layer "Levels": levl per-channel level records has no engine equivalent and was not imported' \
	--manual-check "The levl block sets the composite record plus red, green and blue; the engine holds one set of level values, so only the composite maps and the rest is warned about." \
	--manual-check "Curves sets a composite curve and a red curve, so a reader that discards the curv channel bitmap fails here." \
	$(printf -- '--lossy %s ' \
		layers\[1\].type layers\[1\].adjustmentKind \
		layers\[1\].adjustmentParams.hueShift layers\[1\].adjustmentParams.saturation \
		layers\[1\].adjustmentParams.lightness layers\[1\].adjustmentParams.colorize \
		layers\[1\].adjustmentParams.reds layers\[1\].adjustmentParams.yellows \
		layers\[1\].adjustmentParams.greens layers\[1\].adjustmentParams.cyans \
		layers\[1\].adjustmentParams.blues layers\[1\].adjustmentParams.magentas \
		layers\[2\].type layers\[2\].adjustmentKind \
		layers\[2\].adjustmentParams.points layers\[2\].adjustmentParams.redPoints \
		layers\[3\].type layers\[3\].adjustmentKind \
		layers\[3\].adjustmentParams.channel layers\[3\].adjustmentParams.gamma \
		layers\[3\].adjustmentParams.inputBlack layers\[3\].adjustmentParams.inputWhite \
		layers\[3\].adjustmentParams.outputBlack layers\[3\].adjustmentParams.outputWhite \
		warnings psdRecords\[0\].channelIds psdRecords\[3\].mask.rect)

$PY tools/psdfixtures/derive_expectations.py "$FX/rgb8-adjustment-binary.psd" \
	--id rgb8-adjustment-binary --out "$CORPUS" "${common[@]}" \
	--generated-by "tools/psdfixtures/generate_pytoshop.py#fixture_rgb8_adjustment_binary" \
	--verification-notes "psd-tools reads every re-exported record back; the adjustments read as kind=pixel for the same writer gap. See VERIFICATION.md." \
	--description "32x24 RGB/8, the remaining binary adjustment blocks one layer each: Color Balance, Channel Mixer, Selective Color, Threshold, Posterize, Invert, Photo Filter." \
	--warning 'layer "Channel Mixer": mixr constant term has no engine equivalent and was not imported' \
	--manual-check "The mixr block carries a constant term; the engine's channel mixer has source weights only, so the constant is warned about rather than folded into a weight." \
	--manual-check "Threshold and Posterize are a single padded uint16 and Invert has no payload at all; they are here because a parser that requires a non-empty payload breaks on exactly these three." \
	$(printf -- '--lossy %s ' \
		layers\[1\].type layers\[1\].adjustmentKind layers\[1\].adjustmentParams.color \
		layers\[1\].adjustmentParams.density layers\[1\].adjustmentParams.preserveLuminosity \
		layers\[2\].type layers\[2\].adjustmentKind \
		layers\[3\].type layers\[3\].adjustmentKind layers\[3\].adjustmentParams.levels \
		layers\[4\].type layers\[4\].adjustmentKind layers\[4\].adjustmentParams.threshold \
		layers\[5\].type layers\[5\].adjustmentKind layers\[5\].adjustmentParams.mode \
		layers\[5\].adjustmentParams.reds layers\[5\].adjustmentParams.yellows \
		layers\[5\].adjustmentParams.greens layers\[5\].adjustmentParams.cyans \
		layers\[5\].adjustmentParams.blues layers\[5\].adjustmentParams.magentas \
		layers\[5\].adjustmentParams.whites layers\[5\].adjustmentParams.neutrals \
		layers\[5\].adjustmentParams.blacks \
		layers\[6\].type layers\[6\].adjustmentKind \
		layers\[6\].adjustmentParams.monochrome layers\[6\].adjustmentParams.red \
		layers\[7\].type layers\[7\].adjustmentKind \
		layers\[7\].adjustmentParams.shadows layers\[7\].adjustmentParams.midtones \
		layers\[7\].adjustmentParams.highlights layers\[7\].adjustmentParams.preserveLuminosity \
		warnings psdRecords\[0\].channelIds)

$PY tools/psdfixtures/derive_expectations.py "$FX/rgb8-adjustment-descriptor.psd" \
	--id rgb8-adjustment-descriptor --out "$CORPUS" "${common[@]}" \
	--generated-by "tools/psdfixtures/generate_pytoshop.py#fixture_rgb8_adjustment_descriptor" \
	--verification-notes "psd-tools reads every re-exported record back; the adjustments read as kind=pixel for the same writer gap. See VERIFICATION.md." \
	--description "32x24 RGB/8, the descriptor-valued adjustments: Black & White, and Brightness/Contrast carrying both the obsolete brit block and the live CgEd descriptor." \
	--manual-check "Brightness/Contrast carries brit (+11/-7) and CgEd (+30/-20) with deliberately different values; the expectation pins the CgEd numbers, so a reader preferring the obsolete block fails." \
	$(printf -- '--lossy %s ' \
		layers\[1\].type layers\[1\].adjustmentKind \
		layers\[1\].adjustmentParams.brightness layers\[1\].adjustmentParams.contrast \
		layers\[1\].adjustmentParams.legacy \
		layers\[2\].type layers\[2\].adjustmentKind \
		layers\[2\].adjustmentParams.reds layers\[2\].adjustmentParams.yellows \
		layers\[2\].adjustmentParams.greens layers\[2\].adjustmentParams.cyans \
		layers\[2\].adjustmentParams.blues layers\[2\].adjustmentParams.magentas \
		layers\[2\].adjustmentParams.tint \
		warnings psdRecords\[0\].channelIds)

cp "$FX"/rgb8-adjustment-*.psd "$CORPUS"/

$PY - <<'PYEOF'
import json, hashlib, pathlib, collections
root = pathlib.Path("packages/engine-wasm/internal/io/psdfixture/testdata")
mp = root / "manifest.json"
m = json.loads(mp.read_text(), object_pairs_hook=collections.OrderedDict)
for entry in m["fixtures"]:
    if entry["id"].startswith("rgb8-adjustment-"):
        data = (root / "corpus" / entry["file"]).read_bytes()
        entry["bytes"] = len(data)
        entry["sha256"] = hashlib.sha256(data).hexdigest()
mp.write_text(json.dumps(m, indent=2) + "\n")
print("manifest refreshed")
PYEOF
# manifest.json is rewritten with plain json.dumps above; Biome formats every
# *.json in this repo and `just ci` runs treefmt --fail-on-change, so normalise
# it before leaving.
treefmt --allow-missing-formatter packages/engine-wasm/internal/io/psdfixture/testdata/manifest.json >/dev/null

echo "done"
