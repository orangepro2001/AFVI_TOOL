#!/usr/bin/env bash
# Run the GUI from source against the dev config. Extra arguments go to the app:
#   scripts/run-dev.sh --fontsize=16
#   scripts/run-dev.sh --config=/path/to/config.json
# The real machines are read-only in this phase; use scripts/dev-local.sh to add
# a writable LOCAL TEST machine for trying copies.
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v dotnet >/dev/null 2>&1 && [ -x "$HOME/.dotnet/dotnet" ]; then
  export PATH="$HOME/.dotnet:$PATH" DOTNET_ROOT="$HOME/.dotnet"
fi
export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
mkdir -p bin
(cd core && go build -o ../bin/afvi-core ./cmd/afvi-core)
export AFVI_CORE="$PWD/bin/afvi-core"
exec dotnet run --project app/src/AfviTools -- "$@"
