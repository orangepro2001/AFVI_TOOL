#!/usr/bin/env bash
# Tests of the Win32 GUI on Linux. The application (gui/app.c + skin.c) is platform independent: it talks to a host
# interface, so here a Linux host runs the SAME code against the real Go core on throw-away machines.
#
#   scripts/test-gui.sh              scripted UI journey (-selftest logic) + snapshot comparison with tests/snapshots
#   scripts/test-gui.sh --update     re-render and replace the baselines (after a deliberate look change; review the PNGs, commit them)
#   scripts/test-gui.sh --render DIR just render the screens into DIR (no comparison)
#
# Needs: gcc, zlib headers, Go with cgo, fonts-dejavu-core and fonts-noto-cjk (Tahoma is not on Linux; the baselines use these).
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=build/linux-test; mkdir -p "$OUT/obj"
echo "== build the Linux test host"
( cd core && CGO_ENABLED=1 go build -buildmode=c-archive -trimpath -o ../$OUT/libafvi-core.a ./cmd/afvi-core-dll )
python3 gui/gen_lang.py gui/lang gui/lang_data.c
for f in app skin i18n util lang_data third_party/cJSON; do gcc -c -O2 -Wall -Wno-unused-function -Igui gui/$f.c -o $OUT/obj/$(basename $f).o; done
gcc -O2 -Igui gui/preview/preview_main.c $OUT/obj/*.o $OUT/libafvi-core.a -lz -lpthread -lm -ldl -o $OUT/host

# a fixed path: it is shown in the status bar of the snapshots
W=/tmp/afvi-gui-test
[ -n "$W" ] && [ -d "$W" ] && rm -rf -- "$W"
mkdir -p "$W"
case "${1:-}" in
  --render) mkdir -p "${2:?usage: --render DIR}"; $OUT/host "$2" "$W/work" 1 14 | tail -1 ;;
  --update) $OUT/host --update tests/snapshots "$W/work" ;;
  "")
    echo "== scripted UI journey"
    $OUT/host --selftest "$W/selftest"
    echo "== snapshots"
    $OUT/host --check tests/snapshots "$W/work" || { mkdir -p shots/failed; cp "$W"/work/render/*.png shots/failed/ 2>/dev/null || true; echo "rendered images and *.diff.png copied to shots/failed/"; exit 1; } ;;
  *) echo "unknown option $1" >&2; exit 2 ;;
esac
echo "== OK"
