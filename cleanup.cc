// clang-format off
//go:build v8go_source
// clang-format on

#include "cleanup.h"

#include <memory>

#include "deps/include/libplatform/libplatform.h"
#include "deps/include/v8-locker.h"

#include "botify_context.h"
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

constinit thread_local Isolate* botify_cleanup_isolate = nullptr;

// Upper bound on the tasks one Cleanup runs. Some V8 tasks re-arm themselves
// (e.g. GC jobs posting their next step), so draining until the queue is
// empty could keep Cleanup busy for a long time. Whatever is left runs at the
// next Cleanup.
static const int kMaxPendingTasksPerCleanup = 10000;

// Whether JavaScript of iso is on this thread's stack, e.g. when Cleanup is
// called from a FunctionCallback: V8 runs JavaScript, and the callbacks it
// calls, in a context, while v8go's C++ functions only enter one for the
// duration of the call. The caller holds the isolate's Locker.
static bool InJavaScript(Isolate* iso) {
  return iso->InContext();
}

// V8 posts work for an isolate to the platform's foreground task queue: GC
// tasks (memory reducer, GC jobs, ...) and FinalizationRegistry cleanups.
// v8go never runs that queue, so the tasks piled up in native memory, and
// without the memory reducer V8 only ran a major GC at its initial old-space
// limit. Runs the pending tasks, the expired delayed ones, and the ones they
// post, without waiting.
//
// No JavaScript runs (nor Go: see botify_cleanup_isolate). The page is done
// when Cleanup is called, and nothing would stop a callback that never
// returns. A termination is requested before each task, so that the
// JavaScript a task calls is terminated at its first instruction: the only
// such tasks are FinalizationRegistry cleanups, and V8 15.4's
// (FinalizationRegistryCleanupTask, JSFinalizationRegistry::Cleanup) pops a
// cell before it calls the callback, stops at the exception, and posts itself
// again while cells remain: each run makes progress, so the pump ends. The
// task's TryCatch, at call depth zero, clears the termination, hence one
// request per task. GC tasks (memory reducer, GC jobs) run no JavaScript and
// don't check for termination: they run as before.
//
// The termination is cancelled afterwards, with any other one requested in
// the meantime (e.g. by a watchdog firing during the pump): the next script
// starts with none.
static void RunPendingTasks(Isolate* iso) {
  // Function callbacks a task calls don't call Go (botify_context.h).
  botify_cleanup_isolate = iso;
  for (int i = 0; i < kMaxPendingTasksPerCleanup; i++) {
    // A task consumes the request when it runs JavaScript: renew it.
    iso->TerminateExecution();
    if (!platform::PumpMessageLoop(default_platform.get(), iso)) {
      break;
    }
  }
  iso->CancelTerminateExecution();
  botify_cleanup_isolate = nullptr;
}

void ContextCleanup(ContextPtr ctx) {
  if (ctx == nullptr) {
    return;
  }
  Locker locker(ctx->iso);
  Isolate::Scope isolate_scope(ctx->iso);
  if (InJavaScript(ctx->iso)) {
    return;
  }

  // Weak owners of Go values (go_handle != 0, see NewValueGo) stay tracked:
  // JS may still reference their External, and GoValueWeakCallback releases
  // them, and their cgo.Handle, when V8 collects it.
  ctx->vals.release_if([](m_value* val) { return val->go_handle != 0; },
                       [](m_value* val) {
                         val->ptr.Reset();
                         delete val;
                       });

  for (m_unboundScript* us : ctx->unboundScripts) {
    us->ptr.Reset();
    delete us;
  }
  ctx->unboundScripts.clear();
}

int IsolateCleanup(IsolatePtr iso) {
  if (iso == nullptr) {
    return 0;
  }
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  if (InJavaScript(iso)) {
    return 0;
  }
  RunPendingTasks(iso);
  ContextCleanup(isolateInternalContext(iso));
  return 1;
}

int IsolateInternalRetainedValueCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->vals.size();
}

int IsolateInternalUnboundScriptCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->unboundScripts.size();
}
