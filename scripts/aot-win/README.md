# Native AOT for Windows, cross-linked from Linux with Zig (experiment)

`dotnet publish -r win-x64 -p:PublishAot=true -p:DisableUnsupportedError=true` on Linux gets through the whole compile (ILC runs on Linux and
emits a win-x64 COFF object); only the MSVC `link.exe` step is missing. `link` here is a drop-in `link @file.rsp` that maps the MSVC options onto
`zig cc -target x86_64-windows-gnu` (lld-link underneath, mingw-w64 import libraries, no Windows SDK needed). The NativeAOT runtime libraries are
MSVC-built, so the pieces of the MSVC CRT they expect (operator new/delete with MSVC names, `__security_cookie`, `__GSHandlerCheck`, the
`wmainCRTStartup` entry ...) come from `msvcabi.c`; empty `libLIBCMT.a libOLDNAMES.a liblibcpmt.a libuuid.a` satisfy their /DEFAULTLIB directives.

    zig=~/.local/share/zig/zig-x86_64-linux-0.17.0
    mkdir -p /tmp/aotx/stubs && $zig/zig cc -target x86_64-windows-gnu -c msvcabi.c -o /tmp/aotx/stubs/msvcabi.o
    for l in LIBCMT OLDNAMES libcpmt uuid; do (cd /tmp/aotx/stubs && $zig/zig ar rc lib$l.a); done
    install -m755 link /tmp/aotx/bin/link
    PATH=/tmp/aotx/bin:$PATH dotnet publish app/src/AfviTools -c Release -r win-x64 --self-contained \
        -p:PublishAot=true -p:DisableUnsupportedError=true -p:EmbeddedCore=<afvi-core.exe>

STATUS: links to a PE32+ GUI executable; NOT run (no Windows or Wine here). Unverified: start-up, exceptions through the stub handler, CFG, stack
cookie, thread-local storage. Not for release use. See docs/VALIDATION_AOT.md.
