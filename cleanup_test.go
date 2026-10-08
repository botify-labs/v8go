package v8go_test

import (
	"runtime"
	"testing"
	"time"

	v8 "github.com/botify-labs/v8go"
)

func TestContextCleanupReleasesValues(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	for i := 0; i < 100; i++ {
		if _, err := ctx.RunScript("({a: 1})", "cleanup.js"); err != nil {
			t.Fatal(err)
		}
	}
	if n := ctx.RetainedValueCount(); n < 100 {
		t.Fatalf("expected at least 100 retained values, got %d", n)
	}

	ctx.Cleanup()
	if n := ctx.RetainedValueCount(); n != 0 {
		t.Fatalf("expected 0 retained values after Cleanup, got %d", n)
	}

	v, err := ctx.RunScript("1 + 1", "after.js")
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 2 {
		t.Fatalf("expected 2, got %v", v)
	}
}

func TestIsolateCleanupReleasesInternalValues(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	base := iso.InternalRetainedValueCount()
	for i := 0; i < 100; i++ {
		if _, err := v8.NewValue(iso, "some string"); err != nil {
			t.Fatal(err)
		}
	}
	if n := iso.InternalRetainedValueCount(); n < base+100 {
		t.Fatalf("expected at least %d internal values, got %d", base+100, n)
	}

	iso.Cleanup()
	if n := iso.InternalRetainedValueCount(); n != base {
		t.Fatalf("expected %d internal values after Cleanup, got %d", base, n)
	}
}

func TestIsolateCleanupKeepsUndefinedAndNullUsable(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	iso.Cleanup()
	if !v8.Undefined(iso).IsUndefined() {
		t.Fatal("Undefined is not undefined after Cleanup")
	}
	if !v8.Null(iso).IsNull() {
		t.Fatal("Null is not null after Cleanup")
	}

	fn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		return v8.Undefined(info.Context().Isolate())
	})
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("f", fn); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()
	v, err := ctx.RunScript("f()", "undef.js")
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsUndefined() {
		t.Fatalf("expected undefined, got %v", v)
	}
}

func TestIsolateCleanupReleasesUnboundScripts(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	const src = "function add(a, b) { return a + b }; add(40, 2)"
	us, err := iso.CompileUnboundScript(src, "us.js", v8.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cache := us.CreateCodeCache()
	for i := 0; i < 9; i++ {
		if _, err := iso.CompileUnboundScript(src, "us.js", v8.CompileOptions{CachedData: cache}); err != nil {
			t.Fatal(err)
		}
	}
	if n := iso.InternalUnboundScriptCount(); n != 10 {
		t.Fatalf("expected 10 unbound scripts, got %d", n)
	}

	iso.Cleanup()
	if n := iso.InternalUnboundScriptCount(); n != 0 {
		t.Fatalf("expected 0 unbound scripts after Cleanup, got %d", n)
	}

	us, err = iso.CompileUnboundScript(src, "us.js", v8.CompileOptions{CachedData: cache})
	if err != nil {
		t.Fatal(err)
	}
	v, err := us.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 42 {
		t.Fatalf("expected 42, got %v", v)
	}
}

// cleanupPayload holds a pointer so that it isn't a tiny allocation: Go packs
// pointer-free objects under 16 bytes into shared 16-byte blocks, and a
// finalizer only runs once the whole block is unreachable, so it may never
// run while another goroutine's object in the block is live.
type cleanupPayload struct {
	n int
	_ *int
}

func TestIsolateCleanupKeepsGoValueReferencedByJS(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	v, err := v8.NewValue(iso, &cleanupPayload{n: 42})
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.Global().Set("goVal", v); err != nil {
		t.Fatal(err)
	}

	ctx.Cleanup()
	iso.Cleanup()
	iso.LowMemoryNotification()
	runtime.GC()

	got, err := ctx.Global().Get("goVal")
	if err != nil {
		t.Fatal(err)
	}
	ext, ok := got.External()
	if !ok {
		t.Fatal("goVal is no longer an External after Cleanup")
	}
	if p := ext.(*cleanupPayload); p.n != 42 {
		t.Fatalf("expected payload 42, got %d", p.n)
	}
}

func TestIsolateCleanupFreesUnreferencedGoValueHandle(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	collected := make(chan struct{})
	func() {
		p := &cleanupPayload{n: 1}
		runtime.SetFinalizer(p, func(*cleanupPayload) { close(collected) })
		if _, err := v8.NewValue(iso, p); err != nil {
			t.Fatal(err)
		}
	}()

	iso.Cleanup()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		iso.LowMemoryNotification() // V8 GC -> GoValueWeakCallback deletes the cgo.Handle.
		runtime.GC()                // Go GC -> finalizer runs once the handle is gone.
		select {
		case <-collected:
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.Fatal("Go value wrapped by NewValue was never released after Cleanup (cgo.Handle leak)")
}

func TestCleanupAfterTerminateExecution(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	go func() {
		time.Sleep(50 * time.Millisecond)
		iso.TerminateExecution()
	}()
	if _, err := ctx.RunScript("for (;;) {}", "loop.js"); err == nil {
		t.Fatal("expected termination error")
	}

	ctx.Cleanup()
	iso.Cleanup()
	v, err := ctx.RunScript("1 + 1", "after.js")
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 2 {
		t.Fatalf("expected 2, got %v", v)
	}
}

func TestCleanupWithCallbacksAndPromises(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	add := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		args := info.Args()
		v, err := v8.NewValue(iso, args[0].Int32()+args[1].Int32())
		if err != nil {
			t.Error(err)
		}
		return v
	})
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("goAdd", add); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	ctxBase := ctx.RetainedValueCount()
	isoBase := iso.InternalRetainedValueCount()
	for round := 0; round < 3; round++ {
		// var, not let: a top-level let can't be redeclared by the next round.
		v, err := ctx.RunScript("var s = 0; for (let i = 0; i < 1000; i++) s = goAdd(s, 1); new Promise(() => {}); s", "cb.js")
		if err != nil {
			t.Fatal(err)
		}
		if v.Int32() != 1000 {
			t.Fatalf("round %d: expected 1000, got %v", round, v)
		}
		ctx.Cleanup()
		iso.Cleanup()
		if n := ctx.RetainedValueCount(); n > ctxBase {
			t.Fatalf("round %d: context retains %d values (base %d)", round, n, ctxBase)
		}
		if n := iso.InternalRetainedValueCount(); n > isoBase {
			t.Fatalf("round %d: isolate retains %d values (base %d)", round, n, isoBase)
		}
	}
}

func TestCleanupIsIdempotentAndNilSafe(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	ctx := v8.NewContext(iso)
	for i := 0; i < 3; i++ {
		ctx.Cleanup()
		iso.Cleanup()
	}
	ctx.Close()
	ctx.Cleanup() // after Close: no-op
	iso.Dispose()
	iso.Cleanup() // after Dispose: no-op
}

// finishesWithin runs f, and fails the test if it hasn't returned after d. It
// then terminates the JavaScript f is stuck in, so that the test ends.
func finishesWithin(t *testing.T, iso *v8.Isolate, d time.Duration, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
		return
	case <-time.After(d):
	}
	t.Errorf("%s did not return within %v", what, d)
	for {
		iso.TerminateExecution()
		select {
		case <-done:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// collectRegistryTarget makes V8 collect the target registered by setup and
// post the FinalizationRegistry's cleanup task.
func collectRegistryTarget(t *testing.T, iso *v8.Isolate, ctx *v8.Context, setup string) {
	t.Helper()
	if _, err := ctx.RunScript(setup, "registry.js"); err != nil {
		t.Fatal(err)
	}
	iso.LowMemoryNotification()
}

// V8 posts GC work (memory reducer, GC jobs, ...) and FinalizationRegistry
// cleanups to the platform's foreground task queue of the isolate, which
// Isolate.Cleanup runs. It must not run JavaScript though, as the old fork,
// which never ran these tasks, didn't: the page is done, and nothing would
// stop a callback that loops.
func TestIsolateCleanupDoesNotRunFinalizationCallbacks(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	collectRegistryTarget(t, iso, ctx, `var cleaned = 0;
var registry = new FinalizationRegistry(() => { cleaned++; });
(function () { for (let i = 0; i < 10; i++) registry.register({}, i); })();`)
	ctx.Cleanup()
	iso.Cleanup()

	v, err := ctx.RunScript("cleaned", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 0 {
		t.Fatalf("Isolate.Cleanup ran JavaScript: %d FinalizationRegistry callbacks", v.Int32())
	}
	// The isolate is usable afterwards: Cleanup leaves no termination behind.
	if v, err := ctx.RunScript("cleaned + 2", "after.js"); err != nil || v.Int32() != 2 {
		t.Fatalf("after Cleanup: %v, %v", v, err)
	}
}

// A FinalizationRegistry callback that never returns must not hang Cleanup.
func TestIsolateCleanupDoesNotHangOnLoopingFinalizationCallback(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	collectRegistryTarget(t, iso, ctx, `var registry = new FinalizationRegistry(() => { for (;;) {} });
(function () { registry.register({}, 0); })();
for (let i = 0; i < 1000; i++) new Array(1000);`)
	finishesWithin(t, iso, 20*time.Second, "Isolate.Cleanup", iso.Cleanup)

	if v, err := ctx.RunScript("1 + 1", "after.js"); err != nil || v.Int32() != 2 {
		t.Fatalf("after Cleanup: %v, %v", v, err)
	}
}

// Go functions, called directly as the FinalizationRegistry callback or from a
// JavaScript one, aren't called during Cleanup either, also when their
// Context is closed (they would get no Context).
func TestIsolateCleanupDoesNotCallGoFromTasks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, setup string
		closeCtx    bool
	}{
		{"js callback", `var r = new FinalizationRegistry(() => { goFn(); });`, false},
		{"go callback", `var r = new FinalizationRegistry(goFn);`, false},
		{"js callback, closed context", `var r = new FinalizationRegistry(() => { goFn(); });`, true},
		{"go callback, closed context", `var r = new FinalizationRegistry(goFn);`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iso := v8.NewIsolate()
			defer iso.Dispose()
			calls := 0
			global := v8.NewObjectTemplate(iso)
			if err := global.Set("goFn", v8.NewFunctionTemplate(iso, func(*v8.FunctionCallbackInfo) *v8.Value {
				calls++
				return v8.Undefined(iso)
			})); err != nil {
				t.Fatal(err)
			}
			ctx := v8.NewContext(iso, global)
			collectRegistryTarget(t, iso, ctx, tc.setup+"\n(function () { r.register({}, 1); })();")
			if tc.closeCtx {
				ctx.Close()
			} else {
				defer ctx.Close()
			}
			iso.Cleanup()
			if calls != 0 {
				t.Fatalf("Isolate.Cleanup called a Go function %d times", calls)
			}
			if !v8.Undefined(iso).IsUndefined() || !v8.Null(iso).IsNull() {
				t.Fatal("Undefined/Null unusable after Cleanup")
			}
		})
	}
}

// Context.Cleanup and Isolate.Cleanup release the values of the current call
// (its receiver, its arguments, the caller's values) when called from a
// FunctionCallback: they do nothing there.
func TestCleanupInsideFunctionCallbackIsNoop(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var ctxBefore, ctxAfter, isoBefore, isoAfter int
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("f", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		if _, err := v8.NewValue(iso, "internal"); err != nil {
			t.Error(err)
		}
		ctx := info.Context()
		ctxBefore, isoBefore = ctx.RetainedValueCount(), iso.InternalRetainedValueCount()
		ctx.Cleanup()
		iso.Cleanup()
		ctxAfter, isoAfter = ctx.RetainedValueCount(), iso.InternalRetainedValueCount()
		return info.Args()[0]
	})); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	v, err := ctx.RunScript("f(41) + 1", "cb.js")
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 42 {
		t.Fatalf("expected 42, got %v", v)
	}
	if ctxAfter != ctxBefore {
		t.Errorf("Context.Cleanup inside a callback released values: %d -> %d", ctxBefore, ctxAfter)
	}
	if isoAfter != isoBefore {
		t.Errorf("Isolate.Cleanup inside a callback released values: %d -> %d", isoBefore, isoAfter)
	}

	// Outside a callback, both still release them.
	ctx.Cleanup()
	iso.Cleanup()
	if n := ctx.RetainedValueCount(); n != 0 {
		t.Errorf("Context.Cleanup outside a callback: %d values left", n)
	}
	if n := iso.InternalRetainedValueCount(); n >= isoBefore {
		t.Errorf("Isolate.Cleanup outside a callback: %d internal values left (%d before)", n, isoBefore)
	}
}
