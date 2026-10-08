# Sourced by login shells in the v8go dev container.

# Baseline v8go (V8 9.0) was built with gcc and libstdc++.
use_baseline() {
  export CC=gcc CXX=g++
  unset CGO_CXXFLAGS
}

# New v8go as consumers build it: system gcc, no flags, prebuilt bridge.
use_consumer() {
  export CC=gcc CXX=g++
  unset CGO_CXXFLAGS
}

# New v8go from its C++ sources (go build -tags v8go_source, tools/build_bridge.sh):
# Chromium's libc++ requires clang >= 21 and -nostdinc++.
use_source() {
  export CC=clang-21 CXX=clang++-21 CGO_CXXFLAGS=-nostdinc++
}

# Workspace for v8go itself: go.mod pins deps/* to commits that only exist once
# pushed, so the local deps module is used instead.
v8go_work() {
  cat >/tmp/v8go.work <<'EOF'
go 1.27

use (
	/src/v8go
	/src/v8go/deps/linux_amd64
)
EOF
  export GOWORK=/tmp/v8go.work
}

# Workspace for gojs, an internal consumer (bench/run.sh: GOJS_* variables).
# $1 is "baseline" (gojs on v8go 6f9829d, /src/v8go-baseline) or "new" (gojs
# on this v8go checkout).
gojs_work() {
  local dirs d
  case "$1" in
  baseline) dirs="${GOJS_BASELINE:?} ${GOJS_BASELINE_USE:-}" ;;
  new) dirs="${GOJS_UPGRADE:?} ${GOJS_UPGRADE_USE:-}" ;;
  *) echo "usage: gojs_work baseline|new" >&2; return 1 ;;
  esac
  {
    printf 'go 1.27\n\nuse (\n'
    for d in $dirs; do printf '\t%s\n' "$d"; done
    printf ')\n\n'
    if [ "$1" = baseline ]; then
      echo 'replace rogchap.com/v8go => /src/v8go-baseline'
    else
      # Unversioned replaces: local runs use this v8go checkout and its deps
      # modules whatever version gojs pins. v8go is not also listed in `use`:
      # Go rejects a go.work that replaces a workspace module at all versions.
      echo 'replace github.com/botify-labs/v8go => /src/v8go'
      for d in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64; do
        echo "replace github.com/botify-labs/v8go/deps/$d => /src/v8go/deps/$d"
      done
    fi
  } >/tmp/gojs.work
  export GOWORK=/tmp/gojs.work
}
