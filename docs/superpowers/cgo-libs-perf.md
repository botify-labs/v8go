# cgo libraries rebuilt for the V8 upgrade: performance

Checked 2026-10-07: each C/C++ library rebuilt for the V8 upgrade, benchmarked
against the library it replaces on the same machine. Benchmarks are committed
next to each package. Linux ran in the v8go dev container
(`tools/docker/dev.sh`). Windows ran on one GitHub windows-latest runner per
repository, with the same Go 1.27.1 for both builds:

- MinGW: MinGW-Builds 13.2.0, msvcrt.
- MSVC: LLVM 21.1.8 with `CC="clang -fuse-ld=lld"`.

On Windows, the two builds run alternately for 10 rounds (20 for urlnorm
after the fixes) with `-benchmem`, then benchstat compares them. Old and new
builds give byte-identical outputs: URL normalization and gzip/deflate/zstd
frames. Go allocations don't change.

The two Windows regressions found on 2026-10-07 were fixed on 2026-10-08
(see "Fixes"). Current state:

| Library | Old -> new | sec/op (benchstat) | Verdict |
|---|---|---|---|
| liburlnorm, Linux | unchanged: `liburlnorm.a` + `libicu*.a` (the 2026-10-06 prelink was removed) | n/a | OK |
| liburlnorm, Windows | MinGW GCC 9.2 libs -> `lib/windows_msvc` (ICU and urlnorm via clang-cl, /O2) | EPYC 7763: +2.2%, all rows `~` (p >= 0.48); Xeon 8573C: +2.0% (IDN +4.3%, p=0.005; other rows `~`) | OK |
| igzip, cdf (ISA-L 2.31.1) | `libigzip.a` -> `igzip.lib` | -0.4%, all `~` (later runs +0.5% and +1.6%, all `~`) | OK |
| igzip, pulse (ISA-L 2.30.0) | `libigzip.a` -> `igzip.lib` | -3.7% on an AVX-512 CPU (Gzip -5.4%, p=0.000); -0.5% without AVX-512 | OK, faster |
| zstd, gocdf (1.5.0) | `libzstd_windows.a` -> `zstd_windows.lib` (with BMI2 paths) | +0.9%: Compress `~`; Uncompress +3.3% (p=0.009) at level 1, `~` (+1.8%) at level 3 | OK |

Before the fixes: liburlnorm on Windows was 4-12% slower (EPYC 7763: +11.6%,
Normalize +16.1% p=0.005, IDN +16.7% p=0.011), zstd decompression 16-18%
slower (+17.7% / +15.9%, p=0.000, seen in 2 runs).

## Fixes

**zstd: BMI2 decoder paths (gocdf `msvc-static-libs`, 1834e050).**

- Cause: zstd 1.5.0's `compiler.h` enables `DYNAMIC_BMI2` for clang, but
  defines `TARGET_ATTRIBUTE` only under `__GNUC__`, which clang-cl doesn't
  define. The `_bmi2` decoder variants were therefore built without BMI2: the
  `.lib` had 0 shrx/shlx/bzhi instructions, the MinGW `.a` has 293.
- Fix: `zstd-windows-msvc.yml` compiles with `/clang:-fgnuc-version=4.2.1`;
  the rebuilt `zstd_windows.lib` has 308 BMI2 instructions and is
  byte-identical to the workflow's build (`/Brepro`). The workflow now fails
  if the built or the committed `.lib` has fewer than 100 BMI2 instructions.
- Result (EPYC 7763): Uncompress within 3.3% of MinGW (was +17.7%), frames
  identical.

**liburlnorm on Windows: ICU compiled by clang-cl (cdf `upgrade-v8go`,
2e19a9c workflow, 261ba76 libs).**

- Cause: ICU 65 compiled by cl /O2. Built by clang-cl with the same
  `runConfigureICU MSYS/MSVC` line and flags, it recovers most of the gap.
- urlnorm's own code generation isn't the cause: clang `-O3` changed nothing,
  and neither did `/GS-` (no stack cookies, like MinGW gcc) for liburlnorm,
  ICU or both (+1.5% to +7.2% vs MinGW, within noise of the committed build).
  No fast-math. Nothing else adopted.
- The `go/urlnorm` and parser tests, including `TestNormalizePinned`, pass
  on Windows against the committed libraries. Every variant normalizes the
  64 benchmark URLs exactly like MinGW.
- Remaining gap: ~2% geomean. Only the IDN path (UTS 46 in ICU) is
  significant on one CPU (+4.3% on Xeon 8573C, `~` on EPYC 7763). Possible
  causes: the CRT (static UCRT vs msvcrt.dll) and the C++ standard library
  (MSVC STL vs libstdc++). Per URL it is ~0.02 us.

## Other observations

- pulse's old MinGW `libigzip.a` was assembled with yasm, so it has no
  AVX-512 kernels. The MSVC build, assembled with nasm, has them.
- cdf's two igzip builds have the same asm kernels.
- The runner's preinstalled MinGW targets UCRT, and MSYS2's GCC 16 uses a
  native-TLS libstdc++. Neither can link the old GCC 9.2 liburlnorm/ICU, so
  the bench pins MinGW-Builds 13.2.0 msvcrt.
- Runner noise: urlnorm benchmarks vary by 15-100% between rounds on the
  shared 2-vCPU runners, and one zstd run was spoiled by a noisy neighbour
  (rounds 5-8 twice as slow, run attempt discarded). Each verdict rests on at
  least 2 runs.
- The Linux liburlnorm prelink (`liburlnorm_prelinked.a`, measured at +0.4%,
  no regression) was removed from cdf (ebbccce): v8go now isolates its own
  C++ runtime, so the original libraries link statically with V8.

The full report, with every benchstat table, the flags of each build and the
run links, is in `.superpowers/sdd/2026-10-06-v8-upgrade/cgo-libs-perf-report.md`
(local). Bench workflows:

- cdf: `.github/workflows/windows-cgo-bench.yml`
- pulse: `.github/workflows/igzip-windows-bench.yml`
- gocdf: `.github/workflows/zstd-windows-bench.yml`

Runs:

- Before the fixes: cdf [37683241448](https://github.com/botify-hq/cdf/actions/runs/37683241448), [37685763298](https://github.com/botify-hq/cdf/actions/runs/37685763298); pulse [37683249454](https://github.com/botify-hq/pulse/actions/runs/37683249454); gocdf [37683257195](https://github.com/botify-hq/gocdf/actions/runs/37683257195)
- After the fixes: cdf [37691507467](https://github.com/botify-hq/cdf/actions/runs/37691507467) (attempts 1-2); gocdf [37690149823](https://github.com/botify-hq/gocdf/actions/runs/37690149823) (attempt 2)
- Library builds: cdf [37690288063](https://github.com/botify-hq/cdf/actions/runs/37690288063), [37691507553](https://github.com/botify-hq/cdf/actions/runs/37691507553) (committed libs); gocdf [37690149197](https://github.com/botify-hq/gocdf/actions/runs/37690149197)
