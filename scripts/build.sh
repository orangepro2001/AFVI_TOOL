#!/usr/bin/env bash
# One-click check and build of AFVI TOOLS (Windows only, cross-built from Linux).
#
#   scripts/build.sh             gofmt + vet + Go tests, then the Windows app into ./build/native-win
#   scripts/build.sh --no-test   skip the tests (Go and the Linux GUI tests)
#   scripts/build.sh --go-only   only the Go checks (no mingw needed)
#
# Needs: Go, and for the app x86_64-w64-mingw32-gcc (apt: gcc-mingw-w64-x86-64), python3, bc.
set -euo pipefail
cd "$(dirname "$0")/.."
run_tests=1; go_only=0
for a in "$@"; do
  case "$a" in
    --no-test) run_tests=0 ;;
    --go-only) go_only=1 ;;
    *) echo "unknown option: $a" >&2; exit 2 ;;
  esac
done

echo "== gofmt"
bad="$(command gofmt -l core)"   # plain `gofmt` may be aliased to a rewriting formatter in zsh
if [ -n "$bad" ]; then echo "gofmt needed:"; echo "$bad"; exit 1; fi
echo "== go vet"
(cd core && go vet ./...)
if [ "$run_tests" = 1 ]; then echo "== go test"; (cd core && go test ./...); fi
if [ "$go_only" = 1 ]; then echo "== OK (Go only)"; exit 0; fi

if [ "$run_tests" = 1 ]; then echo "== GUI tests on Linux (scripted journey + snapshots)"; scripts/test-gui.sh; fi

echo "== Windows app"
scripts/win32.sh
echo "== OK: build/native-win/AfviTools.exe"
