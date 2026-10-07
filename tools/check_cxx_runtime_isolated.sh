#!/usr/bin/env bash
# Fails unless V8's C++ runtime is isolated in the Linux archives: no archive
# of deps/linux_* (V8, libc++, libc++abi, compiler-rt, the bridge) defines or
# references a symbol of tools/cxx-runtime-rename.map under its original
# name, and the map and the source-mode aliases
# (deps/linux_*/libv8go_cxxalias.a, botify_cxxalias_linux.go) are the ones
# tools/gen_cxx_runtime_rename_map.sh derives from the archives (a new libc++
# symbol is renamed too).
# tools/sync_tommie.sh and tools/build_bridge.sh apply the map
# (tools/rename_cxx_runtime.sh).
# Usage: tools/check_cxx_runtime_isolated.sh [os_arch ...]   (default: all deps/linux_*)
# Reads ELF archives with llvm-nm (llvm-nm-NN...), else $NM (default nm).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

llvm_tool() {
  local t
  for t in "llvm-$1" $(compgen -c "llvm-$1-" | grep -E "^llvm-$1-[0-9]+$" | sort -t- -k3 -rn); do
    if command -v "$t" >/dev/null; then echo "$t"; return; fi
  done
  echo "$2"
}
NM=$(llvm_tool nm "${NM:-nm}")
MAP=tools/cxx-runtime-rename.map

bad=0
gen=$(mktemp -d)
trap 'rm -rf "$gen"' EXIT
tools/gen_cxx_runtime_rename_map.sh "$gen"
for f in "$MAP" deps/linux_*/libv8go_cxxalias.a botify_cxxalias_linux.go; do
  if ! diff -u "$f" "$gen/$f" >&2; then
    echo "$f is stale: run tools/gen_cxx_runtime_rename_map.sh, then tools/rename_cxx_runtime.sh" >&2
    bad=1
  fi
done

targets=("$@")
[ ${#targets[@]} -gt 0 ] || targets=($(cd deps && ls -d linux_*/ | tr -d /))
for t in "${targets[@]}"; do
  n=0
  for lib in "deps/$t"/{libv8-*,libc++-cr,libc++abi-cr,libclang_rt.builtins-cr,libv8go}.a; do
    [ -f "$lib" ] || continue
    n=$((n + 1))
    if ! syms=$("$NM" -P "$lib" 2> >(grep -v ': no symbols$' >&2)); then
      echo "$lib: $NM cannot read it" >&2
      bad=1
      continue
    fi
    left=$(awk 'NR == FNR { if ($0 !~ /^#/) old[$1] = 1; next }
      NF >= 2 && ($1 in old) { print "  " $1 " (" ($2 ~ /^[Uvw]$/ ? "referenced" : "defined") ")" }' "$MAP" - <<<"$syms" |
      LC_ALL=C sort -u)
    if [ -n "$left" ]; then
      echo "$lib: C++ runtime symbol(s) not renamed (tools/rename_cxx_runtime.sh):"
      echo "$left"
      bad=1
    fi
  done
  if [ "$n" -lt 4 ]; then
    echo "deps/$t: expected the V8 and libc++ archives, found $n" >&2
    bad=1
  fi
done
[ $bad -eq 0 ] && echo "C++ runtime isolated: ${targets[*]}"
exit $bad
