# Stack comparison (validated 2026-10-24)

Five front ends were built or tried for the same tool; the Windows native one was chosen. Full reports: `docs/validation/`. Code of the others: tag `v0.1.0-avalonia`
(Avalonia + Go sidecar) and the branches `wails-rpc`, `avalonia-aot`, `lazarus-lcl`.

| Stack | Windows size | Idle RSS (Linux) | Start | One machine builds all | Verdict |
| --- | --- | --- | --- | --- | --- |
| **C Win32 + Go core (this)** | **5.2 MB, 1 file** (static) or 0.2 + 5.1 MB | not measured | not measured | Windows only, cross-built from Linux | chosen; hand-drawn controls; verified on Windows by the user (works, fast) |
| Wails v2 + in-process RPC | 12 MB | about 460 MB | about 0.25 s | Windows yes; Linux/mac need their hosts | good; needs WebView2; CSS gives the look |
| Lazarus (golcl) | 10.4 + 8.2 MB (2 files) | 155-163 MB | 0.11-0.15 s | mac cannot be built on Linux | possible; skins need custom drawing for most controls |
| Avalonia trimmed single file + Go sidecar | 24 MB | about 208 MB | about 1.2 s | yes (win/linux/mac) | complete and tested; heaviest to ship; `v0.1.0-avalonia` |
| Avalonia Native AOT | 33 + 16 MB | 182 MB | n/a | Windows only via an unsupported Zig link experiment | not smaller; DataGrid sort broke |

Lessons that outlive the choice: native OS controls cannot give the Windows 2000 look on Windows 8+; measure size and memory on day one; keep the core a pure Go package behind a
small API so the front end can be swapped (it was swapped four times); tests must never touch the real machines.
