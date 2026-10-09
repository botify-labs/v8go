#!/usr/bin/env bash
# Pins the deps/<os>_<arch> modules required by go.mod to a commit already
# pushed to github.com/botify-labs/v8go, or to a release of those modules.
# Usage: tools/pin_deps.sh <commit>|<version>
#   <commit>:  a commit of this clone (full or abbreviated sha, branch, tag...),
#              pinned as a pseudo-version, or as a release when the commit
#              carries the deps/<os>_<arch>/vX.Y.Z tags;
#   <version>: vX.Y.Z, i.e. the tags deps/<os>_<arch>/vX.Y.Z (one per module,
#              all on the same commit), pinned as vX.Y.Z (BOTIFY.md, "Publier
#              une version").
# Consumers get the bridges from these pinned modules, not from deps/ in the
# commit they require: the commit must contain the bridges of the current
# sources (its deps/*/bridge.sha256 equal to tools/bridge_hash.sh), i.e. it is
# the botify-bridge commit or a later one, and its deps/<os>_<arch> trees
# must be HEAD's (tools/check_pinned_deps.sh checks both). Run it after every
# bridge rebuild, before tagging or merging; botify-ci's pinned-deps-fresh job
# (tools/check_pinned_deps.sh) fails until then.
# bench/go.mod gets the same versions: it replaces the deps modules with the
# local directories, but its requirements must still match the root module's.
set -euo pipefail
ARG=${1:?usage: tools/pin_deps.sh <pushed commit>|<vX.Y.Z>}
cd "$(git rev-parse --show-toplevel)"
export GOWORK=off GOPROXY=direct GOFLAGS=-mod=mod

# rev_of <deps dir>: the commit to pin for that module, as a full sha (Go gets
# the full sha too, or the version).
if [[ "$ARG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  rev_of() {
    git rev-parse -q --verify "refs/tags/$1/$ARG^{commit}" || {
      echo "no tag $1/$ARG in this clone (git fetch --tags?)" >&2
      return 1
    }
  }
  query() { echo "$ARG"; }
else
  SHA=$(git rev-parse -q --verify "$ARG^{commit}") || {
    echo "$ARG: no such commit in this clone, or an ambiguous abbreviation (git fetch? full sha?)" >&2
    exit 1
  }
  rev_of() { echo "$SHA"; }
  query() { echo "$SHA"; }
fi

want=$(tools/bridge_hash.sh)
for gomod in deps/*_*/go.mod; do
  d=$(dirname "$gomod")
  rev=$(rev_of "$d")
  got=$(git show "$rev:$d/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" != "$want" ]; then
    echo "$rev:$d/bridge.sha256 is $got, the sources are $want: pin the bridge commit" >&2
    exit 1
  fi
  if [ "$(git rev-parse "$rev:$d")" != "$(git rev-parse "HEAD:$d")" ]; then
    echo "$rev:$d differs from HEAD's (git diff $rev HEAD -- $d): pin a commit whose $d is HEAD's" >&2
    exit 1
  fi
done

# All versions first, then the edits: a failed lookup leaves go.mod and
# bench/go.mod untouched rather than half-pinned.
pins=()
for gomod in deps/*_*/go.mod; do
  mod="github.com/botify-labs/v8go/$(dirname "$gomod")"
  ver=$(go list -m -f '{{.Version}}' "$mod@$(query)")
  [ -n "$ver" ] || { echo "$mod@$(query): no version" >&2; exit 1; }
  pins+=("$mod@$ver")
done
for pin in "${pins[@]}"; do
  go mod edit -require="$pin"
  go mod edit -require="$pin" bench/go.mod
  echo "$pin"
done
go mod tidy
