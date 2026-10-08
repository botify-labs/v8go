#!/usr/bin/env bash
# Fails if a prebuilt bridge is missing or doesn't match the current sources,
# or if a directory named on the command line is no deps module.
# Usage: tools/check_bridge.sh [os_arch ...]   (default: every deps/*_*/go.mod)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# cleanup.cc declares tommie's platform global as
# `extern std::unique_ptr<Platform> default_platform;`: a renamed global fails
# to link, but a changed type would compile into undefined behaviour.
if ! grep -qxF 'auto default_platform = platform::NewDefaultPlatform();' isolate.cc; then
  echo "isolate.cc no longer defines 'auto default_platform = platform::NewDefaultPlatform();'," \
    "which cleanup.cc (RunPendingTasks) relies on: update its extern declaration" >&2
  exit 1
fi

want=$(tools/bridge_hash.sh)
targets=("$@")
named=${#targets[@]}
# By default, the deps/<os>_<arch> modules: deps/include_libcxx and the like
# are no module.
[ "$named" -gt 0 ] || targets=($(cd deps && ls -d *_*/ | tr -d /))
stale=0
for t in "${targets[@]}"; do
  if [ ! -f "deps/$t/go.mod" ]; then
    if [ "$named" -gt 0 ]; then
      echo "deps/$t is no deps module (no go.mod)"
      stale=1
    fi
    continue
  fi
  archive=libv8go.a
  [[ "$t" != windows_* ]] || archive=v8go.lib
  if [ ! -f "deps/$t/$archive" ]; then
    echo "missing bridge: deps/$t/$archive"
    stale=1
    continue
  fi
  got=$(cat "deps/$t/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" != "$want" ]; then
    echo "stale bridge: deps/$t ($got, want $want)"
    stale=1
  fi
done
exit $stale
