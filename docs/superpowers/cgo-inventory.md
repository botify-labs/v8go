# cgo dependencies of ftl and pulse

Purpose: on Windows everything will be linked statically with the MSVC-target clang toolchain, so every
cgo package that links a PREBUILT MinGW library (`-L... -l...` in `#cgo windows LDFLAGS`) has to be
rebuilt for MSVC. Packages compiled from source need nothing. On Linux, any prebuilt static C++ library
built against g++/libstdc++ hit a fully-static-link clash with V8's libc++abi. That clash is now gone
for every library: v8go renames V8's C++ runtime in its Linux archives (the liburlnorm prelink that
worked around it first was removed from cdf).

Inventory taken on 2026-10-06 from worktrees of the remote default branches (`ftl` = origin/master
@ 2cfc409f0, `pulse` = origin/main @ de3ffe99), in the dev container, with
`go list -deps ./...` under `GOOS=linux|windows GOARCH=amd64 CGO_ENABLED=1`. Lists are the cgo packages
(`.CgoFiles` non-empty) in the dependency closure. "PREBUILT" means `.CgoLDFLAGS` (resolved for the target
OS by `go list`) contains `-L`. Paths are shortened (`modcache/` = `modcache/`).
ftl root, ftl `backend/apps/cbjsex` and pulse `go/apps/lambda` are separate Go modules; pulse root and
lambda are in the same go.work, so they were run in workspace mode. The other ftl modules
(`cdn/fastly/compute/activation/bundlegen`, `modules/fastly/*`) and pulse `third_party/extsort` are tooling
and were not scanned.

## Raw output

## ftl (linux)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/cdf/go/pkg@v0.1.44/igzip/lib/linux -ligzip
- `github.com/botify-hq/cdf/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/linux -lurlnorm -licui18n -licuio -licutu -licuuc -licudata -ldl
- `github.com/botify-hq/ftl/backend/engine/api/dictionarybuild` | C++ sources: none | compiled from sources
- `github.com/botify-hq/ftl/backend/pkg/systat` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.19.11/compress/zstd/lib -lzstd_linux
- `net` | C++ sources: none | compiled from sources
- `os/user` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/linux_x86_64
- `runtime/cgo` | C++ sources: none | compiled from sources

## ftl (windows)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/cdf/go/pkg@v0.1.44/igzip/lib/windows -ligzip
- `github.com/botify-hq/cdf/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/windows -Wl,-Bstatic -lurlnorm -lsicuin -lsicuio -lsicutu -lsicuuc -lsicudt -v
- `github.com/botify-hq/ftl/backend/engine/api/dictionarybuild` | C++ sources: none | compiled from sources
- `github.com/botify-hq/ftl/backend/pkg/systat` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.19.11/compress/zstd/lib -lzstd_windows
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/windows_x86_64 -static -ldbghelp -lssp -lwinmm -lz
- `runtime/cgo` | C++ sources: none | compiled from sources

## ftl/backend/apps/cbjsex (linux)

- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/linux -lurlnorm -licui18n -licuio -licutu -licuuc -licudata -ldl
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.18.11/compress/zstd/lib -lzstd_linux
- `net` | C++ sources: none | compiled from sources
- `os/user` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/linux_x86_64
- `runtime/cgo` | C++ sources: none | compiled from sources

## ftl/backend/apps/cbjsex (windows)

- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/windows -Wl,-Bstatic -lurlnorm -lsicuin -lsicuio -lsicutu -lsicuuc -lsicudt -v
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.18.11/compress/zstd/lib -lzstd_windows
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/windows_x86_64 -static -ldbghelp -lssp -lwinmm -lz
- `runtime/cgo` | C++ sources: none | compiled from sources

## pulse (linux)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/linux -lurlnorm -licui18n -licuio -licutu -licuuc -licudata -ldl
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.16.16/compress/zstd/lib -lzstd_linux
- `github.com/botify-hq/pulse/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lpulse/go/pkg/igzip/lib/linux -ligzip
- `github.com/botify-hq/pulse/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `github.com/botify-hq/pulse/go/pulse/index/backend/wasm/fixtures` | C++ sources: none | compiled from sources
- `net` | C++ sources: none | compiled from sources
- `os/user` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/linux_x86_64
- `runtime/cgo` | C++ sources: none | compiled from sources

## pulse (windows)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/windows -Wl,-Bstatic -lurlnorm -lsicuin -lsicuio -lsicutu -lsicuuc -lsicudt -v
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.16.16/compress/zstd/lib -lzstd_windows
- `github.com/botify-hq/pulse/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lpulse/go/pkg/igzip/lib/windows -ligzip
- `github.com/botify-hq/pulse/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `github.com/botify-hq/pulse/go/pulse/index/backend/wasm/fixtures` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/windows_x86_64 -static -ldbghelp -lssp -lwinmm -lz
- `runtime/cgo` | C++ sources: none | compiled from sources
## pulse/go/apps/lambda (linux)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/linux -lurlnorm -licui18n -licuio -licutu -licuuc -licudata -ldl
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.16.16/compress/zstd/lib -lzstd_linux
- `github.com/botify-hq/pulse/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lpulse/go/pkg/igzip/lib/linux -ligzip
- `github.com/botify-hq/pulse/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `net` | C++ sources: none | compiled from sources
- `os/user` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/linux_x86_64
- `runtime/cgo` | C++ sources: none | compiled from sources

## pulse/go/apps/lambda (windows)

- `github.com/DataDog/zstd` | C++ sources: none | compiled from sources
- `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` | C++ sources: dummy.cpp | PREBUILT: -Lmodcache/botify-hq/cdf/liburlnorm/go/urlnorm@v0.1.1/lib/windows -Wl,-Bstatic -lurlnorm -lsicuin -lsicuio -lsicutu -lsicuuc -lsicudt -v
- `github.com/botify-hq/gocdf/v3/compress/lz4` | C++ sources: none | compiled from sources
- `github.com/botify-hq/gocdf/v3/compress/zstd` | C++ sources: none | PREBUILT: -Lmodcache/botify-hq/gocdf/v3@v3.16.16/compress/zstd/lib -lzstd_windows
- `github.com/botify-hq/pulse/go/pkg/igzip` | C++ sources: none | PREBUILT: -Lpulse/go/pkg/igzip/lib/windows -ligzip
- `github.com/botify-hq/pulse/go/pkg/jq/wasm` | C++ sources: none | compiled from sources
- `rogchap.com/v8go` | C++ sources: v8go.cc | PREBUILT: -pthread -lv8 -Lmodcache/botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985/deps/windows_x86_64 -static -ldbghelp -lssp -lwinmm -lz
- `runtime/cgo` | C++ sources: none | compiled from sources


## Packages to rebuild with MSVC for Windows

All of these are Windows packages whose LDFLAGS contain `-L` (prebuilt MinGW `.a`). Evidence that the
Windows archives are MinGW-built: undefined references to `___chkstk_ms`, `__mingw_vfprintf`,
`__mingw_vsnprintf` (libgcc/mingw-w64 runtime), which MSVC/clang-cl do not provide.

| Package | Used by | Windows prebuilt libs | Source repo / maintainer | Notes |
|---|---|---|---|---|
| `github.com/botify-hq/cdf/liburlnorm/go/urlnorm` v0.1.1 | ftl, cbjsex, pulse, lambda | `liburlnorm.a`, `libsicuin/io/tu/uc/dt.a` (ICU 65, static) | botify-hq/cdf (`liburlnorm/`), Botify (top committer jblernout) | C++ (`dummy.cpp` + urlnorm). Already handled by Task 17. Only this one needs libstdc++/libc++ care. |
| `github.com/botify-hq/gocdf/v3/compress/zstd` v3.19.11 (ftl), v3.18.11 (cbjsex), v3.16.16 (pulse) | ftl, cbjsex, pulse, lambda | `compress/zstd/lib/libzstd_windows.a` | botify-hq/gocdf, Botify (top committers diatboti, pcaneill) | C (zstd). References `___chkstk_ms`, `__mingw_vfprintf`, `__imp___acrt_iob_func` (MinGW UCRT). The `.a` is the same size in all three versions, so one rebuild should serve all. Package also carries `zstd/lib` C sources and `-I${SRCDIR}/zstd/lib/` CFLAGS, so a source build (like DataDog/zstd) may be an alternative. NEW task needed (same type as Task 17). |
| `github.com/botify-hq/cdf/go/pkg/igzip` v0.1.44 | ftl (root module only) | `igzip/lib/windows/libigzip.a` | botify-hq/cdf (`go/pkg/igzip`), Botify | ISA-L built with `make -f Makefile.unx arch=mingw` (nasm + gcc, per readme.txt). References `___chkstk_ms`. NEW task needed. |
| `github.com/botify-hq/pulse/go/pkg/igzip` | pulse, lambda | `go/pkg/igzip/lib/windows/libigzip.a` (in the pulse repo) | botify-hq/pulse (in-repo copy of the above, different binary and header) | Same ISA-L/MinGW recipe (readme says yasm). References `___chkstk_ms`. NEW task needed (pulse repo). |
| `rogchap.com/v8go` (replaced by `botify-labs/v8go@v0.0.0-20211129082619-6f9829d18985`) | ftl, cbjsex, pulse, lambda | `deps/windows_x86_64` (libv8) + `-static -ldbghelp -lssp -lwinmm -lz` | botify-labs/v8go fork | This is the module being replaced by this upgrade (Tasks of the plan), not a separate rebuild. |

Not in the build graph, but also prebuilt MinGW and would need the same treatment if ever enabled:
`pulse/go/pkg/jq/c` (`libjq_windows.a`, `libonig_windows.a`, `-lshlwapi`); the file is `//go:build exclude`,
so it is not compiled. The live jq implementation is `jq/wasm` (no cgo linkage).

## Prebuilt C++ libraries on Linux (libstdc++ clash risk)

- `liburlnorm` (+ `libicui18n/io/tu/uc/data.a`, `-ldl`): the only prebuilt C++ static library on Linux. 358
  libstdc++ symbol references in `liburlnorm.a`, 75/47/38/6 in the ICU archives. No action
  needed since v8go renames V8's C++ runtime (the prelink of Tasks 21-22 was removed).
- `rogchap.com/v8go` `libv8` (old fork): C++, replaced by this upgrade.
- Everything else prebuilt on Linux is plain C with zero libstdc++ references (checked with `nm`): 
  `libzstd_linux.a` (gocdf, `-lzstd_linux`), `libigzip.a` (cdf and pulse). No action needed.
- Linux system libs only: `net` (`-lresolv`), `runtime/cgo` (`-lpthread`).

## Compiled from source (nothing to do on Windows)

`github.com/DataDog/zstd`, `gocdf/v3/compress/lz4`, `jq/wasm`, `ftl/backend/engine/api/dictionarybuild`,
`ftl/backend/pkg/systat`, `pulse/go/pulse/index/backend/wasm/fixtures`, `os/user`, and `net` /
`runtime/cgo` (Go std; `net` links `-lresolv` on Linux only).

## Findings to flag

- Besides liburlnorm, two more prebuilt MinGW dependencies must be rebuilt for MSVC: gocdf `compress/zstd`
  and igzip (cdf copy for ftl, in-repo copy for pulse). Each needs a task of the same type as Task 17,
  though both are plain C (no libstdc++ issue), so the work is a straight MSVC/clang-cl rebuild, plus
  publishing a new module version of cdf and gocdf, and a pulse commit for its igzip.
- The three gocdf versions in use differ (v3.19.11 / v3.18.11 / v3.16.16): consumers must bump to a gocdf
  release that carries the MSVC `libzstd_windows.a`.
