# What the machines look like (observed 2026-10-24, read-only)

Machines: `192.168.1.61` (FM1), `.62` (FM2), `.63` (BM) of **AFVI 13**. Shares used: `PxRepository`
and `PxInventory` (the others, `PxData`, `PxInspector`, `PxTemporary`, `ResultData`, are not touched).
Login: user `pixel`, empty password. Windows file names are case insensitive, so every comparison
in the tool is case insensitive.

## A model is four kinds of entries

For a model called `N`:

| Part | Location | Kind |
| --- | --- | --- |
| Teach data | `PxRepository\N` | directory: `<resolution 2.0 or 3.5>\Line\{Top,Bottom}\...` (images, `*_handler.txt`, `*_Inspector.txt`), hundreds of MB to a few GB |
| History | `PxRepository\N.his` | CSV file, optional. **The first column of every row is the model name.** CRLF line ends |
| Inspect spec | `PxInventory\INSPECT_SPEC\N-NN` | directories, `NN` is two digits, normally `-00` (one node has `-01` too) |
| Light spec | `PxInventory\LIGHT_SPEC\N` | directory |

The same model usually exists with different case in the four places: `24fbo060-19` (repository),
`24FBO060-19.his`, `24FBO060-19-00` (inspect spec), `24FBO060-19` (light spec). Entries that do not
follow the pattern are **not** part of the bundle and are never touched, for example
`INSPECT_SPEC\24FBO060-19-00 스캔원본` (a backup of the spec), `INSPECT_SPEC\23csp` or `LIGHT_SPEC\25FBO`.

Nothing else mentions the model name: no file inside the repository directory does, and the root
files of `PxInventory` (`Application.xml`, `SystemList.xml`, ...) have no per-model entries.
`Application.xml` has `<system ID="AFVI13" Model="7M99-535" ...>`, the model the machine runs now.
That file is not touched.

## Name shapes (a hint column, not a rule)

| Class | Pattern | Example | FM1 / FM2 / BM |
| --- | --- | --- | --- |
| A | `NNaaaNNN-NN` | `21csp043-09` | 36 / 14 / 30 |
| B | `NaaNNNNaNN` | `2AN0859F01` | 8 / 8 / 8 |
| C | `NaaNNNNa` | `6sg0504a` | 3 / 3 / 3 |
| M99 | `[N]M99-NNN[-N]` | `7M99-535` | 12 / 12 / 12 |
| COPY | contains `bk`, `test`, `copy`, `복사본`, `원본` | `7M99-631bk`, `6AN0921I01 - 복사본 (2)` | 17 / 16 / 16 |
| OTHER | everything else | `danny`, `Sample`, `SPFCE300N4.KMA0` | 15 / 11 / 12 |

Names may contain spaces, dots, Hangul and parentheses. The tool only requires what Windows requires
(no `<>:"/\|?*`, no trailing dot or space, no reserved device names, at most 100 characters) and that
the name does not end in `.his`.

Models seen per node: FM1 91, FM2 64, BM 81 (complete = repository + inspect + light: 54 / 29 / 43).
Many models exist only partly (22 / 21 / 24 have no repository directory, some have only the spec
entries); the list shows which parts each one has and Copy mirrors exactly what the source has.
