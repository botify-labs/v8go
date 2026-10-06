package bench

import "testing"

// heapB/iso: V8 heap used by a fresh isolate with one context.
func BenchmarkHeapPerIsolate(b *testing.B) {
	var total uint64
	for i := 0; i < b.N; i++ {
		iso := NewIsolate()
		ctx := NewContext(iso)
		mustRun(b, ctx, "1")
		total += iso.GetHeapStatistics().UsedHeapSize
		ctx.Close()
		iso.Dispose()
	}
	b.ReportMetric(float64(total)/float64(b.N), "heapB/iso")
}

// heapB/ctx: V8 heap added by each extra context in one isolate.
func BenchmarkHeapPerContext(b *testing.B) {
	const contexts = 50
	var total float64
	for i := 0; i < b.N; i++ {
		iso := NewIsolate()
		before := iso.GetHeapStatistics().UsedHeapSize
		ctxs := make([]*Context, 0, contexts)
		for j := 0; j < contexts; j++ {
			ctx := NewContext(iso)
			mustRun(b, ctx, "var x = {a: 1}")
			ctxs = append(ctxs, ctx)
		}
		after := iso.GetHeapStatistics().UsedHeapSize
		total += float64(after-before) / contexts
		for _, ctx := range ctxs {
			ctx.Close()
		}
		iso.Dispose()
	}
	b.ReportMetric(total/float64(b.N), "heapB/ctx")
}
