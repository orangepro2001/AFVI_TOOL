#!/usr/bin/env bash
# Adds a writable machine "LOCAL TEST" (FM1/FM2/BM below .local-mirror/LOCAL) to the
# user configuration. The real machines (AFVI 13) stay read-only; to get a real model
# into the sandbox, use the GUI: source AFVI 13 / FM1, pick a model, "整组复制" to
# LOCAL TEST. That only reads from the network.
#   scripts/dev-local.sh [config-file]
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin
(cd core && go build -o ../bin/afvi-core ./cmd/afvi-core)
args=(dev-machine -name "LOCAL TEST" -dir .local-mirror/LOCAL)
if [ $# -ge 1 ]; then args+=(-config "$1"); fi
exec bin/afvi-core "${args[@]}"
