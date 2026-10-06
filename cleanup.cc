//go:build v8go_source

#include "cleanup.h"

#include "deps/include/v8-locker.h"

#include "context.h"
#include "isolate-macros.h"
#include "unbound_script.h"
#include "value.h"

using namespace v8;

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
}

int IsolateInternalRetainedValueCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->vals.size();
}

int IsolateInternalUnboundScriptCount(IsolatePtr iso) {
  return isolateInternalContext(iso)->unboundScripts.size();
}
