package bench

import (
	"strings"
	"testing"
)

func BenchmarkNewIsolate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewIsolate().Dispose()
	}
}

func BenchmarkNewContext(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewContext(iso).Close()
	}
}

func BenchmarkRunScriptTrivial(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanupEvery(b, i, 10000, iso, ctx)
		mustRun(b, ctx, "1")
	}
}

func BenchmarkCallJSFromGo(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	mustRun(b, ctx, "function add(a, b) { return a + b }")
	fn := globalFunc(b, ctx, "add")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if cleanupEvery(b, i, 10000, iso, ctx) {
			fn = globalFunc(b, ctx, "add")
		}
		x, _ := NewValue(iso, int32(i))
		y, _ := NewValue(iso, int32(1))
		if _, err := fn.Call(Undefined(iso), x, y); err != nil {
			b.Fatal(err)
		}
	}
}

// One op = 1000 JS->Go callbacks.
func BenchmarkCallGoFromJS1000(b *testing.B) {
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
	mustRun(b, ctx, "function loop() { let s = 0; for (let i = 0; i < 1000; i++) s = goAdd(s, 1); return s }")
	fn := globalFunc(b, ctx, "loop")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if cleanupEvery(b, i, 100, iso, ctx) {
			fn = globalFunc(b, ctx, "loop")
		}
		if _, err := fn.Call(Undefined(iso)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewValueString(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	s := strings.Repeat("x", 256)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanupEvery(b, i, 10000, iso, ctx)
		if _, err := NewValue(iso, s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValueToGoString(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanupEvery(b, i, 10000, iso, ctx)
		_ = mustRun(b, ctx, "'x'.repeat(256)").String()
	}
}

func BenchmarkObjectSetGet(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	obj := ctx.Global()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if cleanupEvery(b, i, 10000, iso, ctx) {
			obj = ctx.Global()
		}
		if err := obj.Set("k", int32(i)); err != nil {
			b.Fatal(err)
		}
		if _, err := obj.Get("k"); err != nil {
			b.Fatal(err)
		}
	}
}

func benchJSONDoc() string {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 200; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"id":1,"name":"product name","price":12.5,"tags":["a","b","c"],"ok":true}`)
	}
	sb.WriteString("]")
	return sb.String() // ~16 KB
}

func BenchmarkJSONParse(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	doc := benchJSONDoc()
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanupEvery(b, i, 1000, iso, ctx)
		if _, err := JSONParse(ctx, doc); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONStringify(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	doc := benchJSONDoc()
	v, err := JSONParse(ctx, doc)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := JSONStringify(ctx, v); err != nil {
			b.Fatal(err)
		}
	}
}

// Cost of releasing 1000 tracked values.
func BenchmarkCleanup1000Values(b *testing.B) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j := 0; j < 1000; j++ {
			mustRun(b, ctx, "1")
		}
		b.StartTimer()
		ctx.Cleanup()
	}
}
