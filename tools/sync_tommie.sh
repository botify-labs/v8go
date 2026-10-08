#!/usr/bin/env bash
# Imports a tommie/v8go snapshot into this repository, renamed to
# github.com/botify-labs/v8go. Files matching tools/botify-owned.txt are kept.
# Android and tommie's V8 build/upgrade workflows are not imported.
# The C++ sources are restricted to -tags v8go_source: consumers link the
# prebuilt bridge (tools/build_bridge.sh) instead of compiling them.
#
# Usage: tools/sync_tommie.sh <tommie-commit-sha>
set -euo pipefail

SHA=${1:?usage: tools/sync_tommie.sh <tommie-commit-sha>}
ROOT=$(git rev-parse --show-toplevel)
SRC=$(mktemp -d)
trap 'rm -rf "$SRC"' EXIT

git -C "$SRC" init -q
git -C "$SRC" remote add origin https://github.com/tommie/v8go.git
git -C "$SRC" sparse-checkout set --no-cone '/*' '!/deps/android_*/' '!/cgo_android_*.go'
git -C "$SRC" fetch -q --depth=1 --filter=blob:none origin "$SHA"
git -C "$SRC" checkout -q FETCH_HEAD
EXEC=$(cd "$SRC" && find . -type f -perm -u+x ! -path './.git/*' | sed 's#^\./##')
rm -rf "$SRC/.git"

rsync -a --delete --exclude-from="$ROOT/tools/botify-owned.txt" "$SRC/" "$ROOT/"

cd "$ROOT"
rm -rf .gitmodules deps/v8 deps/depot_tools \
  .github/workflows/v8upgrade.yml .github/workflows/v8build.yml .github/workflows/release.yml
git rm -q -r --cached --ignore-unmatch deps/v8 deps/depot_tools >/dev/null

grep -rlZ -e 'github.com/tommie/v8go' \
  --include='*.go' --include='go.mod' --include='*.py' --include='*.sh' \
  --exclude-dir=.git --exclude-dir=bench --exclude-dir=docs --exclude=sync_tommie.sh . |
  xargs -0 -r sed -i 's#github\.com/tommie/v8go#github.com/botify-labs/v8go#g'

go mod edit \
  -droprequire=github.com/botify-labs/v8go/deps/android_amd64 \
  -droprequire=github.com/botify-labs/v8go/deps/android_arm64

# The tag sits between clang-format off/on markers: clang-format would rewrite
# a bare first-line `//go:build` into `// go:build`, which silently drops the
# constraint. Go accepts build constraints preceded by other line comments.
# Idempotent; also upgrades files that only carry the bare single line.
HDR=$'// clang-format off\n//go:build v8go_source\n// clang-format on'
for src in *.cc; do
  [ "$(sed -n 1,3p "$src")" = "$HDR" ] && continue
  # Drop a bare tag on line 1, and line 2 too when it is blank.
  sed -i '1{/^\/\/ \?go:build v8go_source$/{N;s/^[^\n]*\n//;/^$/d}}' "$src"
  sed -i '1i // clang-format off\n//go:build v8go_source\n// clang-format on\n' "$src"
done
for f in deps/*_*/cgo.go; do
  grep -q -- '-lv8go' "$f" ||
    sed -i '/^\/\/ #cgo LDFLAGS: -L\${SRCDIR}$/a // #cgo !v8go_source LDFLAGS: -lv8go' "$f"
  # Consumers link with $CC, which doesn't add libm like $CXX does. -lm goes
  # after the V8 archives: Ubuntu's gcc links with --as-needed, which drops a
  # shared library that nothing before it on the command line needs.
  sed -i '/^\/\/ #cgo linux,!v8go_source LDFLAGS: -lm$/d' "$f"
  sed -i '/^\/\/ #cgo LDFLAGS: .*-lv8-0 /a // #cgo linux,!v8go_source LDFLAGS: -lm' "$f"
  # Source mode: the linker script that aliases the original names of V8's
  # C++ runtime, renamed below (tools/gen_cxx_runtime_rename_map.sh). Before
  # the archives, so that its EXTERNs load their members.
  grep -q -- '-lv8go_cxxalias' "$f" ||
    sed -i '/^\/\/ #cgo LDFLAGS: -L\${SRCDIR}$/a // #cgo linux,v8go_source LDFLAGS: -lv8go_cxxalias' "$f"
done
# The seds above match tommie's cgo.go lines: stop if one of them changed shape.
for f in deps/*_*/cgo.go; do
  for line in '// #cgo !v8go_source LDFLAGS: -lv8go' '// #cgo linux,!v8go_source LDFLAGS: -lm' \
    '// #cgo linux,v8go_source LDFLAGS: -lv8go_cxxalias'; do
    if ! grep -qxF -- "$line" "$f"; then
      echo "$f: missing '$line' (tommie's cgo.go changed shape): update tools/sync_tommie.sh" >&2
      exit 1
    fi
  done
done

# Botify's changes to tommie's files, in order: each patch applies to the
# result of the previous ones. One that no longer applies stops the import:
# update it against the new snapshot (BOTIFY.md).
for p in tools/patches/*.patch; do
  if ! git apply --check "$p"; then
    echo "$p no longer applies to tommie/v8go@$SHA: update it (BOTIFY.md)" >&2
    exit 1
  fi
  git apply "$p"
done

# PartitionAlloc's allocator shim would replace malloc/free/new/delete for the
# whole consumer process: strip its members from the V8 archives. llvm-ar keeps
# each archive's format (GNU, Darwin, COFF) and writes deterministic archives.
llvm_tool() {
  local t
  for t in "llvm-$1" $(compgen -c "llvm-$1-" | grep -E "^llvm-$1-[0-9]+$" | sort -t- -k3 -rn); do
    if command -v "$t" >/dev/null; then echo "$t"; return; fi
  done
  echo "llvm-$1 not found" >&2
  return 1
}
AR=$(llvm_tool ar)
RANLIB=$(llvm_tool ranlib)
SHIM_MEMBERS='allocator_shim|allocator/shim|allocator_interception_apple|malloc_zone_functions_apple'
for lib in deps/*_*/libv8-*.a deps/*_*/v8-*.lib; do
  [ -f "$lib" ] || continue
  shim=$("$AR" t "$lib" | grep -E "$SHIM_MEMBERS" || true)
  [ -z "$shim" ] || {
    echo "$shim" | tr '\n' '\0' | xargs -0 "$AR" dP "$lib"
    "$RANLIB" "$lib"
  }
done
tools/check_no_allocator_shim.sh

# Chromium's libc++abi and libc++ define the C++ ABI symbols libstdc++
# defines too (__cxa_*, std::exception, operator new...): rename them in the
# Linux archives, so that a consumer can link g++-built C++ next to v8go, even
# fully statically. The map and the source-mode alias scripts are derived
# from the new archives.
tools/gen_cxx_runtime_rename_map.sh
tools/rename_cxx_runtime.sh

[ -z "$EXEC" ] || { echo "$EXEC" | xargs git add -- && echo "$EXEC" | xargs git update-index --chmod=+x --; }

echo "$SHA" >deps/tommie_sha
tools/check_cxx_runtime_isolated.sh
echo "Imported tommie/v8go@$SHA (V8 $(cat deps/v8_hash))"
