package bench

import "testing"

func mustRun(tb testing.TB, ctx *Context, src string) *Value {
	tb.Helper()
	v, err := ctx.RunScript(src, "bench.js")
	if err != nil {
		tb.Fatal(err)
	}
	return v
}

// globalFunc returns the global JS function `name`. It must be fetched again
// after Context.Cleanup, which releases it.
func globalFunc(tb testing.TB, ctx *Context, name string) *Function {
	tb.Helper()
	v, err := ctx.Global().Get(name)
	if err != nil {
		tb.Fatal(err)
	}
	fn, err := v.AsFunction()
	if err != nil {
		tb.Fatal(err)
	}
	return fn
}

// cleanupEvery releases tracked values every n iterations, outside the timer,
// so long benchmarks don't measure the growth of the value maps. It returns
// true when a cleanup happened (callers must re-fetch their handles).
func cleanupEvery(b *testing.B, i, n int, iso *Isolate, ctx *Context) bool {
	if i == 0 || i%n != 0 {
		return false
	}
	b.StopTimer()
	ctx.Cleanup()
	iso.Cleanup()
	b.StartTimer()
	return true
}
