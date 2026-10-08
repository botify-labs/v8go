package v8go

// #include "cleanup.h"
import "C"

// Cleanup releases the values and unbound scripts tracked by the Context.
// It avoids leaking memory when the Context is long-lived and the scripts it
// runs are stateless. Any *Value, *Object or *UnboundScript obtained from this
// Context before the call must not be used afterwards.
//
// Cleanup does nothing when called while JavaScript runs on the Isolate, e.g.
// from a FunctionCallback: it would release the values of the calls in
// progress (their receiver, arguments and results). Call it once the script
// has returned.
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
// memory reducer), which nothing else in v8go runs: call it regularly on a
// long-lived Isolate. It never runs JavaScript, nor Go callbacks: the
// JavaScript a task would run is terminated before its first instruction, so
// FinalizationRegistry callbacks never run (as in the V8 9.0 fork, which ran
// no task). A TerminateExecution requested while Cleanup runs is cancelled:
// the next script runs normally.
//
// Like Context.Cleanup, it does nothing when called while JavaScript runs on
// the Isolate, e.g. from a FunctionCallback.
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

func (i *Isolate) internalRetainedValueCount() int {
	return int(C.IsolateInternalRetainedValueCount(i.ptr))
}

func (i *Isolate) internalUnboundScriptCount() int {
	return int(C.IsolateInternalUnboundScriptCount(i.ptr))
}
