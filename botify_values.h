#ifndef V8GO_BOTIFY_VALUES_H
#define V8GO_BOTIFY_VALUES_H

// Botify: the set of values a context tracks (m_ctx::vals), in place of
// tommie's std::unordered_map<long, m_value*>, applied by
// tools/patches/0001-value-tracker.patch.
//
// A tracked value's id is its index in the set plus one (0 means untracked),
// so tracking and releasing a value are O(1) without a node allocation or
// hashing per value. Releasing a value moves the last value into its slot, and
// updates that value's id: ids are not stable, and nothing else must keep them.

#include <stddef.h>
#include <stdint.h>
#include <string.h>

#include <algorithm>
#include <utility>
#include <vector>

#include "value.h"

class ValueTracker {
 public:
  using const_iterator = std::vector<m_value*>::const_iterator;

  // Tracks val, which must not be tracked yet (id 0).
  void add(m_value* val) {
    vals_.push_back(val);
    val->id = static_cast<long>(vals_.size());
  }

  // Stops tracking the value with the given id. Like unordered_map::erase, an
  // id that tracks nothing (0, for an untracked value) is ignored.
  void erase(long id) {
    if (id <= 0 || static_cast<size_t>(id) > vals_.size()) {
      return;
    }
    m_value* last = vals_.back();
    vals_[id - 1] = last;
    last->id = id;
    vals_.pop_back();
  }

  // Releases the values for which keep(val) is false, with release(val),
  // which may delete them. The values kept get new ids, in their current
  // order.
  //
  // V8 reuses freed Global handles last-freed first. Released in any other
  // order than by address, the order in which the next values get them would
  // get shuffled at each Cleanup, until creating a value costs a cache miss
  // on its handle, once they don't fit in the CPU caches. Many values are
  // therefore released by decreasing handle address, so that the next ones
  // get their handles in increasing address order.
  template <typename Keep, typename Release>
  void release_if(Keep keep, Release release) {
    // The values kept move to the front, in one pass: slot kept is never
    // after the one being read.
    size_t kept = 0;
    if (vals_.size() <= kSortReleased) {
      for (m_value* val : vals_) {
        if (keep(val)) {
          vals_[kept++] = val;
          val->id = static_cast<long>(kept);
        } else {
          release(val);
        }
      }
    } else {
      std::vector<std::pair<uintptr_t, m_value*>> byAddress;
      byAddress.reserve(vals_.size());
      for (m_value* val : vals_) {
        if (keep(val)) {
          vals_[kept++] = val;
          val->id = static_cast<long>(kept);
        } else {
          byAddress.emplace_back(handle_address(val), val);
        }
      }
      std::sort(byAddress.begin(), byAddress.end(),
                [](const auto& a, const auto& b) { return a.first > b.first; });
      for (auto& [addr, val] : byAddress) {
        release(val);
      }
    }
    vals_.resize(kept);
  }

  size_t size() const { return vals_.size(); }
  const_iterator begin() const { return vals_.begin(); }
  const_iterator end() const { return vals_.end(); }
  void clear() { vals_.clear(); }

 private:
  // Cleanups of up to that many values don't sort them: their handles stay
  // in the CPU caches (32 bytes each).
  static constexpr size_t kSortReleased = 16384;

  // The address of val's Global handle.
  static uintptr_t handle_address(m_value* val) {
    static_assert(sizeof(val->ptr) == sizeof(uintptr_t));
    uintptr_t addr;
    memcpy(&addr, &val->ptr, sizeof(addr));
    return addr;
  }

  std::vector<m_value*> vals_;
};

#endif
