//go:build soak && linux

package bench

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func rssBytes(tb testing.TB) uint64 {
	tb.Helper()
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		tb.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			kb, err := strconv.ParseUint(strings.Fields(line)[1], 10, 64)
			if err != nil {
				tb.Fatal(err)
			}
			return kb * 1024
		}
	}
	tb.Fatal("VmRSS not found")
	return 0
}

// growthSecondHalf is the relative growth between the middle and the last sample.
func growthSecondHalf(samples []uint64) float64 {
	mid, last := samples[len(samples)/2], samples[len(samples)-1]
	return float64(last)/float64(mid) - 1
}

func soakIterations() int {
	if s := os.Getenv("SOAK_ITERATIONS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 2000 {
			return n
		}
	}
	return 100000
}

// soakDuration is the minimum length of the run (SOAK_DURATION, a Go
// duration such as 25s). When set, it replaces SOAK_ITERATIONS: the run lasts
// that long whatever the machine's speed, so V8's memory reducer fires.
func soakDuration(tb testing.TB) time.Duration {
	s := os.Getenv("SOAK_DURATION")
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 10*time.Second {
		tb.Fatalf("SOAK_DURATION=%q: want a duration of at least 10s", s)
	}
	return d
}

// One long-lived isolate/context, reused across runs: run scripts, call Go
// from JS, wrap Go values, compile from a code cache, then Cleanup.
//
// With the new V8, RSS only plateaus because Isolate.Cleanup runs V8's memory
// reducer, a timer task (>= 8 s): the run must last long enough for it to fire
// (100k iterations take ~20 s). Without it, V8's first major GC comes only
// after ~170k iterations. A fixed iteration count lasts less than that on a
// fast machine: CI sets SOAK_DURATION instead.
func TestSoakCleanup(t *testing.T) {
	iters, minDuration := soakIterations(), soakDuration(t)
	start := time.Now()
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContextWithFuncs(iso, map[string]func(*FunctionCallbackInfo) *Value{
		"goAdd": func(info *FunctionCallbackInfo) *Value {
			args := info.Args()
			v, _ := NewValue(iso, args[0].Int32()+args[1].Int32())
			return v
		},
	})
	defer ctx.Close()
	mustRun(t, ctx, jsWorkloads)

	const script = "var s = 0; for (let i = 0; i < 50; i++) s = goAdd(s, 1); var o = wlObjects(); new Promise(() => {}); s"
	us, err := iso.CompileUnboundScript(script, "soak.js", CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cache := us.CreateCodeCache()
	goroutines := runtime.NumGoroutine()

	var rss, heap, goHeap []uint64
	var ms runtime.MemStats
	for i := 0; ; i++ {
		if minDuration > 0 {
			if i%1000 == 0 && i >= 4000 && time.Since(start) >= minDuration {
				break
			}
		} else if i >= iters {
			break
		}
		us, err := iso.CompileUnboundScript(script, "soak.js", CompileOptions{CachedData: cache})
		if err != nil {
			t.Fatal(err)
		}
		v, err := us.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if v.Int32() != 50 {
			t.Fatalf("iteration %d: expected 50, got %v", i, v)
		}
		if _, err := NewValue(iso, goPayload(i)); err != nil {
			t.Fatal(err)
		}
		ctx.Cleanup()
		iso.Cleanup()

		if i%1000 == 999 {
			runtime.GC()
			runtime.ReadMemStats(&ms)
			rss = append(rss, rssBytes(t))
			heap = append(heap, iso.GetHeapStatistics().UsedHeapSize)
			goHeap = append(goHeap, ms.HeapAlloc)
		}
	}

	if out := os.Getenv("SOAK_OUT"); out != "" {
		var sb strings.Builder
		sb.WriteString("iteration,rss,v8heap,goheap\n")
		for j := range rss {
			fmt.Fprintf(&sb, "%d,%d,%d,%d\n", (j+1)*1000, rss[j], heap[j], goHeap[j])
		}
		if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	g := growthSecondHalf(rss)
	t.Logf("%s: %d iterations in %s, RSS %d -> %d MiB, second-half growth %.2f%%, V8 heap last %d KiB, Go heap last %d KiB",
		Version, len(rss)*1000, time.Since(start).Round(time.Second), rss[0]>>20, rss[len(rss)-1]>>20, g*100, heap[len(heap)-1]>>10, goHeap[len(goHeap)-1]>>10)
	if g > 0.05 {
		t.Errorf("RSS grew %.2f%% over the second half (limit 5%%): leak", g*100)
	}
	if n := runtime.NumGoroutine(); n > goroutines+2 {
		t.Errorf("goroutines grew from %d to %d", goroutines, n)
	}
}
