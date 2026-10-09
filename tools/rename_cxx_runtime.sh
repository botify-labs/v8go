#!/usr/bin/env bash
# Renames V8's C++ runtime symbols (tools/cxx-runtime-rename.map) in Linux
# archives, definitions and references alike, so that V8, Chromium's libc++
# and the v8go bridge bind to each other and never to libstdc++/libsupc++
# (tools/gen_cxx_runtime_rename_map.sh says why). The relocations follow the
# symbols, including the personality routine the .eh_frame CIEs reach through
# DW.ref.__gxx_personality_v0. The unwinder (_Unwind_*, libgcc) stays shared.
#
# Idempotent: an archive that no longer holds an original name is left as is.
# Usage: tools/rename_cxx_runtime.sh [archive ...]
#   (default: the V8, libc++, compiler-rt and bridge archives of deps/linux_*)
# Uses llvm-objcopy/llvm-nm (llvm-*-NN...), else $OBJCOPY/$NM (objcopy/nm).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# llvm_tool <name> <fallback>: llvm-<name>, else the newest llvm-<name>-NN, else <fallback>.
llvm_tool() {
  local t
  for t in "llvm-$1" $(compgen -c "llvm-$1-" | grep -E "^llvm-$1-[0-9]+$" | sort -t- -k3 -rn); do
    if command -v "$t" >/dev/null; then echo "$t"; return; fi
  done
  echo "$2"
}
NM=$(llvm_tool nm "${NM:-nm}")
OBJCOPY=$(llvm_tool objcopy "${OBJCOPY:-objcopy}")
MAP=tools/cxx-runtime-rename.map

libs=("$@")
if [ ${#libs[@]} -eq 0 ]; then
  for lib in deps/linux_*/{libv8-*,libc++-cr,libc++abi-cr,libclang_rt.builtins-cr,libv8go}.a; do
    [ -f "$lib" ] && libs+=("$lib")
  done
fi

for lib in "${libs[@]}"; do
  # Any original name, defined or referenced.
  n=$("$NM" -P "$lib" 2> >(grep -v ': no symbols$' >&2) |
    awk 'NR == FNR { if ($0 !~ /^#/) old[$1] = 1; next } NF >= 2 && ($1 in old) { n++ } END { print n + 0 }' "$MAP" -)
  if [ "$n" -eq 0 ]; then
    echo "$lib: nothing to rename"
    continue
  fi
  "$OBJCOPY" --redefine-syms="$MAP" "$lib"
  echo "$lib: renamed $n symbol occurrences"
done
