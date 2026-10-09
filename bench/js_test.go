package bench

import "testing"

// Deterministic CPU-bound workloads; each returns a number so results are
// checked and can't be optimized away.
const jsWorkloads = `
function lcg(seed) { let s = seed; return () => (s = (s * 1103515245 + 12345) % 2147483648) / 2147483648; }

function wlRegex() {
  let s = '';
  for (let i = 0; i < 500; i++) s += 'user' + String.fromCharCode(97 + i % 26) + '@example.com, ';
  let n = 0;
  for (const m of s.matchAll(/([a-z]+)@([a-z]+)\.com/g)) n += m[1].length;
  return n + s.replace(/example/g, 'botify').length;
}

function wlObjects() {
  const items = [];
  for (let i = 0; i < 10000; i++) items.push({ id: i, name: 'item' + i, tags: ['a', 'b'], price: i * 1.5 });
  let total = 0;
  for (const it of items) total += it.price + it.tags.length + it.name.length;
  return total;
}

function wlArrays() {
  const rnd = lcg(42);
  const a = Array.from({ length: 20000 }, () => rnd());
  a.sort((x, y) => x - y);
  return a.map(x => x * 2).filter(x => x > 1).reduce((s, x) => s + x, 0);
}

function wlStrings() {
  const parts = [];
  for (let i = 0; i < 5000; i++) parts.push('<li class="item-' + i + '">' + 'x'.repeat(i % 20) + '</li>');
  const html = parts.join('');
  return html.split('</li>').length + html.toUpperCase().indexOf('ITEM-4999');
}

function wlJSON() {
  const doc = [];
  for (let i = 0; i < 2000; i++) doc.push({ id: i, url: '/p/' + i, meta: { title: 't' + i, n: [1, 2, 3] } });
  return JSON.parse(JSON.stringify(doc)).length;
}
`

func benchJSWorkload(b *testing.B, name string) {
	iso := NewIsolate()
	defer iso.Dispose()
	ctx := NewContext(iso)
	defer ctx.Close()
	mustRun(b, ctx, jsWorkloads)
	fn := globalFunc(b, ctx, name)
	if _, err := fn.Call(Undefined(iso)); err != nil { // warm up
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if cleanupEvery(b, i, 1000, iso, ctx) {
			fn = globalFunc(b, ctx, name)
		}
		v, err := fn.Call(Undefined(iso))
		if err != nil {
			b.Fatal(err)
		}
		if !v.IsNumber() {
			b.Fatalf("%s returned %v", name, v)
		}
	}
}

func BenchmarkJSRegex(b *testing.B)   { benchJSWorkload(b, "wlRegex") }
func BenchmarkJSObjects(b *testing.B) { benchJSWorkload(b, "wlObjects") }
func BenchmarkJSArrays(b *testing.B)  { benchJSWorkload(b, "wlArrays") }
func BenchmarkJSStrings(b *testing.B) { benchJSWorkload(b, "wlStrings") }
func BenchmarkJSJSON(b *testing.B)    { benchJSWorkload(b, "wlJSON") }
