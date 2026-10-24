# Delivery (placeholder)

Nothing is delivered automatically yet. This folder and the disabled `deploy` and `smoke-windows` jobs in `.github/workflows/ci.yml` hold the place for it.

## What CI produces on every push to `main`

`package-windows` cross-builds with mingw-w64 and uploads `AfviTools.exe` (static link, about 5.5 MB, single file, imports only msvcrt/kernel32/user32/gdi32/shell32).
The Linux preview PNGs are uploaded too.

## Decisions to make before turning delivery on

1. **Where the package goes**: a GitHub Release asset, a file share on the plant network, or the build server's desktop copy.
2. **Smoke test on Windows**: enable `smoke-windows` (start the exe, run `-selftest`) once `-selftest` has been verified on a real machine.
3. **The read-only lock**: the shipped build keeps `allowNetworkWrite: false`. The release that may write to the machines must flip it deliberately and be reviewed for that.
4. **Signing / antivirus**: a Go+cgo exe can trigger false positives; code signing is the usual remedy.
