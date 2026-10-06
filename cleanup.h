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

extern void ContextCleanup(ContextPtr ctx);
extern void IsolateCleanup(IsolatePtr iso);
extern int IsolateInternalRetainedValueCount(IsolatePtr iso);
extern int IsolateInternalUnboundScriptCount(IsolatePtr iso);

#ifdef __cplusplus
}  // extern "C"
#endif

#endif
