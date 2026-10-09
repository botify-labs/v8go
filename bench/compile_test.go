package bench

import (
	"os"
	"testing"
)

func lodash(tb testing.TB) string {
	tb.Helper()
	src, err := os.ReadFile("testdata/lodash.js")
	if err != nil {
		tb.Fatal(err)
	}
	return string(src)
}

func BenchmarkCompileLodashCold(b *testing.B) {
	src := lodash(b)
	b.SetBytes(int64(len(src)))
	for i := 0; i < b.N; i++ {
		// A fresh isolate per op: V8's in-isolate compilation cache would
		// otherwise turn this into a cache hit.
		b.StopTimer()
		iso := NewIsolate()
		b.StartTimer()
		if _, err := iso.CompileUnboundScript(src, "lodash.js", CompileOptions{}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		iso.Dispose()
		b.StartTimer()
	}
}

func BenchmarkCompileLodashWithCodeCache(b *testing.B) {
	src := lodash(b)
	iso := NewIsolate()
	us, err := iso.CompileUnboundScript(src, "lodash.js", CompileOptions{})
	if err != nil {
		b.Fatal(err)
	}
	cache := us.CreateCodeCache()
	iso.Dispose()
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		iso := NewIsolate()
		b.StartTimer()
		if _, err := iso.CompileUnboundScript(src, "lodash.js", CompileOptions{CachedData: cache}); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		iso.Dispose()
		b.StartTimer()
	}
}

// Startup: new isolate + context, load lodash from the code cache, run it.
func BenchmarkStartupWithLodash(b *testing.B) {
	src := lodash(b)
	iso := NewIsolate()
	us, err := iso.CompileUnboundScript(src, "lodash.js", CompileOptions{})
	if err != nil {
		b.Fatal(err)
	}
	cache := us.CreateCodeCache()
	iso.Dispose()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iso := NewIsolate()
		ctx := NewContext(iso)
		us, err := iso.CompileUnboundScript(src, "lodash.js", CompileOptions{CachedData: cache})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := us.Run(ctx); err != nil {
			b.Fatal(err)
		}
		mustRun(b, ctx, "_.chunk([1, 2, 3, 4], 2).length")
		ctx.Close()
		iso.Dispose()
	}
}
