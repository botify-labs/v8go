//go:build linux && cxxprobe

// Compiled by the consumer's g++ against libstdc++: every C++ runtime entry
// point used here (__cxa_throw, __gxx_personality_v0, typeinfo for
// std::exception, __dynamic_cast, operator new, std::ios_base::Init...) must
// bind to libstdc++, never to V8's libc++abi.

#include "cxxprobe.h"

#include <cstdlib>
#include <cstring>
#include <exception>
#include <iostream>
#include <limits>
#include <memory>
#include <new>
#include <sstream>
#include <stdexcept>
#include <string>
#include <typeinfo>

namespace {

class probe_error : public std::runtime_error {
 public:
  explicit probe_error(const std::string& what) : std::runtime_error(what) {}
};

struct shape {
  virtual ~shape() = default;
  virtual std::string name() const { return "shape"; }
};

struct square : shape {
  std::string name() const override { return "square"; }
};

[[noreturn]] void fail(const std::string& input) {
  if (input.empty()) {
    throw std::invalid_argument("empty input");
  }
  throw probe_error("probe: " + input);
}

// Opaque to the optimizer, so the allocation below is not elided.
volatile std::size_t huge = std::numeric_limits<std::size_t>::max() / 2;
char* volatile sink;

}  // namespace

extern "C" char* cxxprobe_run(const char* input) {
  std::ostringstream out;

  // A derived exception caught by base class reference: the personality
  // routine and __si_class_type_info::__do_catch of libsupc++.
  try {
    fail(input);
  } catch (const std::exception& e) {
    const auto* pe = dynamic_cast<const probe_error*>(&e);
    out << (pe ? "caught probe_error: " : "caught other: ") << e.what();
  }

  // operator new failure: std::bad_alloc thrown by libstdc++'s operator new
  // and caught here.
  try {
    sink = new char[huge];
    out << "; no bad_alloc";
  } catch (const std::bad_alloc&) {
    out << "; bad_alloc";
  }

  // Thrown from inside libstdc++ (std::__throw_invalid_argument). Before the
  // rename, a dynamic link let V8's libc++abi interpose libstdc++.so's
  // __cxa_throw and typeinfo: this one reached std::terminate.
  try {
    const int n = std::stoi("not a number");
    out << "; stoi " << n;
  } catch (const std::invalid_argument& e) {
    out << "; stoi " << e.what();
  }

  // Rethrow through catch (...).
  try {
    try {
      throw std::out_of_range("inner");
    } catch (...) {
      throw;
    }
  } catch (const std::logic_error& e) {
    out << "; rethrown " << e.what();
  }

  // RTTI.
  std::unique_ptr<shape> s(new square);
  out << "; " << (dynamic_cast<square*>(s.get()) ? s->name() : "no cast")
      << (typeid(*s) == typeid(square) ? " typeid ok" : " typeid mismatch");

  // iostreams: std::cout is initialized by libstdc++'s ios_base::Init.
  std::cout.flush();
  out << (std::cout.good() ? "; cout ok" : "; cout bad");

  const std::string r = out.str();
  char* p = static_cast<char*>(std::malloc(r.size() + 1));
  std::memcpy(p, r.c_str(), r.size() + 1);
  return p;
}
