//go:build linux && cxxprobe

// Package cxxprobe is a test-only cgo package of C++ code compiled by the
// consumer's g++ against libstdc++, like a cgo library compiled with
// g++/libstdc++ (ICU, for instance) linked next to v8go. Its test links it
// into one binary with v8go,
// also fully statically: V8's C++ runtime (Chromium's libc++ and libc++abi)
// must not clash with libstdc++ (tools/cxx-runtime-rename.map, BOTIFY.md).
//
// Linux only, behind the cxxprobe tag (botify-ci's static-cxx-probe job):
//
//	CC=gcc CXX=g++ go test -tags 'netgo osusergo cxxprobe' \
//	  -ldflags "-linkmode external -extldflags '-static -lm'" ./internal/cxxprobe
package cxxprobe

// #cgo CXXFLAGS: -std=c++17 -O2 -Wall
// #include <stdlib.h>
// #include "cxxprobe.h"
import "C"

import "unsafe"

// Run runs the C++ probe on input: it throws and catches libstdc++ exceptions
// (std::runtime_error caught as std::exception, std::bad_alloc from operator
// new), uses RTTI (dynamic_cast, typeid), std::string and iostreams, and
// reports what it saw.
func Run(input string) string {
	in := C.CString(input)
	defer C.free(unsafe.Pointer(in))
	out := C.cxxprobe_run(in)
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out)
}
