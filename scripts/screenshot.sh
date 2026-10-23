#!/usr/bin/env bash
# Renders the running app to a PNG under an X virtual framebuffer (works on a server):
#   scripts/screenshot.sh shots/main.png [app args...]
# Fonts: Tahoma does not exist on Linux, the fallback (Noto Sans CJK / DejaVu Sans) is used.
set -euo pipefail
cd "$(dirname "$0")/.."
out="${1:?usage: screenshot.sh out.png [app args]}"; shift || true
if ! command -v dotnet >/dev/null 2>&1 && [ -x "$HOME/.dotnet/dotnet" ]; then
  export PATH="$HOME/.dotnet:$PATH" DOTNET_ROOT="$HOME/.dotnet"
fi
export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
mkdir -p bin "$(dirname "$out")"
(cd core && go build -o ../bin/afvi-core ./cmd/afvi-core)
dotnet build app/src/AfviTools -nologo -v q
export AFVI_CORE="$PWD/bin/afvi-core"
xvfb-run -a -s "-screen 0 1280x800x24" dotnet app/src/AfviTools/bin/Debug/net10.0/AfviTools.dll --screenshot="$out" "$@"
echo "wrote $out"
