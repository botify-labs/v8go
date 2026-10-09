#!/usr/bin/env python3
"""Per-callback instruction counts from callgrind runs at 20 and 40 ops.

Usage: cgdiff.py <dir> <variant>...   (reads <dir>/<v>-{20,40}-{self,incl}.txt)
"""
import re
import sys
from collections import defaultdict

CALLS = 20 * 1000  # 20 more ops of 1000 callbacks each
LINE = re.compile(r"^\s*([\d,]+) \([^)]*\)\s+(.*?)(?: \[[^\]]*\])?$")


def parse(path):
    total, funcs, section = 0, {}, 0
    with open(path) as f:
        for line in f:
            if section == 0:
                m = LINE.match(line)
                if m and m.group(2) == "PROGRAM TOTALS":
                    total = int(m.group(1).replace(",", ""))
                if line.startswith("Ir") and "file:function" in line:
                    section = 1
                continue
            if section == 1:
                if line.startswith("-----"):
                    section = 2
                continue
            if line.startswith("-----") or not line.strip():
                break
            m = LINE.match(line)
            if not m or "=>" in line:
                continue
            n = int(m.group(1).replace(",", ""))
            name = re.sub(r"^\?\?\?:", "", m.group(2))
            name = re.sub(r"^[^:]*\.(go|c|cc|s|S|h):", "", name)
            name = re.sub(r"'\d+$", "", name)
            funcs[name] = funcs.get(name, 0) + n
    return total, funcs
def per_call(d, v, kind):
    t20, f20 = parse(f"{d}-{v}-20-{kind}.txt")
    t40, f40 = parse(f"{d}-{v}-40-{kind}.txt")
    funcs = {k: (f40.get(k, 0) - f20.get(k, 0)) / CALLS for k in set(f20) | set(f40)}
    return (t40 - t20) / CALLS, funcs


def category(name):
    if re.search(r"malloc|_int_free|\bfree\b|cfree|operator new|operator delete|tcache", name):
        return "libc malloc/free"
    if name.startswith("runtime.cgo") or name in (
            "runtime.cgocall", "runtime.cgocallback", "runtime.cgocallbackg",
            "runtime.cgocallbackg1", "runtime.asmcgocall", "crosscall2",
            "_cgoexp_", "runtime.entersyscall", "runtime.exitsyscall",
            "runtime.reentersyscall", "runtime.exitsyscallfast",
            "runtime.casgstatus", "runtime.lockOSThread", "runtime.unlockOSThread",
            "runtime.dolockOSThread", "runtime.dounlockOSThread", "runtime.save",
            "runtime.exitsyscallfast_reacquired", "runtime.entersyscall_sysmon",
            "runtime.callbackUpdateSystemStack") or "_cgo_" in name or "cgocall" in name or "syscall" in name:
        return "cgo transitions"
    if name.startswith("runtime.mallocgc") or name.startswith("runtime.") and (
            "alloc" in name or "gc" in name.lower() or "mark" in name or "sweep" in name
            or "heap" in name or "span" in name or "scan" in name or "memclr" in name):
        return "Go alloc/GC"
    if name.startswith("runtime.") or name.startswith("sync.") or name.startswith("internal/"):
        return "Go runtime other"
    if "botify-labs/v8go" in name or name.startswith("main.") or "bench." in name:
        return "v8go Go"
    if name.startswith("v8::") or name.startswith("Builtins_") or "v8::internal" in name:
        return "V8"
    if name[:1].isupper() or "tracked_value" in name or "m_value" in name or "ValueTracker" in name:
        return "v8go C++"
    return "other"


def main():
    d, variants = sys.argv[1], sys.argv[2:]
    data = {v: per_call(d, v, "self") for v in variants}
    print(d + ": instructions per unit ((Ir@40 ops - Ir@20 ops) / 20000)\n")
    print("variant  total")
    for v in variants:
        print(f"{v:8} {data[v][0]:8.0f}")

    print("\nBy category (self cost):")
    cats = {v: defaultdict(float) for v in variants}
    for v in variants:
        for k, n in data[v][1].items():
            cats[v][category(k)] += n
    names = sorted({c for v in variants for c in cats[v]}, key=lambda c: -cats[variants[0]][c])
    print(f"{'category':24}" + "".join(f"{v:>9}" for v in variants))
    for c in names:
        print(f"{c:24}" + "".join(f"{cats[v][c]:9.0f}" for v in variants))

    print("\nTop functions (self cost per callback, union of each variant's top 45):")
    top = set()
    for v in variants:
        top |= set(sorted(data[v][1], key=lambda k: -data[v][1][k])[:45])
    rows = sorted(top, key=lambda k: -max(data[v][1].get(k, 0) for v in variants))
    print("".join(f"{v:>9}" for v in variants) + "  function")
    for k in rows:
        vals = [data[v][1].get(k, 0) for v in variants]
        if max(vals) < 5:
            continue
        print("".join(f"{x:9.0f}" for x in vals) + "  " + k[:150])

    print("\nInclusive cost per callback of v8go entry points:")
    incl = {v: per_call(d, v, "incl")[1] for v in variants}
    keys = [k for k in set().union(*[set(incl[v]) for v in variants])
            if re.search(r"FunctionTemplateCallback|goFunctionCallback$|goContext$|ValueToInt32|"
                         r"NewValueInteger|tracked_value|ISOLATE|Locker::Initialize|"
                         r"Isolate::Enter|Isolate::Exit|HandleScope::Initialize|TryCatch::TryCatch|"
                         r"Context::Enter|Context::Exit|getContext|getCallback|NewValue$|"
                         r"\.Int32$|runtime.cgocallback$|runtime.cgocall$|crosscall2|"
                         r"Builtins_CallApiCallback|HandleApiCallHelper|Builtins_HandleApiCall", k)]
    keys.sort(key=lambda k: -max(incl[v].get(k, 0) for v in variants))
    print("".join(f"{v:>9}" for v in variants) + "  function")
    for k in keys:
        vals = [incl[v].get(k, 0) for v in variants]
        if max(vals) < 5:
            continue
        print("".join(f"{x:9.0f}" for x in vals) + "  " + k[:150])


main()
