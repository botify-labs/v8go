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
