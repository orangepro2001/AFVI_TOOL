#!/usr/bin/env bash
# libw2k tests on Linux: logic checks + snapshot comparison of the control gallery (libw2k/tests/snapshots).
#   libw2k/tests/run.sh [--update]       needs gcc, zlib headers, fonts-dejavu-core; no Go, no application code
set -euo pipefail
cd "$(dirname "$0")/../.."
OUT=build/libw2k-test; mkdir -p "$OUT/shots"
gcc -O2 -Wall -Wno-unused-function -Ilibw2k libw2k/demo/demo.c libw2k/w2k_draw.c libw2k/w2k_ctl.c libw2k/w2k_png.c -lz -lm -o "$OUT/w2k_demo"
"$OUT/w2k_demo" --unit
if [ "${1:-}" = "--update" ]; then "$OUT/w2k_demo" --update libw2k/tests/snapshots "$OUT/shots"
else "$OUT/w2k_demo" --check libw2k/tests/snapshots "$OUT/shots"; fi
echo "== libw2k OK"
