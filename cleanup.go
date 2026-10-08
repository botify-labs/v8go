package v8go

// #include "cleanup.h"
import "C"

// Cleanup releases the values and unbound scripts tracked by the Context.
// It avoids leaking memory when the Context is long-lived and the scripts it
// runs are stateless. Any *Value, *Object or *UnboundScript obtained from this
// Context before the call must not be used afterwards.
//
// Cleanup does nothing when called while JavaScript runs on the Isolate on
// the calling thread, e.g. from a FunctionCallback: it would release the
// values of the calls in progress (their receiver, arguments and results).
// Call it once the script has returned. Called from another goroutine while a
// script runs, it waits for the Isolate's lock, i.e. until the script ends,
// then runs: never call it from a goroutine a Go callback waits for (deadlock).
func (c *Context) Cleanup() {
	if c.ptr == nil {
		return
	}
	C.ContextCleanup(c.ptr)
}

// Cleanup releases the values and unbound scripts tracked by the Isolate's
// internal context, i.e. those created by NewValue, function callbacks and
// CompileUnboundScript. Go values still referenced from JavaScript are kept
// until V8 collects them. Any such *Value or *UnboundScript obtained before
// the call must not be used afterwards; Undefined and Null must be fetched
// again rather than reused from before the call.
//
// Cleanup first runs the tasks V8 posted for the Isolate (GC tasks such as the
// memory reducer, and the tasks that settle promises, e.g. wasm compilations
// and Atomics.waitAsync timeouts), which nothing else in v8go runs: call it
// regularly on a long-lived Isolate. It never runs JavaScript, nor Go
// callbacks (function callbacks, the PromiseRejectedCallback, an Inspector's
// console messages): the JavaScript a task would run is terminated before its
// first instruction, so FinalizationRegistry callbacks never run (as in the V8
// 9.0 fork, which ran no task). The promise reactions still pending (those
// queued by the tasks, or by Go since the last script) are dropped rather
// than left to run at the end of the next script. A TerminateExecution
// requested while Cleanup runs is cancelled, including one requested by a
// watchdog on another goroutine for a script waiting for the Isolate's lock:
// that script then runs to completion. Cleanup also clears the heap limit
// state (HeapLimitReached).
//
// Cleanup needs exclusive use of the Isolate: Undefined and Null are
// replaced without synchronization, and the values other goroutines hold from
// before the call are freed. Don't run scripts or use values of the Isolate
// on other goroutines meanwhile.
//
// Like Context.Cleanup, it does nothing when called while JavaScript runs on
// the Isolate on the calling thread, e.g. from a FunctionCallback, and waits
// for the script to end when called from another goroutine.
func (i *Isolate) Cleanup() {
	if i.ptr == nil {
		return
	}
	if C.IsolateCleanup(i.ptr) == 0 {
		return
	}
	i.null = newValueNull(i)
	i.undefined = newValueUndefined(i)
}

// HeapLimitReached reports whether the Isolate reached its heap limit since
// it was created or since the last Cleanup. v8go then terminates the running
// JavaScript and raises the limit for the time it unwinds (see
// WithResourceConstraints). A script so terminated returns an error matching
// ErrHeapLimitReached, but the limit can also be reached with no error to
// report it: in a promise reaction run after the script's result, while
// compiling, or in a garbage collection. Checked before Cleanup, it tells
// whether the page reached the limit, e.g. to dispose of the Isolate rather
// than reuse it. It can be called from any goroutine.
func (i *Isolate) HeapLimitReached() bool {
	if i.ptr == nil {
		return false
	}
	return C.IsolateHeapLimitRaised(i.ptr) != 0
}

func (i *Isolate) internalRetainedValueCount() int {
	return int(C.IsolateInternalRetainedValueCount(i.ptr))
}

func (i *Isolate) internalUnboundScriptCount() int {
	return int(C.IsolateInternalUnboundScriptCount(i.ptr))
}
