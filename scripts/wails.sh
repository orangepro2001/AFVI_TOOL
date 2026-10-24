#!/usr/bin/env bash
# Build and test the Wails (system webview) version of the GUI. Branch wails-rpc only.
#
#   scripts/wails.sh              build Linux + Windows (cross-built from Linux, no cgo), run the UI self-test on Linux
#   scripts/wails.sh --no-test    skip the self-test
#
# Output: core/cmd/afvi-wails/build/bin/afvi-tools (Linux, needs libwebkit2gtk-4.1) and afvi-tools.exe (Windows,
# needs the WebView2 runtime on the machine). macOS needs a Mac host and is not built here.
set -euo pipefail
cd "$(dirname "$0")/../core"
export PATH="$HOME/go/bin:$PATH"
command -v wails >/dev/null || { echo "wails CLI not found (go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0)"; exit 1; }
test -z "$(command gofmt -l .)" || { echo "gofmt needed"; command gofmt -l .; exit 1; }
go vet ./... && go test ./...
cd cmd/afvi-wails
wails build -platform linux/amd64 -tags webkit2_41 -skipbindings -trimpath -o afvi-tools 2>&1 | grep -E "Built|rror"
wails build -platform windows/amd64 -skipbindings -trimpath -o afvi-tools.exe 2>&1 | grep -E "Built|rror"
ls -la build/bin
if [ "${1:-}" != "--no-test" ] && command -v xvfb-run >/dev/null; then
  H="$(mktemp -d)"
  xvfb-run -a -s "-screen 0 1280x800x24" env HOME="$H" GDK_BACKEND=x11 XDG_CONFIG_HOME="$H/cfg" XDG_DATA_HOME="$H/data" XDG_CACHE_HOME="$H/cache" \
      timeout 120 build/bin/afvi-tools -selftest="$H/st" 2>&1 | grep SELFTEST | tee "$H/out"
  grep -q "SELFTEST OK" "$H/out"
fi
