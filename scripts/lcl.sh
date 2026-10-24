#!/usr/bin/env bash
# Build and test the Lazarus LCL (Go + golcl + prebuilt liblcl) version of the GUI. Branch lazarus-lcl only.
#
#   scripts/lcl.sh              vet + Go tests, build Linux and Windows (cross-built, no cgo), run the UI self-test
#                               in a real GTK3 window for the three skins (native, win2000, platinum)
#   scripts/lcl.sh --no-test    only build
#
# Output: build/lcl/linux/{afvi-lcl,liblcl.so}  build/lcl/win/{afvi-lcl.exe,afvi-lcl-winstyles.exe,liblcl.dll}
# liblcl comes from https://github.com/energye/liblcl (v2.5.4, GTK3 on Linux). macOS cannot be built from Linux (cgo).
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
LIBVER=v2.5.4
out="$root/build/lcl"
mkdir -p "$out/linux" "$out/win" "$root/build/lcl/dl"

fetch() { # zip-name target
  [ -f "$2" ] && return
  curl -fsSL -o "$out/dl/$1" "https://github.com/energye/liblcl/releases/download/$LIBVER/$1"
  (cd "$out/dl" && unzip -oq "$1" && cp "$(basename "$2")" "$2")
}
fetch liblcl-GTK3.Linux64.zip "$out/linux/liblcl.so"
fetch liblcl.Windows64.zip "$out/win/liblcl.dll"

cd core
test -z "$(command gofmt -l .)" || { echo "gofmt needed"; command gofmt -l .; exit 1; }
go vet ./... && go test ./...
go build -trimpath -ldflags "-s -w" -o "$out/linux/afvi-lcl" ./cmd/afvi-lcl
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o "$out/win/afvi-lcl.exe" ./cmd/afvi-lcl
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags winstyles -trimpath -ldflags "-s -w -H windowsgui" -o "$out/win/afvi-lcl-winstyles.exe" ./cmd/afvi-lcl
ls -la "$out/linux" "$out/win"

if [ "${1:-}" != "--no-test" ] && command -v xvfb-run >/dev/null; then
  for skin in native win2000 platinum; do
    H="$(mktemp -d)"
    echo "== self-test, skin $skin"
    xvfb-run -a -s "-screen 0 1280x800x24" env HOME="$H" GDK_BACKEND=x11 XDG_DATA_HOME="$H/d" XDG_CONFIG_HOME="$H/c" \
        timeout 120 "$out/linux/afvi-lcl" -skin="$skin" -selftest="$H/st" 2>&1 | grep -E "SELFTEST" | tee "$H/out"
    grep -q "SELFTEST OK" "$H/out"
  done
fi
