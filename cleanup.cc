//go:build v8go_source

#include "cleanup.h"

#include <memory>

#include "deps/include/libplatform/libplatform.h"
#include "deps/include/v8-locker.h"

#include "context.h"
#include "isolate-macros.h"
#include "unbound_script.h"
#include "value.h"

using namespace v8;

// The platform v8go initializes V8 with, defined in isolate.cc.
extern std::unique_ptr<Platform> default_platform;

// V8 posts work for an isolate to the platform's foreground task queue: GC
// tasks (memory reducer, GC jobs, ...) and FinalizationRegistry cleanups.
// v8go never runs that queue, so the tasks piled up in native memory, and
// without the memory reducer V8 only ran a major GC at its initial old-space
// limit. Runs the pending tasks, the expired delayed ones, and the ones they
// post, without waiting.
static void RunPendingTasks(Isolate* iso) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  while (platform::PumpMessageLoop(default_platform.get(), iso)) {
  }
}

void ContextCleanup(ContextPtr ctx) {
  if (ctx == nullptr) {
    return;
  }
  Locker locker(ctx->iso);

  // Weak owners of Go values (go_handle != 0, see NewValueGo) stay tracked:
  // JS may still reference their External, and GoValueWeakCallback releases
  // them, and their cgo.Handle, when V8 collects it.
  for (auto it = ctx->vals.begin(); it != ctx->vals.end();) {
    m_value* val = it->second;
    if (val->go_handle != 0) {
      ++it;
      continue;
    }
    val->ptr.Reset();
    delete val;
    it = ctx->vals.erase(it);
  }

  for (m_unboundScript* us : ctx->unboundScripts) {
    us->ptr.Reset();
    delete us;
  }
  ctx->unboundScripts.clear();
}

void IsolateCleanup(IsolatePtr iso) {
  if (iso == nullptr) {
    return;
  }
  ContextCleanup(isolateInternalContext(iso));
  RunPendingTasks(iso);
}

int IsolateInternalRetainedValueCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->vals.size();
}

int IsolateInternalUnboundScriptCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->unboundScripts.size();
}
