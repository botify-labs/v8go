package v8go_test

import (
	"fmt"
	"testing"

	v8 "github.com/botify-labs/v8go"
)

// Botify: tests for the tracked-value set (botify_values.h).

// Releasing a value moves the last tracked value into its slot: the moved
// values must stay usable, and releasable.
func TestValueReleaseKeepsOtherValues(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	base := ctx.RetainedValueCount()
	const n = 100
	vals := make([]*v8.Value, n)
	for i := range vals {
		v, err := ctx.RunScript(fmt.Sprint(i), "value.js")
		if err != nil {
			t.Fatal(err)
		}
		vals[i] = v
	}
	// Release from the front, so that each release moves the last value.
	for i := 0; i < n; i += 2 {
		vals[i].Release()
	}
	if got := ctx.RetainedValueCount(); got != base+n/2 {
		t.Fatalf("expected %d retained values, got %d", base+n/2, got)
	}
	for i := 1; i < n; i += 2 {
		if got := vals[i].Int32(); got != int32(i) {
			t.Fatalf("value %d reads %d after other releases", i, got)
		}
	}
	for i := n - 1; i > 0; i -= 2 {
		vals[i].Release()
	}
	if got := ctx.RetainedValueCount(); got != base {
		t.Fatalf("expected %d retained values, got %d", base, got)
	}
}

// Cleanups of many values release them by handle address (sorted). Weak owners
// of Go values must stay tracked, and the next values must work.
func TestCleanupOfManyValuesKeepsGoValues(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()
	ctx := v8.NewContext(iso)
	defer ctx.Close()

	goVal, err := v8.NewValue(iso, &cleanupPayload{n: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.Global().Set("goVal", goVal); err != nil {
		t.Fatal(err)
	}
	base := iso.InternalRetainedValueCount()
	for round := 0; round < 3; round++ {
		for i := 0; i < 40000; i++ {
			if _, err := v8.NewValue(iso, int32(i)); err != nil {
				t.Fatal(err)
			}
			if _, err := ctx.RunScript("1", "many.js"); err != nil {
				t.Fatal(err)
			}
		}
		ctx.Cleanup()
		iso.Cleanup()
		if got := ctx.RetainedValueCount(); got != 0 {
			t.Fatalf("round %d: %d values retained by the context after Cleanup", round, got)
		}
		// The Go value's weak owner stays, as do the Undefined and Null
		// values Cleanup creates again.
		if got := iso.InternalRetainedValueCount(); got > base {
			t.Fatalf("round %d: %d internal values after Cleanup, expected at most %d", round, got, base)
		}
	}
	got, err := ctx.Global().Get("goVal")
	if err != nil {
		t.Fatal(err)
	}
	ext, ok := got.External()
	if !ok || ext.(*cleanupPayload).n != 7 {
		t.Fatal("Go value lost after Cleanups of many values")
	}
}

// Releasing values moves others, the weak owners of Go values included (see
// NewValueGo), and Cleanup renumbers the owners it keeps: when V8 collects
// their Externals, GoValueWeakCallback must release each owner by its current
// id, and only it.
func TestGoValueWeakCallbacksAfterIdsMoved(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	// collect runs V8 GCs until the internal context tracks want values.
	collect := func(want int) {
		t.Helper()
		got := iso.InternalRetainedValueCount()
		for i := 0; i < 5 && got > want; i++ {
			iso.LowMemoryNotification()
			got = iso.InternalRetainedValueCount()
		}
		if got != want {
			t.Fatalf("%d internal values after V8 GCs, expected %d", got, want)
		}
	}

	const n = 100
	newValues := func() (gos, ints []*v8.Value) {
		for i := 0; i < n; i++ {
			g, err := v8.NewValue(iso, &cleanupPayload{n: i})
			if err != nil {
				t.Fatal(err)
			}
			v, err := v8.NewValue(iso, int32(i))
			if err != nil {
				t.Fatal(err)
			}
			gos, ints = append(gos, g), append(ints, v)
		}
		return gos, ints
	}

	// Released values: the last tracked values, owners included, move into
	// their slots.
	base := iso.InternalRetainedValueCount()
	gos, ints := newValues()
	// Per i: the Go value's weak owner, the Go value and the int.
	if got := iso.InternalRetainedValueCount(); got != base+3*n {
		t.Fatalf("%d internal values, expected %d", got, base+3*n)
	}
	for _, g := range gos {
		g.Release()
	}
	collect(base + n)
	for i, v := range ints {
		if got := v.Int32(); got != int32(i) {
			t.Fatalf("value %d reads %d after the Go values were collected", i, got)
		}
	}

	// Values released by Cleanup: the owners it keeps get new ids.
	newValues()
	iso.Cleanup()
	kept := iso.InternalRetainedValueCount()
	if kept < n {
		t.Fatalf("%d internal values after Cleanup, expected at least the %d owners", kept, n)
	}
	collect(kept - n)

	// The same through a Cleanup of more values than kSortReleased
	// (botify_values.h), which takes the path that releases them by handle
	// address. The owners come after thousands of released values: they all
	// get new ids. A stale id would leave a deleted owner tracked, which
	// collect reports, and Dispose would then crash on.
	pad := func() {
		t.Helper()
		for i := 0; i < 17000; i++ {
			if _, err := v8.NewValue(iso, int32(i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	pad()
	newValues()
	pad()
	iso.Cleanup()
	kept = iso.InternalRetainedValueCount()
	if kept < n {
		t.Fatalf("%d internal values after a large Cleanup, expected at least the %d owners", kept, n)
	}
	collect(kept - n)
}
