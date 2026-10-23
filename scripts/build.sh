#!/usr/bin/env bash
# One-click build and check for AFVI TOOLS.
#
#   scripts/build.sh                      gofmt + vet + Go tests, build bin/afvi-core, build and test the GUI
#   scripts/build.sh --no-test            skip the tests
#   scripts/build.sh --platform win       also build the native app into ./build/win
#   scripts/build.sh --platform linux     ... ./build/linux
#   scripts/build.sh --platform mac       ... ./build/mac   (Apple Silicon; MAC_ARCH=x64 for Intel)
#   scripts/build.sh --platform all       all three (comma list works too: win,linux)
#
# Each ./build/<platform> folder holds ONE file, AfviTools[.exe]: the GUI with the .NET runtime, the native
# libraries and the Go core (afvi-core) inside; nothing to install, nothing else to find. Everything is cross-built from this Linux machine: Go
# cross-compiles with CGO_ENABLED=0 and `dotnet publish -r <rid>` needs no native toolchain
# (Native AOT would, so it is not used). Not run-tested on Windows/macOS here.
#
# dotnet is looked up on PATH, then in ~/.dotnet (user-level install).
set -euo pipefail
cd "$(dirname "$0")/.."

run_tests=1
platforms=""
while [ $# -gt 0 ]; do
  case "$1" in
    --no-test) run_tests=0 ;;
    --windows) platforms="$platforms win" ;;
    --platform) shift; platforms="$platforms $(printf '%s' "${1:-}" | tr ',' ' ')" ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done
case " $platforms " in *" all "*) platforms="win linux mac" ;; esac
for p in $platforms; do
  case "$p" in win|linux|mac) ;; *) echo "unknown platform: $p (win, linux, mac, all)" >&2; exit 2 ;; esac
done

if ! command -v dotnet >/dev/null 2>&1 && [ -x "$HOME/.dotnet/dotnet" ]; then
  export PATH="$HOME/.dotnet:$PATH" DOTNET_ROOT="$HOME/.dotnet"
fi
export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1

version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
# assembly version must be numeric; git describe output like v0.2.0-5-gabc123 or "dev" is mapped
numver="$(printf '%s' "$version" | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/p')"
numver="${numver:-0.1.0}"
echo "== version $version (assembly $numver)"

echo "== gofmt"
# plain `gofmt` may be aliased to a rewriting formatter in interactive shells
bad="$(command gofmt -l core)"
if [ -n "$bad" ]; then echo "gofmt needed:"; echo "$bad"; exit 1; fi

echo "== go vet"
(cd core && go vet ./...)

if [ "$run_tests" = 1 ]; then
  echo "== go test"
  (cd core && go test ./...)
fi

echo "== build bin/afvi-core"
mkdir -p bin
(cd core && go build -trimpath -ldflags "-s -w -X main.version=$version" -o ../bin/afvi-core ./cmd/afvi-core)

echo "== build GUI"
dotnet build app/src/AfviTools -c Release -nologo -v q /p:Version="$numver"

if [ "$run_tests" = 1 ]; then
  echo "== GUI tests"
  # the tests start bin/afvi-core; they never touch the network
  AFVI_CORE="$PWD/bin/afvi-core" dotnet test app/tests/AfviTools.Tests -nologo -v q
fi

package() { # platform dotnet-rid goos goarch core-exe-name
  local plat="$1" rid="$2" goos="$3" goarch="$4" exe="$5" out="build/$1"
  echo "== package $plat ($rid) -> $out"
  rm -rf "$out"
  mkdir -p "$out" "bin/core/$plat"
  # 1. the Go core for this platform ...
  (cd core && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=$version" -o "../bin/core/$plat/$exe" ./cmd/afvi-core)
  # 2. ... embedded in the GUI, which is published as ONE file: managed code, .NET runtime and native libraries
  # (Skia, HarfBuzz ...) compressed inside; the unused parts of Avalonia and the framework are trimmed away.
  # On first start the program unpacks its native libraries and the core to the user's data folders, invisibly.
  dotnet publish app/src/AfviTools -c Release -r "$rid" --self-contained true -nologo -v q -o "$out" \
      /p:Version="$numver" /p:EmbeddedCore="$PWD/bin/core/$plat/$exe" \
      /p:PublishSingleFile=true /p:IncludeNativeLibrariesForSelfExtract=true /p:EnableCompressionInSingleFile=true \
      /p:PublishTrimmed=true /p:TrimMode=partial /p:TrimmerSingleWarn=true /p:DebugType=none /p:DebugSymbols=false
  rm -f "$out"/*.pdb
  echo "   $out: $(ls "$out" | tr '\n' ' ')($(du -sh "$out" | cut -f1))"
}

for p in $platforms; do
  case "$p" in
    win)   package win   win-x64 windows amd64 afvi-core.exe ;;
    linux) package linux linux-x64 linux amd64 afvi-core ;;
    mac)   arch="${MAC_ARCH:-arm64}"; package mac "osx-$arch" darwin "$arch" afvi-core ;;
  esac
done

if [ "$run_tests" = 1 ] && [ -x build/linux/AfviTools ] && command -v xvfb-run >/dev/null 2>&1; then
  echo "== selftest of the packaged Linux app (every window, every language, trimmed single file)"
  st="$(mktemp -d)"
  XDG_DATA_HOME="$st/data" xvfb-run -a -s "-screen 0 1280x800x24" \
      build/linux/AfviTools --config="$st/config.json" --selftest="$PWD/shots/selftest"
  rm -rf "$st"
fi

echo "== OK"
