package v8go_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v8 "github.com/botify-labs/v8go"
)

// Botify: a termination must reach the top-level script, whatever the Go
// callbacks it runs through do with the isolate (botify_termination.h,
// tools/patches/0008-termination.patch).
//
// V8 15.4 clears a pending exception, the termination included, on entry to
// most API calls (PrepareForExecutionScope): a Go callback that made any V8
// call after a terminated nested script, or after a JavaScript toString it
// called was terminated, used to swallow the termination, and the outer
// script went on running.

// terminatedWithin waits for done, and fails the test if the script didn't
// end within d, after which it calls stop and requests termination until the
// script ends.
func terminatedWithin(t *testing.T, iso *v8.Isolate, done <-chan error, d time.Duration, stop func()) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
	}
	stop()
	for {
		iso.TerminateExecution()
		select {
		case err := <-done:
			t.Errorf("a single TerminateExecution was lost: the script ended only when told to (%v)", err)
			return err
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func isTermination(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ExecutionTerminated")
}

// A callback runs a nested script, which a single TerminateExecution
// terminates, then makes one more V8 call before returning the nested
// script's error: the outer script must be terminated.
func TestTerminationSurvivesLaterCallsInCallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		call func(ctx *v8.Context) error
	}{
		{"None", func(*v8.Context) error { return nil }},
		{"ObjectGet", func(ctx *v8.Context) error { _, err := ctx.Global().Get("Object"); return err }},
		{"JSONParse", func(ctx *v8.Context) error { _, err := v8.JSONParse(ctx, "1"); return err }},
		{"JSONStringify", func(ctx *v8.Context) error { _, err := v8.JSONStringify(ctx, ctx.Global()); return err }},
		{"RunScript", func(ctx *v8.Context) error { _, err := ctx.RunScript("1", "x.js"); return err }},
		// Looping JavaScript after the termination: terminated too, as with
		// V8 9.0, rather than run until another TerminateExecution.
		{"RunScriptLoop", func(ctx *v8.Context) error { _, err := ctx.RunScript("for (;;) {}", "loop.js"); return err }},
		{"NewValueString", func(ctx *v8.Context) error { _, err := v8.NewValue(ctx.Isolate(), "s"); return err }},
		{"ValueStringOfNumber", func(ctx *v8.Context) error {
			v, err := v8.NewValue(ctx.Isolate(), int32(7))
			if err == nil {
				_ = v.String()
			}
			return err
		}},
		{"ObjectTemplateNewInstance", func(ctx *v8.Context) error {
			_, err := v8.NewObjectTemplate(ctx.Isolate()).NewInstance(ctx)
			return err
		}},
		// Then and Catch panic on an error: they must not fail.
		{"PromiseThen", func(ctx *v8.Context) error {
			r, err := v8.NewPromiseResolver(ctx)
			if err == nil {
				r.GetPromise().Then(func(*v8.FunctionCallbackInfo) *v8.Value { return nil })
			}
			return err
		}},
		{"PromiseThen2Catch", func(ctx *v8.Context) error {
			r, err := v8.NewPromiseResolver(ctx)
			if err == nil {
				noop := func(*v8.FunctionCallbackInfo) *v8.Value { return nil }
				r.GetPromise().Then(noop, noop).Catch(noop)
			}
			return err
		}},
		{"ResolveThenable", func(ctx *v8.Context) error {
			r, err := v8.NewPromiseResolver(ctx)
			if err != nil {
				return err
			}
			v, err := ctx.Global().Get("thenable")
			if err == nil {
				r.Resolve(v)
			}
			return err
		}},
		{"FunctionTemplateGetFunction", func(ctx *v8.Context) error {
			fn := v8.NewFunctionTemplate(ctx.Isolate(), func(*v8.FunctionCallbackInfo) *v8.Value { return nil })
			_ = fn.GetFunction(ctx)
			return nil
		}},
		{"ValueObjectAndDetailString", func(ctx *v8.Context) error {
			v, err := v8.NewValue(ctx.Isolate(), int32(7))
			if err == nil {
				_ = v.Object()
				_ = v.DetailString()
			}
			return err
		}},
		// Conversions whose JavaScript is terminated return zero values
		// rather than abort the process.
		{"Int32OfValueOf", func(ctx *v8.Context) error {
			v, err := ctx.Global().Get("valueOfLoops")
			if err == nil {
				_, _, _, _ = v.Int32(), v.Integer(), v.Number(), v.Uint32()
			}
			return err
		}},
		{"ProxyHasDelete", func(ctx *v8.Context) error {
			v, err := ctx.Global().Get("proxy")
			if err == nil {
				o, _ := v.AsObject()
				_, _ = o.Has("x"), o.Delete("x")
			}
			return err
		}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			iso := v8.NewIsolate()
			defer iso.Dispose()
			var ctx *v8.Context
			var stop atomic.Bool // Ends the outer script if the test fails.
			global := v8.NewObjectTemplate(iso)
			if err := global.Set("nested", v8.NewFunctionTemplateWithError(iso, func(*v8.FunctionCallbackInfo) (*v8.Value, error) {
				if stop.Load() {
					return v8.NewValue(iso, "stop")
				}
				_, err := ctx.RunScript("for (;;) {}", "nested.js")
				if err == nil {
					err = fmt.Errorf("nested script returned")
				}
				_ = tc.call(ctx)
				return nil, err
			})); err != nil {
				t.Fatal(err)
			}
			ctx = v8.NewContext(iso, global)
			defer ctx.Close()
			// Values whose use runs looping JavaScript, for the calls above.
			if _, err := ctx.RunScript(`var thenable = { get then() { for (;;) {} } };
				var valueOfLoops = { valueOf() { for (;;) {} } };
				var proxy = new Proxy({}, { has() { for (;;) {} }, deleteProperty() { for (;;) {} } });`, "values.js"); err != nil {
				t.Fatal(err)
			}

			done := make(chan error, 1)
			go func() {
				_, err := ctx.RunScript("for (;;) { try { if (nested() === 'stop') break } catch (e) {} }", "outer.js")
				done <- err
			}()
			time.Sleep(100 * time.Millisecond)
			iso.TerminateExecution()
			if err := terminatedWithin(t, iso, done, 3*time.Second, func() { stop.Store(true) }); !isTermination(err) {
				t.Errorf("outer script: got %v, want a termination", err)
			}
			if v, err := ctx.RunScript("1 + 1", "after.js"); err != nil || v.Int32() != 2 {
				t.Errorf("after the termination: %v, %v", v, err)
			}
		})
	}
}

// A callback converts its arguments with String(): the first one's toString
// loops, and a single TerminateExecution terminates it; the second one is a
// number. The run must end, terminated, also when its script would end
// right after the call, and the next script must run normally.
func TestTerminationInArgumentToString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, after string }{
		{"Loop", "for (;;) { if (stopped()) break }"},
		{"End", "'done'"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			iso := v8.NewIsolate()
			defer iso.Dispose()
			var stop atomic.Bool
			global := v8.NewObjectTemplate(iso)
			if err := global.Set("f", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
				_ = info.Args()[0].String()
				_ = info.Args()[1].String()
				return nil
			})); err != nil {
				t.Fatal(err)
			}
			if err := global.Set("stopped", v8.NewFunctionTemplate(iso, func(*v8.FunctionCallbackInfo) *v8.Value {
				v, _ := v8.NewValue(iso, stop.Load())
				return v
			})); err != nil {
				t.Fatal(err)
			}
			ctx := v8.NewContext(iso, global)
			defer ctx.Close()

			done := make(chan error, 1)
			go func() {
				_, err := ctx.RunScript(`var o = { toString() { for (;;) { if (stopped()) return "" } } };
					f(o, 1); `+tc.after, "run.js")
				done <- err
			}()
			time.Sleep(200 * time.Millisecond)
			iso.TerminateExecution()
			if err := terminatedWithin(t, iso, done, 3*time.Second, func() { stop.Store(true) }); !isTermination(err) {
				t.Errorf("run: got %v, want a termination", err)
			}
			if v, err := ctx.RunScript("'next run'", "next.js"); err != nil || v.String() != "next run" {
				t.Errorf("next run: %v, %v", v, err)
			}
		})
	}
}

// A nested script reaches the heap limit, then the callback reads a property
// before returning the error: the outer script must still report the heap
// limit, rather than catch the error and go on.
func TestNestedHeapLimitSurvivesLaterCallsInCallback(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate(v8.WithResourceConstraints(16<<20, 64<<20))
	defer iso.Dispose()
	var ctx *v8.Context
	var nestedErr error
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("nested", v8.NewFunctionTemplateWithError(iso, func(*v8.FunctionCallbackInfo) (*v8.Value, error) {
		_, nestedErr = ctx.RunScript(heapLimitLoop, "nested.js")
		_, _ = ctx.Global().Get("Object")
		_, _ = v8.JSONParse(ctx, `{"a": 1}`)
		return nil, nestedErr
	})); err != nil {
		t.Fatal(err)
	}
	ctx = v8.NewContext(iso, global)
	defer ctx.Close()

	v, err := ctx.RunScript("var r = 'start'; try { nested() } catch (e) { r = 'caught ' + e } r + ' continued'", "outer.js")
	if !errors.Is(nestedErr, v8.ErrHeapLimitReached) {
		t.Errorf("nested script: got %v, want ErrHeapLimitReached", nestedErr)
	}
	if !errors.Is(err, v8.ErrHeapLimitReached) {
		t.Errorf("outer script: got %v, %v, want ErrHeapLimitReached", v, err)
	}
	iso.Cleanup()
	if v, err := ctx.RunScript("1 + 1", "after.js"); err != nil || v.Int32() != 2 {
		t.Errorf("after Cleanup: %v, %v", v, err)
	}
}

// Callbacks that don't see a termination are unaffected: their results and
// errors reach JavaScript as before.
func TestCallbackWithoutTerminationUnaffected(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("fail", v8.NewFunctionTemplateWithError(iso, func(*v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, fmt.Errorf("boom")
	})); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()
	v, err := ctx.RunScript("let n = 0; for (let i = 0; i < 1000; i++) { try { fail() } catch (e) { n++ } } n", "fail.js")
	if err != nil || v.Int32() != 1000 {
		t.Fatalf("got %v, %v, want 1000", v, err)
	}
}

// osThread runs functions on a goroutine locked to its own OS thread.
type osThread chan func()

func newOSThread(t *testing.T) osThread {
	c := make(osThread)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		for f := range c {
			f()
		}
	}()
	t.Cleanup(func() { close(c) })
	return c
}

func (c osThread) do(f func()) {
	done := make(chan struct{})
	c <- func() { defer close(done); f() }
	<-done
}

// V8 keeps a TerminateExecution request in per-thread state: requested while
// no thread holds the isolate's lock, it was lost if the next script ran on
// another OS thread, and terminated a later script of the first thread
// instead. The request now terminates the next script (or function call),
// whichever thread runs it, and is forgotten afterwards.
func TestTerminateWhileUnlockedAcrossThreads(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()
	a, b := newOSThread(t), newOSThread(t)
	const long = "let s = 0; for (let i = 0; i < 3e8; i++) s += i; 'ran to completion'"

	a.do(func() {
		if _, err := ctx.RunScript("1", "a1.js"); err != nil {
			t.Errorf("a1: %v", err)
		}
	})
	iso.TerminateExecution() // A watchdog firing between two scripts.
	b.do(func() {
		if v, err := ctx.RunScript(long, "b1.js"); !isTermination(err) {
			t.Errorf("b1, the script after the request, on another thread: got %v, %v, want a termination", v, err)
		}
		if v, err := ctx.RunScript("'ok'", "b2.js"); err != nil {
			t.Errorf("b2: %v, %v", v, err)
		}
	})
	a.do(func() {
		if v, err := ctx.RunScript("'next run'", "a2.js"); err != nil {
			t.Errorf("a2, a later script back on the first thread: %v, %v", v, err)
		}
	})

	// Same with a function call on the other thread.
	var fn *v8.Function
	a.do(func() {
		v, err := ctx.RunScript("(function () { "+long+" })", "fn.js")
		if err != nil {
			t.Fatal(err)
		}
		fn, _ = v.AsFunction()
	})
	iso.TerminateExecution()
	b.do(func() {
		if v, err := fn.Call(v8.Undefined(iso)); !isTermination(err) {
			t.Errorf("call after the request, on another thread: got %v, %v, want a termination", v, err)
		}
	})
	a.do(func() {
		if v, err := ctx.RunScript("'next run'", "a3.js"); err != nil {
			t.Errorf("a3: %v, %v", v, err)
		}
	})
}

// A TerminateExecution requested between two scripts on the same thread
// still terminates the next one, as V8 does; Cleanup cancels it.
func TestTerminateBetweenScripts(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()
	th := newOSThread(t)
	th.do(func() {
		if _, err := ctx.RunScript("1", "first.js"); err != nil {
			t.Fatal(err)
		}
		iso.TerminateExecution()
		if _, err := ctx.RunScript("for (;;) {}", "loop.js"); !isTermination(err) {
			t.Errorf("next script: got %v, want a termination", err)
		}
		if v, err := ctx.RunScript("'ok'", "after.js"); err != nil {
			t.Errorf("after: %v, %v", v, err)
		}
		iso.TerminateExecution()
		iso.Cleanup()
		if v, err := ctx.RunScript("'ok'", "cleaned.js"); err != nil {
			t.Errorf("after Cleanup: %v, %v", v, err)
		}
	})
}
