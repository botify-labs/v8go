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
git -C "$SRC" remote add origin https://github.com/botify-labs/v8go.git
git -C "$SRC" sparse-checkout set --no-cone '/*' '!/deps/android_*/' '!/cgo_android_*.go'
git -C "$SRC" fetch -q --depth=1 --filter=blob:none origin "$SHA"
git -C "$SRC" checkout -q FETCH_HEAD
rm -rf "$SRC/.git"

rsync -a --delete --exclude-from="$ROOT/tools/botify-owned.txt" "$SRC/" "$ROOT/"

cd "$ROOT"
rm -rf .gitmodules deps/v8 deps/depot_tools \
  .github/workflows/v8upgrade.yml .github/workflows/v8build.yml .github/workflows/release.yml
git rm -q -r --cached --ignore-unmatch deps/v8 deps/depot_tools >/dev/null

grep -rlZ -e 'github.com/botify-labs/v8go' \
  --include='*.go' --include='go.mod' --include='*.py' --include='*.sh' \
  --exclude-dir=.git --exclude-dir=bench --exclude-dir=docs . |
  xargs -0 -r sed -i 's#github\.com/tommie/v8go#github.com/botify-labs/v8go#g'

go mod edit \
  -droprequire=github.com/botify-labs/v8go/deps/android_amd64 \
  -droprequire=github.com/botify-labs/v8go/deps/android_arm64

for src in *.cc; do
  grep -q '^//go:build v8go_source' "$src" || sed -i '1i //go:build v8go_source\n' "$src"
done
for f in deps/*_*/cgo.go; do
  grep -q -- '-lv8go' "$f" ||
    sed -i '/^\/\/ #cgo LDFLAGS: -L\${SRCDIR}$/a // #cgo !v8go_source LDFLAGS: -lv8go\n// #cgo linux,!v8go_source LDFLAGS: -lm' "$f"
done

echo "$SHA" >deps/tommie_sha
echo "Imported tommie/v8go@$SHA (V8 $(cat deps/v8_hash))"
