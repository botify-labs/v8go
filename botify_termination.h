#ifndef V8GO_BOTIFY_TERMINATION_H
#define V8GO_BOTIFY_TERMINATION_H

// Botify: a TerminateExecution request terminates the whole run it was made
// for, i.e. the top-level script or function call in progress (or the next
// one, when none is), as with V8 9.0. tools/patches/0008-termination.patch
// calls this from tommie's files.
//
// V8 15.4 loses a request in two cases:
//
// - In a Go callback. V8 clears a pending exception, the termination
//   included, on entry to most API calls (PrepareForExecutionScope in
//   api-inl.h; V8 9.0 refused the call instead). After a nested script was
//   terminated, or a JavaScript toString a conversion called, any later V8
//   call of the callback (Object.Get, JSONParse, Value.String of a number...)
//   cancelled the termination: the outer script went on running, and so did
//   any JavaScript the callback ran afterwards.
//
// - Across threads. V8 keeps a request in per-thread state, which a Locker
//   taken by another OS thread archives: requested while no thread holds the
//   lock, it was lost for a script run by another thread, and terminated a
//   later script of the first thread instead.
//
// Every request is counted (BotifyRequestTermination: Isolate.TerminateExecution
// and the near-heap-limit callback), and the count is recorded at the end of
// each top-level run, and by Isolate.Cleanup, which cancels the requests made
// until it ends (BotifyTerminationHandled). A count that differs from the
// recorded one means a termination is owed to the run in progress
// (BotifyTerminationOwed). Then:
//
// - the entry points that run scripts (BotifyRunScope: RunScript,
//   UnboundScript.Run, Function.Call, Function.NewInstance,
//   PerformMicrotaskCheckpoint) request it again, on this thread, at the
//   start of a top-level run; they cancel any request V8 still holds for this
//   thread otherwise (a stale one);
// - every v8go entry point called by a Go callback (BotifyEntry, in the
//   LOCAL_* and ISOLATE_SCOPE macros) requests it again before its V8 call
//   clears it, so that the JavaScript that call runs is terminated at its
//   first instruction; Promise.Then and Catch, which panic on an error, hold
//   the request back during their call (BotifyHoldTermination);
// - FunctionTemplateCallback makes it effective again before returning to
//   JavaScript, if no longer pending (BotifyTerminateNow).

#include <atomic>
#include <cstdint>

#include "deps/include/v8-context.h"
#include "deps/include/v8-isolate.h"
#include "deps/include/v8-local-handle.h"
#include "deps/include/v8-primitive.h"
#include "deps/include/v8-script.h"

// Isolate data slot; 0 is the internal context, 1 tommie's IsolateState.
constexpr uint32_t kBotifyTerminationSlot = 2;

struct BotifyTermination {
  // Number of termination requests. Incremented without the isolate's lock
  // (e.g. by a watchdog goroutine).
  std::atomic<uint32_t> requests{0};
  // requests as of the end of the last top-level run, or of Cleanup. Only
  // used with the isolate's lock held.
  uint32_t handled = 0;
  // Number of runs in progress (BotifyRunScope): more than one when a Go
  // callback of the top-level run runs a script. Lock held.
  int runs = 0;
};

inline BotifyTermination* BotifyTerminationOf(v8::Isolate* iso) {
  return static_cast<BotifyTermination*>(iso->GetData(kBotifyTerminationSlot));
}

// NewIsolate; IsolateDispose deletes it.
inline void BotifyTerminationInit(v8::Isolate* iso) {
  iso->SetData(kBotifyTerminationSlot, new BotifyTermination);
}

// Requests the termination of the running (or next) run; callable without
// the isolate's lock. Counted first: BotifyRunScope reads the count after it
// cancels a stale request, so a request it may cancel is always seen.
inline void BotifyRequestTermination(v8::Isolate* iso) {
  BotifyTerminationOf(iso)->requests.fetch_add(1, std::memory_order_seq_cst);
  iso->TerminateExecution();
}

// Whether a termination was requested since the last top-level run ended (or
// Cleanup): during a run, the run must be terminated. Called with the
// isolate's lock held.
inline bool BotifyTerminationOwed(v8::Isolate* iso) {
  BotifyTermination* t = BotifyTerminationOf(iso);
  return t->requests.load(std::memory_order_seq_cst) != t->handled;
}

// The requests made so far are done with: the run they were meant for has
// ended, or Cleanup cancelled them. Called with the isolate's lock held.
inline void BotifyTerminationHandled(v8::Isolate* iso) {
  BotifyTermination* t = BotifyTerminationOf(iso);
  t->handled = t->requests.load(std::memory_order_seq_cst);
}

// Constructed by v8go's entry points once the isolate is locked and entered,
// before they enter a context. With a termination owed and JavaScript of the
// isolate on the stack (the entry point is called by a Go callback), the
// termination is requested again: the V8 call to come would clear a pending
// one. Otherwise (no termination owed, the common case, checked first; or a
// top-level call), nothing changes.
class BotifyEntry {
 public:
  explicit BotifyEntry(v8::Isolate* iso) : iso_(iso), owed_(false) {
    if (BotifyTerminationOwed(iso) && iso->InContext()) {
      owed_ = true;
      iso->TerminateExecution();
    }
  }
  BotifyEntry(const BotifyEntry&) = delete;
  BotifyEntry& operator=(const BotifyEntry&) = delete;

  v8::Isolate* isolate() const { return iso_; }
  bool owed() const { return owed_; }

 private:
  v8::Isolate* iso_;
  bool owed_;
};

// For the entry points whose Go caller panics on an error (Promise.Then and
// Catch): their V8 call runs builtin JavaScript, which an owed termination
// would make fail. The request is withdrawn for the duration of the call, and
// made again afterwards.
class BotifyHoldTermination {
 public:
  explicit BotifyHoldTermination(const BotifyEntry& entry)
      : iso_(entry.owed() ? entry.isolate() : nullptr) {
    if (iso_ != nullptr) {
      iso_->CancelTerminateExecution();
    }
  }
  ~BotifyHoldTermination() {
    if (iso_ != nullptr) {
      iso_->TerminateExecution();
    }
  }
  BotifyHoldTermination(const BotifyHoldTermination&) = delete;
  BotifyHoldTermination& operator=(const BotifyHoldTermination&) = delete;

 private:
  v8::Isolate* iso_;
};

// Wraps a run of an entry point that runs JavaScript. A top-level run (no
// other run in progress): a request made since the previous run ended is
// meant for this run, and requested again on this thread, whichever thread
// received it. Otherwise, any request V8 still holds for this thread is stale
// (made during an earlier run that ended before V8 acted on it, or archived by
// V8 with this thread's state while another thread ran scripts): cancelled.
// Nested runs, from a Go callback, are part of the top-level run, and left to
// BotifyEntry. (A script run by a Go callback of a top-level call that isn't
// a run, e.g. Value.String calling a JavaScript toString, counts as a
// top-level run.)
class BotifyRunScope {
 public:
  explicit BotifyRunScope(const BotifyEntry& entry)
      : iso_(entry.isolate()),
        top_level_(BotifyTerminationOf(iso_)->runs++ == 0) {
    if (!top_level_) {
      return;
    }
    BotifyTermination* t = BotifyTerminationOf(iso_);
    if (t->requests.load(std::memory_order_seq_cst) == t->handled) {
      // Never requested: nothing can be stale.
      if (t->handled == 0) {
        return;
      }
      iso_->CancelTerminateExecution();
      // A request counted since the first check may have been cancelled.
      if (t->requests.load(std::memory_order_seq_cst) == t->handled) {
        return;
      }
    }
    iso_->TerminateExecution();
  }
  ~BotifyRunScope() {
    if (top_level_) {
      BotifyTerminationHandled(iso_);
    }
    BotifyTerminationOf(iso_)->runs--;
  }
  BotifyRunScope(const BotifyRunScope&) = delete;
  BotifyRunScope& operator=(const BotifyRunScope&) = delete;

 private:
  v8::Isolate* iso_;
  bool top_level_;
};

// Called by FunctionTemplateCallback, in JavaScript, when a termination is
// owed but not pending as the Go callback returns (a V8 call of the callback
// cleared it, or V8 hadn't acted on the request yet): makes it pending, so
// that the JavaScript that called the callback unwinds as soon as it returns,
// rather than at its next interrupt check, which may never come (a script
// ending right after the call would return its result). V8 only acts on a
// request in JavaScript: a trivial script is run, which the request
// terminates on entry.
inline void BotifyTerminateNow(v8::Isolate* iso) {
  iso->TerminateExecution();
  v8::Local<v8::Context> ctx = iso->GetCurrentContext();
  v8::Local<v8::String> src =
      v8::String::NewFromUtf8Literal(iso, "for (let i = 0; i < 2; i++) {}");
  v8::Local<v8::Script> script;
  if (v8::Script::Compile(ctx, src).ToLocal(&script)) {
    (void)script->Run(ctx);
  }
}

#endif
