package v8go_test

import (
	"errors"
	"testing"
	"time"

	v8 "github.com/botify-labs/v8go"
)

const heapLimitLoop = `{ const a = []; for (;;) a.push({x: [1, 2, 3]}); }`

// runTerminated runs a script that loops until another goroutine terminates
// it, and returns its error.
func runTerminated(t *testing.T, iso *v8.Isolate, ctx *v8.Context) error {
	t.Helper()
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				iso.TerminateExecution()
			}
		}
	}()
	_, err := ctx.RunScript("for (;;) {}", "loop.js")
	if err == nil {
		t.Fatal("loop.js: expected a termination error")
	}
	return err
}

// The heap limit reached in a microtask, after the script's result: no error
// reports it, but HeapLimitReached does until Cleanup, which forgets it. A
// later termination isn't a heap limit one.
func TestHeapLimitInMicrotaskNotReportedLater(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate(v8.WithResourceConstraints(16<<20, 64<<20))
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	_, err := ctx.RunScript(`Promise.resolve().then(() => `+heapLimitLoop+`); 0`, "micro.js")
	t.Logf("micro.js: %v", err)
	if !iso.HeapLimitReached() {
		t.Error("HeapLimitReached: got false before Cleanup, want true")
	}
	ctx.Cleanup()
	iso.Cleanup()
	if iso.HeapLimitReached() {
		t.Error("HeapLimitReached: got true after Cleanup, want false")
	}

	if err := runTerminated(t, iso, ctx); errors.Is(err, v8.ErrHeapLimitReached) {
		t.Fatalf("a TerminateExecution reported as the heap limit: %v", err)
	}
}

// A script run by a Go callback reaches the heap limit: the termination goes
// on through the outer script, whose error reports the heap limit too.
func TestNestedHeapLimitReportedToOuterScript(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate(v8.WithResourceConstraints(16<<20, 64<<20))
	defer iso.Dispose()
	var ctx *v8.Context
	var nestedErr error
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("nested", v8.NewFunctionTemplateWithError(iso, func(*v8.FunctionCallbackInfo) (*v8.Value, error) {
		_, err := ctx.RunScript(heapLimitLoop, "nested.js")
		nestedErr = err
		return nil, err
	})); err != nil {
		t.Fatal(err)
	}
	ctx = v8.NewContext(iso, global)
	defer ctx.Close()

	_, err := ctx.RunScript("try { nested() } catch (e) {} 'caught'", "outer.js")
	if !errors.Is(nestedErr, v8.ErrHeapLimitReached) {
		t.Errorf("nested script: got %v, want ErrHeapLimitReached", nestedErr)
	}
	if !errors.Is(err, v8.ErrHeapLimitReached) {
		t.Errorf("outer script: got %v, want ErrHeapLimitReached", err)
	}

	// Reported, the heap limit isn't reported again, but HeapLimitReached
	// still tells until Cleanup.
	if err := runTerminated(t, iso, ctx); errors.Is(err, v8.ErrHeapLimitReached) {
		t.Fatalf("a TerminateExecution reported as the heap limit: %v", err)
	}
	if !iso.HeapLimitReached() {
		t.Error("HeapLimitReached: got false before Cleanup, want true")
	}
	iso.Cleanup()
	if iso.HeapLimitReached() {
		t.Error("HeapLimitReached: got true after Cleanup, want false")
	}
}

func TestHeapLimitReachedFalseWithoutHeapLimit(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	ctx := v8.NewContext(iso)
	if err := runTerminated(t, iso, ctx); errors.Is(err, v8.ErrHeapLimitReached) {
		t.Fatalf("a TerminateExecution reported as the heap limit: %v", err)
	}
	if iso.HeapLimitReached() {
		t.Error("HeapLimitReached: got true, want false")
	}
	ctx.Close()
	iso.Dispose()
	if iso.HeapLimitReached() {
		t.Error("HeapLimitReached after Dispose: got true, want false")
	}
}
