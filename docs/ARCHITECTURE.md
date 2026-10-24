# Architecture

```
AfviTools.exe  (C, plain Win32 API, GDI, everything drawn in the Windows 2000 style)
   |  C ABI, in the same process:   afvi_init(config)   afvi_call(method, lang, json) -> json   afvi_free(ptr)
   v
Go core (service layer: rules, plans, jobs, vfs)  --->  \\192.168.1.6x\PxRepository / PxInventory   (read-only in the development phase)
```

The Go core is linked into the exe statically (`go build -buildmode=c-archive`, 5.2 MB single file) or kept in `afvi-core.dll` (`c-shared`, exe 0.2 MB). Both import only
old system DLLs (`msvcrt`, `kernel32`, `user32`, `gdi32`, `shell32`). A Go panic never crosses the C boundary (`core/internal/capi.Call` recovers and returns a JSON error);
memory returned by `afvi_call` is C-malloc'ed and freed with `afvi_free`. The GUI draws only through `libw2k/w2k_gfx.h` (five operations), which has a GDI backend and a PNG
backend, so the same code is rendered on Linux for review and tests (`docs/LINUX_TESTING.md`). The drawn controls live in `libw2k/` (`libw2k/README.md`, `docs/COMPONENT_LIBRARY.md`).

Other front ends were validated and are kept as history (`docs/STACK_COMPARISON.md`). `core/cmd/afvi-core` plus `core/internal/server` is an optional loopback HTTP transport
of the same service layer (used by the earlier Avalonia version); it is not part of the Windows deliverable.

## API (`core/internal/server`)

All JSON, camelCase. The GUI language goes in `X-Afvi-Lang` (`ko` default, `zh-Hans`, `zh-Hant`, `en`); every
user-visible sentence the core returns is in that language.

| Method + path | Purpose |
| --- | --- |
| `GET /v1/health` | version, os, whether network writes are enabled |
| `GET /v1/config` | machines and settings |
| `POST /v1/machines`, `PUT/DELETE /v1/machines/{name}`, `POST /v1/machines/restore-defaults` | machine configuration |
| `GET /v1/machines/{name}/nodes/{FM1\|FM2\|BM}/models` | scan: one row per model with which parts exist |
| `GET /v1/policy/targets?machine=&node=` | the single-node targets and group targets the rules allow |
| `POST /v1/names/check` | validate a new model name |
| `POST /v1/plans/copy`, `POST /v1/plans/rename` | build a plan (reads the sources, changes nothing): steps, warnings, blockers |
| `POST /v1/jobs` `{planId}` | run a plan once; `GET /v1/jobs/{id}` progress; `POST /v1/jobs/{id}/cancel` (only before the first deletion) |
| `POST /v1/export/excel` | reserved entry, answers 501 |

## Copy and rename semantics (`core/internal/ops`)

Plan -> confirm -> job. A plan is single use and expires after 30 minutes; one job runs at a time.

Copy, per target node (a group copy runs the nodes one after the other):

1. re-walk every source part and compare with the plan (a changed source aborts, nothing touched yet)
2. point of no return: delete every existing target part of the model (case-insensitive; each delete is checked with `Part.Belongs`)
3. copy each part, keeping modification times; the `.his` is rewritten on the way when the name changes (first CSV column)
4. verify the copied tree (relative paths and sizes); on any failure remove the incomplete new parts

Rename: directories first (`Rename`), then the `.his` (write a temp file with the new first column, remove the old
file, rename the temp file). Every completed step is journaled; a failure rolls back all of them, across all
nodes of a group rename.

## Rules for new code

- Touch the machines only through `vfs.FS`. Never call `os.*` on a share path.
- Every sentence a user can read is a key in `core/internal/i18n/catalog.go` (core) or `Assets/lang/*.json` (GUI), in all four languages;
  `TestCatalogComplete` and `LocalizationTests` fail on a missing key or a different placeholder list. Technical errors with OS text stay English.
- Tests use temp directories (`internal/testkit`, `Harness`), never the real machines.
