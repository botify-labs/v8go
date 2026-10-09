// clang-format off
//go:build v8go_source
// clang-format on

#include "_cgo_export.h"

#include "botify_context.h"
#include "botify_termination.h"
#include "deps/include/v8-context.h"
#include "deps/include/v8-function.h"
#include "isolate-macros.h"
#include "template-macros.h"
#include "template.h"

using namespace v8;

void FunctionTemplateCallback(const FunctionCallbackInfo<Value>& info) {
  Isolate* iso = info.GetIsolate();
  // Botify: V8 calls this from JS, so iso is already locked and entered by
  // this thread: a Locker and an Isolate::Scope would only cost time.
  HandleScope handle_scope(iso);

  // Botify: Isolate.Cleanup runs no JS or Go code (botify_context.h).
  if (BotifyInCleanup(iso)) {
    return;
  }

  // This callback function can be called from any Context, which we only know
  // at runtime. We extract the Context reference from the embedder data so that
  // we can use the context registry to match the Context on the Go side
  Local<Context> local_ctx = iso->GetCurrentContext();
  int ctx_ref = local_ctx->GetEmbedderDataV2(ContextDataIndex::REF)
                    .As<Integer>()
                    ->Value();
  // Botify: found without a call into Go, unless the Context is closed.
  m_ctx* ctx = BotifyContextGet(iso, local_ctx);
  if (ctx == nullptr) {
    ctx = goContext(ctx_ref);
  }
  // Botify: the function's Context is closed, but JS still holds the function
  // (e.g. another Context it was passed to). There is no Go Context to call
  // the callback with.
  if (ctx == nullptr) {
    iso->ThrowError("v8go: context closed");
    return;
  }

  int callback_ref = info.Data().As<Integer>()->Value();

  m_value* _this = new m_value;
  _this->id = 0;
  _this->iso = iso;
  _this->ctx = ctx;
  _this->ptr.Reset(iso, info.This());

  // Botify: calls with few arguments need no heap allocation.
  size_t count = 1 + info.Length();
  ValuePtr stackArgs[8];
  std::vector<ValuePtr> heapArgs;
  ValuePtr* thisAndArgs = stackArgs;
  if (count > std::size(stackArgs)) {
    heapArgs.resize(count);
    thisAndArgs = heapArgs.data();
  }
  thisAndArgs[0] = tracked_value(ctx, _this);
  for (size_t i = 1; i < count; ++i) {
    m_value* val = new m_value;
    val->id = 0;
    val->iso = iso;
    val->ctx = ctx;
    val->ptr.Reset(iso, info[i - 1]);
    thisAndArgs[i] = tracked_value(ctx, val);
  }

  goFunctionCallback_return retval =
      goFunctionCallback(ctx_ref, callback_ref, thisAndArgs, count - 1);
  // Botify: a script the callback ran was terminated, and the termination
  // goes on through the caller's frames. The callback's error is that
  // termination's: thrown, it would replace the termination with an exception
  // the caller's JS could catch, and go on running.
  if (iso->IsExecutionTerminating()) {
    return;
  }
  // Botify: a termination is owed to the run, but not pending: a V8 call of
  // the callback cleared it (botify_termination.h). It must still terminate
  // the JavaScript that called the callback.
  if (BotifyTerminationOwed(iso)) {
    BotifyTerminateNow(iso);
    return;
  }
  if (retval.r1 != nullptr) {
    iso->ThrowException(retval.r1->ptr.Get(iso));
  } else if (retval.r0 != nullptr) {
    info.GetReturnValue().Set(retval.r0->ptr.Get(iso));
  } else {
    info.GetReturnValue().SetUndefined();
  }
}

TemplatePtr NewFunctionTemplate(IsolatePtr iso, int callback_ref) {
  Locker locker(iso);
  Isolate::Scope isolate_scope(iso);
  HandleScope handle_scope(iso);

  // (rogchap) We only need to store one value, callback_ref, into the
  // C++ callback function data, but if we needed to store more items we could
  // use an V8::Array; this would require the internal context from
  // iso->GetData(0)
  Local<Integer> cbData = Integer::New(iso, callback_ref);

  m_template* ot = new m_template;
  ot->iso = iso;
  ot->ptr.Reset(iso,
                FunctionTemplate::New(iso, FunctionTemplateCallback, cbData));
  return ot;
}

RtnValue FunctionTemplateGetFunction(m_template* ptr, m_ctx* ctx) {
  LOCAL_TEMPLATE(ptr);
  TryCatch try_catch(iso);
  Local<Context> local_ctx = ctx->ptr.Get(iso);
  Context::Scope context_scope(local_ctx);

  Local<FunctionTemplate> fn_tmpl = tmpl.As<FunctionTemplate>();
  RtnValue rtn = {};
  Local<Function> fn;
  if (!fn_tmpl->GetFunction(local_ctx).ToLocal(&fn)) {
    rtn.error = ExceptionError(try_catch, iso, local_ctx);
    return rtn;
  }

  m_value* val = new m_value;
  val->id = 0;
  val->iso = iso;
  val->ctx = ctx;
  val->ptr = Global<Value>(iso, fn);
  rtn.value = tracked_value(ctx, val);
  return rtn;
}

m_template* FunctionTemplateInstanceTemplate(m_template* ptr) {
  LOCAL_TEMPLATE(ptr);
  Local<FunctionTemplate> fn_tmpl = tmpl.As<FunctionTemplate>();
  m_template* ot = new m_template;
  ot->iso = iso;
  ot->ptr.Reset(iso, fn_tmpl->InstanceTemplate());

  return ot;
}

m_template* FunctionTemplatePrototypeTemplate(m_template* ptr) {
  LOCAL_TEMPLATE(ptr);
  Local<FunctionTemplate> fn_tmpl = tmpl.As<FunctionTemplate>();
  m_template* ot = new m_template;
  ot->iso = iso;
  ot->ptr.Reset(iso, fn_tmpl->PrototypeTemplate());

  return ot;
}

void FunctionTemplateInherit(TemplatePtr ptr, TemplatePtr base) {
  LOCAL_TEMPLATE(ptr);
  Local<FunctionTemplate> fn_tmpl = tmpl.As<FunctionTemplate>();
  Local<FunctionTemplate> base_tmp = base->ptr.Get(iso).As<FunctionTemplate>();
  fn_tmpl->Inherit(base_tmp);
}
