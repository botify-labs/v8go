# cgo libraries rebuilt for the V8 upgrade: performance

Checked 2026-10-07: each C/C++ library rebuilt for the V8 upgrade, benchmarked
against the library it replaces on the same machine. Benchmarks are committed
next to each package. Linux ran in the v8go dev container
(`tools/docker/dev.sh`). Windows ran on one GitHub windows-latest runner per
repository, with the same Go 1.27.1 for both builds:

- MinGW: MinGW-Builds 13.2.0, msvcrt.
- MSVC: LLVM 21.1.8 with `CC="clang -fuse-ld=lld"`.

On Windows, the two builds run alternately for 10 rounds (`-benchmem`), then
benchstat compares them. Old and new builds give byte-identical outputs: URL
normalization and gzip/deflate/zstd frames. Go allocations don't change.

| Library | Old -> new | sec/op (benchstat) | Verdict |
|---|---|---|---|
| liburlnorm, Linux | `liburlnorm.a` + `libicu*.a` -> `liburlnorm_prelinked.a` | +0.4% dynamic, +0.4% static, all rows `~` (p >= 0.22). Old-dynamic -> new-static (the production pair): -1.9% | OK |
| liburlnorm, Windows | MinGW GCC 9.2 libs -> `lib/windows_msvc` (ICU via cl, urlnorm via clang-cl, both /O2) | +11.6% (Normalize +16.1% p=0.005, IDN +16.7% p=0.011) on EPYC 7763; +4.0% on EPYC 9V74 (IDN +9.8% p=0.009) | **regression** |
| igzip, cdf (ISA-L 2.31.1) | `libigzip.a` -> `igzip.lib` | -0.4%, all `~` | OK |
| igzip, pulse (ISA-L 2.30.0) | `libigzip.a` -> `igzip.lib` | -3.7% on an AVX-512 CPU (Gzip -5.4%, p=0.000); -0.5% without AVX-512 | OK, faster |
| zstd, gocdf (1.5.0) | `libzstd_windows.a` -> `zstd_windows.lib` | Compress ~0%; Uncompress +17.7% / +15.9% (p=0.000), seen in 2 runs | **regression** |

## Regressions

**zstd: decompression is 16-18% slower.**

- Cause: zstd 1.5.0's `compiler.h` enables `DYNAMIC_BMI2` for clang, but
  defines `TARGET_ATTRIBUTE` only under `__GNUC__`, which clang-cl doesn't
  define. The `_bmi2` decoder variants therefore get built without BMI2: the
  `.lib` has 0 shrx/shlx/bzhi instructions, the MinGW `.a` has 293.
- Fix: add `/clang:-fgnuc-version=4.2.1` to the clang-cl line of
  `zstd-windows-msvc.yml`. This is verified in gocdf's bench workflow:
  - 308 BMI2 instructions;
  - identical frames;
  - Uncompress 12.8% faster than the committed `.lib`, and within 2.6% of
    MinGW.
- Not committed yet: rebuild the library and run the compress tests first.

**liburlnorm on Windows: 4-12% slower, the IDN path the most.**

- urlnorm's own optimization level isn't the cause: clang `-O3` changes
  nothing.
- Building ICU with clang-cl instead of cl recovers about half the gap: +4.0%
  -> +0.2% and +11.9% -> +5.7%, with identical outputs.
- The rest is within runner noise. Candidate causes: the CRT (static UCRT vs
  msvcrt.dll), the C++ standard library (MSVC STL vs libstdc++), and `/GS`.
- Proposed fix: build ICU 65 with clang-cl in `windows-msvc-libs.yml` (no
  fast-math flags). Then re-measure.

## Other observations

- pulse's old MinGW `libigzip.a` was assembled with yasm, so it has no
  AVX-512 kernels. The MSVC build, assembled with nasm, has them.
- cdf's two igzip builds have the same asm kernels.
- The runner's preinstalled MinGW targets UCRT, and MSYS2's GCC 16 uses a
  native-TLS libstdc++. Neither can link the old GCC 9.2 liburlnorm/ICU, so
  the bench pins MinGW-Builds 13.2.0 msvcrt.

The full report, with every benchstat table, the flags of each build and the
run links, is in `.superpowers/sdd/2026-10-06-v8-upgrade/cgo-libs-perf-report.md`
(local). Bench workflows:

- cdf: `.github/workflows/windows-cgo-bench.yml`
- pulse: `.github/workflows/igzip-windows-bench.yml`
- gocdf: `.github/workflows/zstd-windows-bench.yml`

Runs:

- cdf: [37683241448](https://github.com/botify-hq/cdf/actions/runs/37683241448), [37685763298](https://github.com/botify-hq/cdf/actions/runs/37685763298)
- pulse: [37683249454](https://github.com/botify-hq/pulse/actions/runs/37683249454)
- gocdf: [37683257195](https://github.com/botify-hq/gocdf/actions/runs/37683257195)
