package v8go

// #include "cleanup.h"
import "C"

// Cleanup releases the values and unbound scripts tracked by the Context.
// It avoids leaking memory when the Context is long-lived and the scripts it
// runs are stateless. Any *Value, *Object or *UnboundScript obtained from this
// Context before the call must not be used afterwards.
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
// the call must not be used afterwards.
//
// Cleanup first runs the tasks V8 posted for the Isolate (GC tasks such as the
// memory reducer, FinalizationRegistry callbacks), which nothing else in v8go
// runs: call it regularly on a long-lived Isolate. As a consequence:
//   - JavaScript, and the Go callbacks it calls (e.g. from a
//     FinalizationRegistry callback), may run during Cleanup;
//   - Cleanup must not be called from inside a FunctionCallback, or while
//     JavaScript is otherwise on the stack;
//   - Undefined and Null must be fetched again after Cleanup, rather than
//     reused from before the call;
//   - a TerminateExecution (e.g. from a watchdog) or the heap-limit callback
//     may take effect while those tasks run, and then affect the next
//     RunScript or Run.
func (i *Isolate) Cleanup() {
	if i.ptr == nil {
		return
	}
	C.IsolateCleanup(i.ptr)
	i.null = newValueNull(i)
	i.undefined = newValueUndefined(i)
}

func (i *Isolate) internalRetainedValueCount() int {
	return int(C.IsolateInternalRetainedValueCount(i.ptr))
}

func (i *Isolate) internalUnboundScriptCount() int {
	return int(C.IsolateInternalUnboundScriptCount(i.ptr))
}
