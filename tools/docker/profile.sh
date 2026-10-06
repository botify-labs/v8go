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

# Workspace for gojs. $1 is "baseline" (cdf master + v8go 6f9829d) or "new"
# (cdf upgrade-v8go + local v8go).
gojs_work() {
  case "$1" in
  baseline)
    cat >/tmp/gojs.work <<'EOF'
go 1.27

use (
	/src/cdf-gojs-baseline/gojs
	/src/cdf-gojs-baseline/go/pkg
	/src/cdf-gojs-baseline/liburlnorm/go/urlnorm
)

replace rogchap.com/v8go => /src/v8go-baseline
EOF
    ;;
  new)
    cat >/tmp/gojs.work <<'EOF'
go 1.27

use (
	/src/cdf-gojs-upgrade/gojs
	/src/cdf-gojs-upgrade/go/pkg
	/src/cdf-gojs-upgrade/liburlnorm/go/urlnorm
	/src/v8go
	/src/v8go/deps/linux_amd64
)
EOF
    ;;
  *) echo "usage: gojs_work baseline|new" >&2; return 1 ;;
  esac
  export GOWORK=/tmp/gojs.work
}
