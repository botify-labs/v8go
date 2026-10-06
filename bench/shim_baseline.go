//go:build v8baseline

// Package bench compares the baseline v8go (6f9829d, V8 9.0) with the new one.
// The shim gives both the same names; select the baseline with -tags v8baseline.
package bench

import v8 "rogchap.com/v8go"

const Version = "baseline"

type (
	Isolate              = v8.Isolate
	Context              = v8.Context
	Value                = v8.Value
	Object               = v8.Object
	Function             = v8.Function
	FunctionCallbackInfo = v8.FunctionCallbackInfo
	CompileOptions       = v8.CompileOptions
	CompilerCachedData   = v8.CompilerCachedData
	Valuer               = v8.Valuer
	HeapStatistics       = v8.HeapStatistics
)

var (
	NewValue      = v8.NewValue
	Undefined     = v8.Undefined
	JSONParse     = v8.JSONParse
	JSONStringify = v8.JSONStringify
)

func NewIsolate() *Isolate { return v8.NewIsolate() }

func NewContext(iso *Isolate) *Context { return v8.NewContext(iso) }

func NewContextWithFuncs(iso *Isolate, fns map[string]func(*FunctionCallbackInfo) *Value) *Context {
	global := v8.NewObjectTemplate(iso)
	for name, fn := range fns {
		if err := global.Set(name, v8.NewFunctionTemplate(iso, fn)); err != nil {
			panic(err)
		}
	}
	return v8.NewContext(iso, global)
}

// goPayload: Go values in JS don't exist in 6f9829d; use a string instead.
func goPayload(i int) interface{} { return "payload" }
