# Validation: Native AOT for the Avalonia GUI (branch `avalonia-aot`)

Date 2026-10-24, Linux dev machine, .NET SDK 10.0.402, ILCompiler 10.0.13. Baseline = trimmed + compressed single file with the Go core embedded
(`main`, `scripts/build.sh`): win 24 MB, linux 25 MB, idle RSS about 208 MB.

## Results

| | Native AOT | Baseline (trimmed single file) |
| --- | --- | --- |
| Linux publish | works (52 s). `AfviTools` 31 MB (core embedded) **plus** libSkiaSharp.so 9.2 MB and libHarfBuzzSharp.so 2.5 MB beside it: about 43 MB | 25 MB, one file |
| Linux idle RSS | 182 MB | 208 MB |
| Linux UI self-test | 19 of 20 windows/steps pass; **1 failure: DataGrid sort** (sorting a column uses reflection that AOT removed) | passes |
| Windows, cross-built from Linux | ILC compiles for win-x64 on Linux; **linking works with Zig** (see below): `AfviTools.exe` 33 MB (PE32+ GUI) plus about 16 MB of native DLLs (Skia, HarfBuzz, ANGLE) | 24 MB, one file |
| Windows run test | **not run** (no Windows, no Wine) | not run either |

AOT is not smaller: it cannot bundle the native libraries into one file and the AOT code for Avalonia + the runtime pieces is bigger than the trimmed IL.
Its only gains are lower memory (about 13 %) and faster start (no JIT; not measured separately). It also removes the need for the .NET runtime
on disk, which the self-contained build already did.

## Windows from Linux with Zig

`PublishAotCross` (NuGet) only supports a Windows host targeting Linux, not the reverse. What worked instead:

1. `-p:PublishAot=true -p:DisableUnsupportedError=true -r win-x64`: the SDK on Linux runs ILC and emits a COFF object, then calls `link @link.rsp`.
2. A `link` shim (`scripts/aot-win/link`) first in `PATH` maps the MSVC options to `zig cc -target x86_64-windows-gnu -nostartfiles` (Zig 0.17.0, installed
   user-level in `~/.local/share/zig`), which supplies import libraries for kernel32, ntdll, advapi32 ... from its bundled mingw-w64 data: no Windows SDK needed.
3. The NativeAOT runtime `.lib` files are MSVC-built and expect pieces of the MSVC CRT. Missing symbols were closed by `scripts/aot-win/msvcabi.c`: MSVC-mangled
   `operator new/delete`, `std::nothrow`, `__security_cookie`, `__security_check_cookie`, `__GSHandlerCheck`, `__guard_dispatch_icall_dummy`, `__tls_guard`,
   `_purecall`, `_fltused`, a `wmainCRTStartup` entry that calls `wmain`, and a dummy `WinMain`; empty archives for the default-lib names `LIBCMT OLDNAMES libcpmt uuid`.
4. Result: a PE32+ GUI executable importing KERNEL32, ntdll, ADVAPI32, bcrypt, CRYPT32, ole32 ... and the `api-ms-win-crt-*` UCRT API sets, with a TLS directory.

Risks that only a Windows run can settle: start-up through the custom entry, structured exceptions with the stub `__GSHandlerCheck` (the runtime uses SEH
for some GC and exception paths), thread-local storage setup, no CFG and an unrandomised stack cookie, no `/MERGE` of sections, no PDB symbols.
This is an experiment, not a build you can ship; Microsoft does not support cross-OS Native AOT.

## What had to change in the app

The localisation binding was a reflection binding; it was replaced by an AOT-safe one (`Localization/TExtension.cs`). Remaining reflection: DataGrid column
bindings and sorting (the failing self-test step). The Classic theme and CommonControls assemblies produced no blocking warnings in this run.

## Verdict: no-go as the default packaging; keep as an option

Same functionality, larger on disk, no single file, one failing feature and an unsupported Windows cross-link. Prefer the trimmed single file (`main`).
Revisit only if start-up time or memory become requirements, and then build on a Windows runner (`windows-latest`) with the real MSVC linker.
Effort to make it production grade: about 2 days (compiled bindings for the DataGrid, replace sorting, a Windows CI job and a real start-up test).
