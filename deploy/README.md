# Delivery (placeholder)

Nothing is delivered automatically yet. This folder and the disabled `deploy` job in
`.github/workflows/ci.yml` hold the place for it.

## What CI produces on every push to `main`

`package-windows` uploads `AFVI-TOOLS-win-x64.zip` (the `build/win` folder): a single `AfviTools.exe` of about 25 MB with the .NET runtime,
Avalonia + Skia and the Go core inside (compressed and trimmed).
On first start it unpacks its native libraries (.NET cache folder) and the Go core (`%LOCALAPPDATA%\AFVI_TOOLS\core`), invisibly. `AFVI_CORE` or a core next to the exe overrides the embedded one.

## Decisions to make before turning delivery on

1. **Where the package goes**: a GitHub Release asset (rolling `nightly` like AFVI_TEACH), a
   file share on the plant network, or the Windows desktop through the build server.
2. **Installer or zip**: zip needs no install and no admin rights; an installer (Inno Setup / WiX)
   adds shortcuts and uninstall.
3. **The read-only lock**: the shipped build keeps `allowNetworkWrite: false`. The release that
   is allowed to write to the machines must flip it deliberately (config `settings.allowNetworkWrite`
   or the `-allow-network-write` flag) and be reviewed for that.
4. **Windows smoke test**: a `windows-latest` job that starts `afvi-core.exe`, calls `/v1/health`
   and renders one window to a PNG, before anything is delivered.

The AFVI_TEACH repo already has a working push-main -> build server -> Windows desktop chain
(`deploy/server/`, `docs/SERVER_CI.md` there); reuse it instead of inventing a second one.
