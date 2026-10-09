//go:build linux && cxxprobe

package cxxprobe_test

import (
	"testing"

	v8 "github.com/botify-labs/v8go"
	"github.com/botify-labs/v8go/internal/cxxprobe"
)

// V8 (Chromium's libc++ and libc++abi) and g++-compiled C++ (libstdc++) in
// one binary: both runtimes throw, catch and use RTTI, each with its own
// symbols. Run fully static by botify-ci's static-cxx-probe job.
func TestCxxProbeWithV8(t *testing.T) {
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	v, err := ctx.RunScript(`[1, 2, 3].map(x => x * 2).join("-")`, "probe.js")
	if err != nil {
		t.Fatal(err)
	}
	const want = "caught probe_error: probe: 2-4-6; bad_alloc; stoi stoi; rethrown inner; square typeid ok; cout ok"
	if got := cxxprobe.Run(v.String()); got != want {
		t.Fatalf("probe:\n got %q\nwant %q", got, want)
	}

	// V8 keeps working after the probe has thrown: a JS exception, Intl (ICU)
	// and a Go callback.
	if _, err := ctx.RunScript(`throw new RangeError("js")`, "throw.js"); err == nil {
		t.Fatal("expected a JS exception")
	}
	if v, err := ctx.RunScript(`new Intl.NumberFormat("de-DE").format(1234.5)`, "intl.js"); err != nil || v.String() != "1.234,5" {
		t.Fatalf("Intl: %v, %v", v, err)
	}
	fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		r, _ := v8.NewValue(iso, cxxprobe.Run(info.Args()[0].String()))
		return r
	})
	if err := ctx.Global().Set("probe", fn.GetFunction(ctx)); err != nil {
		t.Fatal(err)
	}
	if v, err := ctx.RunScript(`probe("")`, "cb.js"); err != nil || v.String() != "caught other: empty input; bad_alloc; stoi stoi; rethrown inner; square typeid ok; cout ok" {
		t.Fatalf("probe from a callback: %v, %v", v, err)
	}
}
