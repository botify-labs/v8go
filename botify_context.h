#ifndef V8GO_BOTIFY_CONTEXT_H
#define V8GO_BOTIFY_CONTEXT_H

// Botify: each Context's m_ctx is also stored in its V8 context's embedder
// data, so that function callbacks find it without a call into Go
// (goContext), which cost as much as the rest of the callback's C++ code.
// NewContext, ContextFree and FunctionTemplateCallback use it, through
// tools/patches/0002-context-pointer.patch.
//
// The V8 context can outlive its Context (JS may still reference its
// functions): ContextFree clears the pointer before freeing the m_ctx, and
// callbacks then fall back to goContext, as before.

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-locker.h"
#include "deps/include/v8-object.h"

#include "context.h"

// After ContextDataIndex::REF; slot 0 has a special meaning for Chrome's
// debugger.
constexpr int kBotifyContextIndex = 2;

// Stores ctx in local_ctx, which NewContext just created.
inline void BotifyContextSet(v8::Local<v8::Context> local_ctx, m_ctx* ctx) {
  local_ctx->SetAlignedPointerInEmbedderData(kBotifyContextIndex, ctx,
                                             v8::kEmbedderDataTypeTagDefault);
}

// Returns the m_ctx of local_ctx, a context created by NewContext, or nullptr
// once ContextFree freed it.
inline m_ctx* BotifyContextGet(v8::Isolate* iso,
                               v8::Local<v8::Context> local_ctx) {
  return static_cast<m_ctx*>(local_ctx->GetAlignedPointerFromEmbedderData(
      iso, kBotifyContextIndex, v8::kEmbedderDataTypeTagDefault));
}

// Clears the pointer BotifyContextSet stored, before ContextFree frees ctx.
// The Isolate's internal context isn't created by NewContext, and has none.
inline void BotifyContextForget(m_ctx* ctx) {
  v8::Isolate* iso = ctx->iso;
  if (ctx->ptr.IsEmpty() || iso->GetData(0) == ctx) {
    return;
  }
  v8::Locker locker(iso);
  v8::Isolate::Scope isolate_scope(iso);
  v8::HandleScope handle_scope(iso);
  ctx->ptr.Get(iso)->SetAlignedPointerInEmbedderData(
      kBotifyContextIndex, nullptr, v8::kEmbedderDataTypeTagDefault);
}

#endif
