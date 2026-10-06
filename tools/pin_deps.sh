#!/usr/bin/env bash
# Pins the deps/<os>_<arch> modules required by go.mod to a commit already
# pushed to github.com/botify-labs/v8go. Usage: tools/pin_deps.sh <commit-sha>
# bench/go.mod gets the same versions: it replaces the deps modules with the
# local directories, but its requirements must still match the root module's.
set -euo pipefail
SHA=${1:?usage: tools/pin_deps.sh <pushed-commit-sha>}
cd "$(git rev-parse --show-toplevel)"
export GOWORK=off GOPROXY=direct GOFLAGS=-mod=mod

for gomod in deps/*_*/go.mod; do
  mod="github.com/botify-labs/v8go/$(dirname "$gomod")"
  ver=$(go list -m -f '{{.Version}}' "$mod@$SHA")
  go mod edit -require="$mod@$ver"
  go mod edit -require="$mod@$ver" bench/go.mod
  echo "$mod $ver"
done
go mod tidy
