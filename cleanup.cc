// clang-format off
//go:build v8go_source
// clang-format on

#include "cleanup.h"

#include <memory>

#include "deps/include/libplatform/libplatform.h"
#include "deps/include/v8-locker.h"

#include "context.h"
#include "isolate-macros.h"
#include "unbound_script.h"
#include "value.h"

using namespace v8;

// The platform v8go initializes V8 with, defined in tommie's isolate.cc as
// `auto default_platform = platform::NewDefaultPlatform();`. This declaration
// must match that definition's type: if a sync renames it, linking fails, but
// if it changes its type, nothing catches it (undefined behaviour), hence the
// exact-line check in tools/check_bridge.sh.
extern std::unique_ptr<Platform> default_platform;

// Upper bound on the tasks one Cleanup runs. Some V8 tasks re-arm themselves
// (e.g. GC jobs posting their next step), so draining until the queue is
// empty could keep Cleanup busy for a long time. Whatever is left runs at the
// next Cleanup.
static const int kMaxPendingTasksPerCleanup = 10000;

// V8 posts work for an isolate to the platform's foreground task queue: GC
// tasks (memory reducer, GC jobs, ...) and FinalizationRegistry cleanups.
// v8go never runs that queue, so the tasks piled up in native memory, and
// without the memory reducer V8 only ran a major GC at its initial old-space
// limit. Runs the pending tasks, the expired delayed ones, and the ones they
// post, without waiting. Tasks may run JS, and Go callbacks through it.
static void RunPendingTasks(Isolate* iso) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  for (int i = 0; i < kMaxPendingTasksPerCleanup &&
                  platform::PumpMessageLoop(default_platform.get(), iso);
       i++) {
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
  // Tasks first: the JS they run may call Go callbacks that return values
  // tracked by the internal context, e.g. the Isolate's cached Undefined and
  // Null, which ContextCleanup releases. Values those callbacks create are
  // then released by this Cleanup too.
  RunPendingTasks(iso);
  ContextCleanup(isolateInternalContext(iso));
}

int IsolateInternalRetainedValueCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->vals.size();
}

int IsolateInternalUnboundScriptCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->unboundScripts.size();
}
