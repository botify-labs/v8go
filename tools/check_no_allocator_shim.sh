#!/usr/bin/env bash
# Fails if a V8 archive still holds PartitionAlloc's allocator shim, which would
# replace malloc/free/new/delete for the whole consumer process.
# tools/sync_tommie.sh strips it at import time.
# Usage: tools/check_no_allocator_shim.sh [os_arch ...]   (default: all deps/*_*)
# Reads ELF, Mach-O and COFF archives with llvm-ar/llvm-nm (llvm-nm-NN...);
# without them, $AR/$NM (default ar/nm).
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
AR=$(llvm_tool ar "${AR:-ar}")
NM=$(llvm_tool nm "${NM:-nm}")

# Members of PartitionAlloc's allocator_shim target (Linux allocator_shim.o,
# Apple allocator_shim_apple.o + malloc zone interception, Windows
# allocator_shim/*.obj).
SHIM_MEMBERS='allocator_shim|allocator/shim|allocator_interception_apple|malloc_zone_functions_apple'
# Process-wide allocator entry points: libc malloc & co ("_" prefix on Mach-O),
# UCRT ones, Itanium operator new/delete (_Znwm, _ZdlPv...) and MSVC ones
# (??2@, ??3@, ??_U@, ??_V@).
ALLOC_SYMS='_?(malloc|free|calloc|realloc|posix_memalign|aligned_alloc|memalign|valloc|pvalloc|malloc_usable_size|malloc_size)'
ALLOC_SYMS+='|_(malloc|free|calloc|realloc|recalloc|msize)_base|_(aligned_malloc|aligned_free|aligned_realloc|msize|recalloc|expand)'
ALLOC_SYMS+='|_?_Z(nw|na|dl|da)[^[:space:]]*|\?\?(2|3|_U|_V)@[^[:space:]]*'

targets=("$@")
[ ${#targets[@]} -gt 0 ] || targets=($(cd deps && ls -d *_*/go.mod | xargs -n1 dirname))
bad=0
for t in "${targets[@]}"; do
  if [ ! -f "deps/$t/go.mod" ]; then
    echo "deps/$t: not a platform directory" >&2
    bad=1
    continue
  fi
  for lib in "deps/$t"/libv8-*.a "deps/$t"/v8-*.lib; do
    [ -f "$lib" ] || continue
    if ! members=$("$AR" t "$lib"); then
      echo "$lib: $AR cannot read it" >&2
      bad=1
      continue
    fi
    if ! syms=$("$NM" -P -A --defined-only --extern-only "$lib" 2> >(grep -v ': no symbols$' >&2)); then
      echo "$lib: $NM cannot read it" >&2
      bad=1
      continue
    fi
    shim=$(grep -E "$SHIM_MEMBERS" <<<"$members" || true)
    defs=$(awk '{print $2 " " $1}' <<<"$syms" | grep -E "^($ALLOC_SYMS) " || true)
    if [ -n "$shim" ]; then
      echo "$lib: allocator shim member(s):"
      sed 's/^/  /' <<<"$shim"
      bad=1
    fi
    if [ -n "$defs" ]; then
      echo "$lib: defines allocator symbol(s):"
      sed 's/^/  /' <<<"$defs"
      bad=1
    fi
  done
done
exit $bad
