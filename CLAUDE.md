# CLAUDE.md

AFVI TOOLS: Model Copy + Export Data To Excel helper for AFVI machines. **Windows only**: a plain Win32 GUI in C (everything drawn in the Windows 2000 style) with a Go
core behind a C ABI, cross-built from Linux with mingw-w64. User-facing docs are Chinese.

## Hard rules

- **Dev phase: the real machines (192.168.1.61/62/63, shares PxRepository/PxInventory, user `pixel`, empty password) are read-only.** Never write, rename or delete there.
  Test copies on `LOCAL TEST` (`scripts/dev-local.sh`) or temp dirs. The vfs layer enforces it (`vfs.Open` returns read-only file systems for network paths unless
  `allowNetworkWrite`); do not bypass it, do not pass `-allow-network-write`.
- No commit/push unless asked. Remote `git@github.com:orangepro2001/AFVI_TOOL.git` is configured, nothing pushed. Commits need an identity: use `git -c user.name=tony -c user.email=yangziyi0801@gmail.com` (none is configured).
- `.local-mirror/` holds copies of customer data: git-ignored, never commit it.
- The look is the Windows 2000 style of the `win2000-style` skill (tokens in `gui/skin.c`): no rounded corners (radio buttons excepted), shadows or hover effects. Native OS controls cannot give this look
  on Windows 8+; everything is drawn. Verify with a rendered image, not by reading code.
- Nothing here has been run under Wine; the user ran the Windows exe and reported it works. Do not claim Windows behaviour you have not seen.

## Commands

```bash
scripts/build.sh [--no-test|--go-only]   # gofmt + vet + go test, then the Windows app -> build/native-win/AfviTools.exe (static) and dynamic/
scripts/win32.sh                         # app only: both link modes, sizes and imports, plus the Linux PNG preview in shots/
cd core && go test ./...                 # Go only (pure Go; cgo only in cmd/afvi-core-dll)
command gofmt -l core                    # `gofmt` may be aliased to a rewriting tool in zsh
```

## Where things are

README.md (layout), docs/ARCHITECTURE.md (C ABI, process model), docs/NAMING.md (what a model is on the machines), docs/LINUX_TESTING.md, docs/COMPONENT_LIBRARY.md (plan to extract the controls),
docs/STACK_COMPARISON.md and docs/validation/ (the five front ends tried; Avalonia is tag `v0.1.0-avalonia`).
Copy/rename rules: `core/internal/policy` only. Languages: default Korean; core packs `core/internal/i18n/catalog.go`, GUI packs `gui/lang/*.json` (flat key -> text, `{0}` placeholders, same keys in all four; embedded by `gui/gen_lang.py`).
`core/internal/wailsapp` is the UI-independent facade (name is historical) used by `capi`.

## Gotchas learned

- In zsh an unquoted `$VAR` holding several options is not word-split; put multi-option commands in a bash script. `pkill -f <pattern>` can kill the calling shell.
- Go c-archive/c-shared: a panic must never cross the C boundary (`capi.Call` recovers); memory from `afvi_call` is freed with `afvi_free`.
- Tahoma is not on Linux: previews use DejaVu Sans / Noto Sans CJK and say so.
