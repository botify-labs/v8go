package v8go_test

import (
	"fmt"
	"strings"
	"testing"

	v8 "github.com/botify-labs/v8go"
)

// Botify: tests for the JS->Go callback path: the context pointer callbacks
// use (botify_context.h, tools/patches/0002), the callback's C++ fast path
// (0003) and the callback frame (0004).

// A function callback finds its context without calling into Go: the values
// it gets must be tracked by the context it runs in, and Context() must be
// that context, also when one template serves several contexts.
func TestFunctionCallbackContextAcrossContexts(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var seen *v8.Context
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("f", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		seen = info.Context()
		return info.Args()[0]
	})); err != nil {
		t.Fatal(err)
	}
	ctx1 := v8.NewContext(iso, global)
	defer ctx1.Close()
	ctx2 := v8.NewContext(iso, global)

	for _, ctx := range []*v8.Context{ctx1, ctx2, ctx1} {
		before := ctx.RetainedValueCount()
		v, err := ctx.RunScript("f(41) + 1", "cb.js")
		if err != nil {
			t.Fatal(err)
		}
		if v.Int32() != 42 {
			t.Fatalf("expected 42, got %v", v)
		}
		if seen != ctx {
			t.Fatal("FunctionCallbackInfo.Context() is not the calling context")
		}
		// The receiver, the argument and the result.
		if got := ctx.RetainedValueCount(); got < before+3 {
			t.Fatalf("callback values not tracked by the calling context: %d -> %d", before, got)
		}
	}

	// Closing a context must not affect the callbacks of another one.
	ctx2.Close()
	ctx1.Cleanup()
	if v, err := ctx1.RunScript("f(1) + f(2)", "cb.js"); err != nil || v.Int32() != 3 {
		t.Fatalf("callback after closing another context: %v, %v", v, err)
	}
	if seen != ctx1 {
		t.Fatal("FunctionCallbackInfo.Context() is not the calling context")
	}
}

// Calls with any number of arguments: the Go side's frames for up to 2 and 4
// arguments and its general case, the C++ side's stack array (up to 7
// arguments) and its heap vector. Args() must not share its backing array with
// anything an append could overwrite.
func TestFunctionCallbackArgumentCounts(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	var got []int32
	var this *v8.Object
	global := v8.NewObjectTemplate(iso)
	if err := global.Set("f", v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
		args := info.Args()
		if cap(args) != len(args) {
			t.Errorf("Args() has capacity %d for %d arguments", cap(args), len(args))
		}
		got = got[:0]
		for _, a := range args {
			got = append(got, a.Int32())
		}
		this = info.This()
		_ = append(args, nil)
		v, _ := v8.NewValue(iso, int32(len(args)))
		return v
	})); err != nil {
		t.Fatal(err)
	}
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	for n := 0; n <= 10; n++ {
		args := make([]string, n)
		for i := range args {
			args[i] = fmt.Sprint(i * 10)
		}
		src := fmt.Sprintf("const o%d = {f, n: %d}; o%d.f(%s)", n, n, n, strings.Join(args, ", "))
		v, err := ctx.RunScript(src, "args.js")
		if err != nil {
			t.Fatal(err)
		}
		if v.Int32() != int32(n) {
			t.Fatalf("%d arguments: callback returned %v", n, v)
		}
		if len(got) != n {
			t.Fatalf("%d arguments: callback got %d", n, len(got))
		}
		for i, a := range got {
			if a != int32(i*10) {
				t.Fatalf("%d arguments: argument %d is %d", n, i, a)
			}
		}
		thisN, err := this.Get("n")
		if err != nil || thisN.Int32() != int32(n) {
			t.Fatalf("%d arguments: wrong receiver (n=%v, %v)", n, thisN, err)
		}
	}
}

// Return values, Go errors and thrown exceptions, whatever the argument count
// (each one of the Go side's frames, and its general case).
func TestFunctionCallbackResultsAndErrors(t *testing.T) {
	t.Parallel()
	iso := v8.NewIsolate()
	defer iso.Dispose()

	global := v8.NewObjectTemplate(iso)
	mustSet := func(name string, cb v8.FunctionCallbackWithError) {
		t.Helper()
		if err := global.Set(name, v8.NewFunctionTemplateWithError(iso, cb)); err != nil {
			t.Fatal(err)
		}
	}
	mustSet("nothing", func(*v8.FunctionCallbackInfo) (*v8.Value, error) { return nil, nil })
	mustSet("last", func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		args := info.Args()
		return args[len(args)-1], nil
	})
	mustSet("goErr", func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, fmt.Errorf("go error %d", len(info.Args()))
	})
	mustSet("jsErr", func(info *v8.FunctionCallbackInfo) (*v8.Value, error) {
		return nil, v8.NewTypeError(iso, fmt.Sprintf("type error %d", len(info.Args())))
	})
	ctx := v8.NewContext(iso, global)
	defer ctx.Close()

	run := func(src string) *v8.Value {
		t.Helper()
		v, err := ctx.RunScript(src, "results.js")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return v
	}
	for _, n := range []int{1, 2, 3, 4, 5, 9} {
		args := make([]string, n)
		for i := range args {
			args[i] = fmt.Sprint(i + 1)
		}
		list := strings.Join(args, ", ")
		if v := run(fmt.Sprintf("nothing(%s)", list)); !v.IsUndefined() {
			t.Errorf("%d arguments: nil result is %v, not undefined", n, v)
		}
		if v := run(fmt.Sprintf("last(%s)", list)); v.Int32() != int32(n) {
			t.Errorf("%d arguments: result is %v, expected %d", n, v, n)
		}
		v := run(fmt.Sprintf("(() => { try { goErr(%s); return null } catch (e) { return e } })()", list))
		if want := fmt.Sprintf("go error %d", n); !v.IsString() || v.String() != want {
			t.Errorf("%d arguments: Go error thrown as %v, expected the string %q", n, v, want)
		}
		v = run(fmt.Sprintf("(() => { try { jsErr(%s); return null } catch (e) { return e.name === \"TypeError\" && e.message } })()", list))
		if want := fmt.Sprintf("type error %d", n); v.String() != want {
			t.Errorf("%d arguments: exception thrown as %v, expected a TypeError %q", n, v, want)
		}
		// An uncaught exception is the script's error.
		if _, err := ctx.RunScript(fmt.Sprintf("jsErr(%s)", list), "results.js"); err == nil ||
			!strings.Contains(err.Error(), fmt.Sprintf("type error %d", n)) {
			t.Errorf("%d arguments: uncaught exception gave error %v", n, err)
		}
	}
}
