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

// V8 posts GC work (memory reducer, GC jobs, ...) and FinalizationRegistry
// cleanups to the platform's foreground task queue of the isolate.
// Isolate.Cleanup must run them: v8go never does otherwise, so they piled up
// in native memory, and the memory reducer never collected old-space garbage
// on a long-lived isolate (Task 12a soak leak).
func TestIsolateCleanupRunsPendingPlatformTasks(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	const setup = `var cleaned = 0;
var registry = new FinalizationRegistry(() => { cleaned++; });
(function () { registry.register({}, 1); })();`
	if _, err := ctx.RunScript(setup, "registry.js"); err != nil {
		t.Fatal(err)
	}
	iso.LowMemoryNotification() // Collects the target, posts the cleanup task.
	ctx.Cleanup()
	iso.Cleanup()

	v, err := ctx.RunScript("cleaned", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	if v.Int32() != 1 {
		t.Fatalf("FinalizationRegistry callback ran %d times after GC and Cleanup, expected 1: Cleanup doesn't run V8's pending platform tasks", v.Int32())
	}
}

// Tasks pumped by Isolate.Cleanup can run JS that calls Go callbacks. Those
// may return the Isolate's cached Undefined/Null, which must still be valid
// then: Cleanup runs the tasks before it releases the internal values.
func TestIsolateCleanupTaskCallbacksCanReturnUndefinedAndNull(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	calls := 0
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("goUndefined", v8.NewFunctionTemplate(iso, func(*v8.FunctionCallbackInfo) *v8.Value {
		calls++
		return v8.Undefined(iso)
	})); err != nil {
		t.Fatal(err)
	}
	if err := global.Set("goNull", v8.NewFunctionTemplate(iso, func(*v8.FunctionCallbackInfo) *v8.Value {
		calls++
		return v8.Null(iso)
	})); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	const setup = `var u = 0, n = 0;
var registry = new FinalizationRegistry(() => { u = goUndefined(); n = goNull(); });
(function () { registry.register({}, 1); })();`
	if _, err := ctx.RunScript(setup, "registry.js"); err != nil {
		t.Fatal(err)
	}
	iso.LowMemoryNotification() // Collects the target, posts the cleanup task.
	iso.Cleanup()               // Runs the task: the callbacks return Undefined/Null.

	if calls != 2 {
		t.Fatalf("expected 2 Go callback calls during Cleanup, got %d", calls)
	}
	v, err := ctx.RunScript("u === undefined && n === null", "check.js")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Boolean() {
		t.Fatal("callbacks run during Cleanup returned a released Undefined/Null")
	}
	if !v8.Undefined(iso).IsUndefined() || !v8.Null(iso).IsNull() {
		t.Fatal("Undefined/Null unusable after Cleanup")
	}
}
