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

# This checkout in the container: tools/docker/dev.sh sets V8GO_DIR to
# /src/<the checkout's directory name>.
V8GO_DIR=${V8GO_DIR:-/src/v8go}

# Workspace for v8go itself: go.mod pins deps/* to commits that only exist once
# pushed, so the local deps module is used instead.
v8go_work() {
  cat >/tmp/v8go.work <<EOF
go 1.27

use (
	$V8GO_DIR
	$V8GO_DIR/deps/linux_amd64
)
EOF
  export GOWORK=/tmp/v8go.work
}
