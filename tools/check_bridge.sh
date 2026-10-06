#!/usr/bin/env bash
# Fails if a prebuilt bridge doesn't match the current sources.
# Usage: tools/check_bridge.sh [os_arch ...]   (default: all deps/*_*)
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
[ ${#targets[@]} -gt 0 ] || targets=($(cd deps && ls -d *_*/ | tr -d /))
stale=0
for t in "${targets[@]}"; do
  [ -f "deps/$t/go.mod" ] || continue
  got=$(cat "deps/$t/bridge.sha256" 2>/dev/null || echo missing)
  if [ "$got" != "$want" ]; then
    echo "stale bridge: deps/$t ($got, want $want)"
    stale=1
  fi
done
exit $stale
