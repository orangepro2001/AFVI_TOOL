#!/usr/bin/env bash
# Cross-build the Win32 front end from Linux with mingw-w64: the Go core as a DLL (dynamic link) and as a
# static archive (static link), the C GUI, and the preview renderer. Output: build/win32/
#   scripts/win32.sh          build everything and print sizes and imported DLLs
set -euo pipefail
cd "$(dirname "$0")/.."
CC=x86_64-w64-mingw32-gcc
OUT=build/win32; OBJ=$OUT/obj; mkdir -p "$OBJ"
objdump() { x86_64-w64-mingw32-objdump "$@"; }

echo "== Go core (cgo, mingw)"
( cd core
  go vet ./internal/capi/ && go test ./internal/capi/ 2>&1 | tail -1            # logic tests run natively, before the cross environment
  export CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$CC
  go build -buildmode=c-shared  -trimpath -ldflags "-s -w" -o ../$OUT/afvi-core.dll ./cmd/afvi-core-dll
  go build -buildmode=c-archive -trimpath -ldflags "-s -w" -o ../$OUT/libafvi-core.a ./cmd/afvi-core-dll )

echo "== GUI objects"
python3 gui/gen_lang.py gui/lang gui/lang_data.c
CF="-O2 -Wall -Wno-unused-function -municode -Igui -Ilibw2k"
for f in win_main app i18n util lang_data third_party/cJSON; do $CC -c $CF gui/$f.c -o $OBJ/$(basename $f).o; done
for f in w2k_draw w2k_ctl w2k_gdi; do $CC -c $CF libw2k/$f.c -o $OBJ/$f.o; done
x86_64-w64-mingw32-windres gui/app.rc -o $OBJ/app_rc.o
OBJS="$OBJ/win_main.o $OBJ/app.o $OBJ/w2k_draw.o $OBJ/w2k_ctl.o $OBJ/w2k_gdi.o $OBJ/i18n.o $OBJ/util.o $OBJ/lang_data.o $OBJ/cJSON.o $OBJ/app_rc.o"
SYS="-lgdi32 -luser32 -lkernel32 -lshell32 -lole32"

echo "== link: dynamic (afvi-core.dll beside the exe)"
$CC -s -mwindows -municode $OBJS $OUT/afvi-core.dll $SYS -o $OUT/afvi-dyn.exe                          # default mingw runtime (msvcrt.dll, libgcc static)
$CC -s -mwindows -municode -static $OBJS $OUT/afvi-core.dll $SYS -o $OUT/afvi-dyn-static.exe          # no mingw runtime DLLs at all
echo "== link: static (core archive linked into one exe)"
$CC -s -mwindows -municode $OBJS $OUT/libafvi-core.a $SYS -lws2_32 -lwinmm -lntdll -lbcrypt -luserenv -o $OUT/afvi-static.exe
$CC -s -mwindows -municode -static $OBJS $OUT/libafvi-core.a $SYS -lws2_32 -lwinmm -lntdll -lbcrypt -luserenv -o $OUT/afvi-static-full.exe

echo "== Linux preview renders (shots/*.png, same app code, real core; see docs/LINUX_TESTING.md)"
scripts/test-gui.sh --render shots

echo "== deliverable: build/native-win (static one-file exe recommended; dynamic pair for reference)"
D=build/native-win; mkdir -p $D/dynamic
cp $OUT/afvi-static.exe $D/AfviTools.exe
cp $OUT/afvi-dyn.exe $D/dynamic/AfviTools.exe; cp $OUT/afvi-core.dll $D/dynamic/afvi-core.dll

echo "== sizes and imports"
for f in afvi-core.dll libafvi-core.a afvi-dyn.exe afvi-dyn-static.exe afvi-static.exe afvi-static-full.exe; do
  printf '%-24s %6.1f MB   imports: ' "$f" "$(echo "$(stat -c %s $OUT/$f) / 1048576" | bc -l)"
  case $f in *.a) echo "-";; *) objdump -p $OUT/$f | grep "DLL Name" | awk '{print tolower($3)}' | sort -u | tr '\n' ' '; echo;; esac
done
