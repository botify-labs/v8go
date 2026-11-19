#ifndef V8GO_CLEANUP_H
#define V8GO_CLEANUP_H

// Botify: releases the values and unbound scripts tracked by long-lived
// contexts, so an Isolate/Context can be reused across many stateless scripts.

#ifdef __cplusplus
namespace v8 {
class Isolate;
}
typedef v8::Isolate v8Isolate;

extern "C" {
#else
typedef struct v8Isolate v8Isolate;
#endif

typedef v8Isolate* IsolatePtr;

typedef struct m_ctx m_ctx;
typedef m_ctx* ContextPtr;

// Both do nothing when called with JavaScript of the isolate on the stack
// (from a FunctionCallback): IsolateCleanup then returns 0, otherwise 1.
extern void ContextCleanup(ContextPtr ctx);
extern int IsolateCleanup(IsolatePtr iso);
extern int IsolateInternalRetainedValueCount(IsolatePtr iso);

// Defined in isolate.cc (tools/patches/0007-heap-limit.patch), next to the
// heap limit state they read or clear.
// Whether the heap limit was reached since the isolate was created or since
// IsolateResetHeapLimitReached; callable without the isolate's lock.
extern int IsolateHeapLimitRaised(IsolatePtr iso);
// Whether a heap limit termination is still to be reported (see
// IsolateTakeHeapLimitReached), without clearing it.
extern int IsolatePeekHeapLimitReached(IsolatePtr iso);
// Isolate.Cleanup forgets both: the termination to report
// (IsolateResetHeapLimitReached) after its pump, and IsolateHeapLimitRaised
// (IsolateResetHeapLimitRaised) before it, so that a limit reached by a GC
// of the pump is still visible.
extern void IsolateResetHeapLimitReached(IsolatePtr iso);
extern void IsolateResetHeapLimitRaised(IsolatePtr iso);
extern int IsolateInternalUnboundScriptCount(IsolatePtr iso);

typedef struct m_unboundScript m_unboundScript;
typedef m_unboundScript* UnboundScriptPtr;

// UnboundScriptRetain makes ContextCleanup keep us, a script of the
// isolate's internal context (IsolateCompileUnboundScript), until
// UnboundScriptRelease or the isolate's disposal. UnboundScriptRelease frees
// a script so kept and returns 1, and does nothing on any other one (0).
extern void UnboundScriptRetain(IsolatePtr iso, UnboundScriptPtr us);
extern int UnboundScriptRelease(IsolatePtr iso, UnboundScriptPtr us);

#ifdef __cplusplus
}  // extern "C"
#endif

#endif
