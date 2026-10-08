# v8go (Botify fork): execute JavaScript from Go

[![Go Reference](https://pkg.go.dev/badge/github.com/botify-labs/v8go.svg)](https://pkg.go.dev/github.com/botify-labs/v8go)

<img src="gopher.jpg" width="200px" alt="V8 Gopher based on original artwork from the amazing Renee French" style="float:right" />

`github.com/botify-labs/v8go` is Botify's fork of v8go, the Go binding to the
[V8](https://v8.dev/) JavaScript engine. It is a snapshot of
[tommie/v8go](https://github.com/tommie/v8go) (commit in `deps/tommie_sha`), itself a fork of
[rogchap/v8go](https://github.com/rogchap/v8go), with **V8 15.4.80.20**, renamed to the module path
`github.com/botify-labs/v8go`, plus the Botify additions below. It replaces the older Botify fork of
`rogchap.com/v8go` (V8 9.0): see [MIGRATION.md](MIGRATION.md).

```sh
go get github.com/botify-labs/v8go@v0.10.0
```

## Requirements

V8 and v8go's C++ bridge ship prebuilt in the `deps/<os>_<arch>` modules and link statically.
Consumers compile no C++ for v8go.

| | Linux, macOS | Windows |
|---|---|---|
| Go | the version in `go.mod` | ≥ 1.27 |
| C compiler | any (system gcc or clang), no flags, no `CGO_CXXFLAGS` | MSVC-target clang (LLVM ≥ 21), with MSVC Build Tools and the Windows SDK |
| Environment | nothing | `CC="clang -fuse-ld=lld"`, `CXX="clang++ -fuse-ld=lld"`, clang on `PATH` |
| C runtime | system libc | static MSVC CRT (`/MT`); **MinGW is not supported** |

Windows notes: `-fuse-ld=lld` must be in `CC` (or passed with `-ldflags=-extldflags=-fuse-ld=lld`)
for Go to detect LLD, otherwise it passes flags only GNU ld accepts. Go splits `CC` on spaces, so
clang must be found on `PATH` rather than given as a full path. Every other cgo dependency of the
binary must be MSVC-compatible too: see [CGO-DEPENDENCIES.md](CGO-DEPENDENCIES.md).

### Platforms

linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. Every one is built and tested
by `botify-ci` (Android, supported upstream, is dropped).

### glibc (Linux)

- V8 needs **glibc ≥ 2.31**: tommie builds it against Chromium's Debian bullseye sysroot. This floor
  is tommie's; Botify's CI does not test it.
- Botify's runtime target is Amazon Linux 2023 (glibc 2.34). The `glibc-floor` job of `botify-ci`
  builds a consumer test binary in `amazonlinux:2023` (linux/amd64), checks that it requires no
  symbol version newer than `GLIBC_2.34`, and runs the tests there.
- The floor of a given binary depends on the host that links it: V8's archives reference
  unversioned libc/libm symbols, bound to the link host's default versions (linking on glibc ≥ 2.38
  requires `fmod@GLIBC_2.38`). Link on the oldest target, or check the binary with
  `objdump -T <binary> | grep -oE 'GLIBC_[0-9.]+' | sort -Vu | tail -1`. Fully static binaries are not
  concerned.

## Botify additions

- **Prebuilt C++ bridge**: the `.cc` files only compile with `-tags v8go_source`; consumers link
  `deps/<os>_<arch>/libv8go.a` (`v8go.lib` on Windows), checked against the sources by CI.
- **`Isolate.Cleanup` / `Context.Cleanup`**: release the values and scripts a long-lived isolate or
  context tracks, and run V8's pending GC tasks (memory reducer). Cleanup never runs JavaScript or Go
  callbacks, and does nothing when called with JavaScript on the stack.
- **`SetDefaultLocale`**: pins the default locale of V8's ICU, process-wide, so that `Intl` and the
  locale-sensitive methods don't depend on the host's locale.
- **Faster JS→Go calls**: indexed value tracking, the context found without a call into Go, one
  allocation per call up to 4 arguments, plus guards for callbacks of closed contexts and terminated
  nested scripts.
- **C++ runtime isolation on Linux**: V8's copy of Chromium's libc++abi/libc++ ABI layer is renamed
  (`.v8cr` suffix) in the Linux archives, so binaries also linking g++/libstdc++ C++ work, fully static
  included.
- **No allocator shim**: PartitionAlloc's malloc replacement is removed from V8's archives; the
  process keeps the system `malloc`.

## Documentation

- [MIGRATION.md](MIGRATION.md): migrating from the old fork (`rogchap.com/v8go`, V8 9.0): toolchains,
  glibc, memory, behaviour and API changes (in French).
- [CGO-DEPENDENCIES.md](CGO-DEPENDENCIES.md): other cgo libraries next to V8 (static linking, MSVC
  rebuilds, linux/arm64 archives), and checks for new ones (in French).
- [BOTIFY.md](BOTIFY.md): how the fork works: prebuilt bridges, pinned `deps/*` modules, release
  procedure, V8 upgrades, benchmarks (in French).
- [CHANGELOG.md](CHANGELOG.md): Botify releases, then tommie/v8go's history.
- Go reference: https://pkg.go.dev/github.com/botify-labs/v8go

## Usage

```go
import v8 "github.com/botify-labs/v8go"
```

### Running a script

```go
ctx := v8.NewContext() // creates a new V8 context with a new Isolate aka VM
ctx.RunScript("const add = (a, b) => a + b", "math.js") // executes a script on the global context
ctx.RunScript("const result = add(3, 4)", "main.js") // any functions previously added to the context can be called
val, _ := ctx.RunScript("result", "value.js") // return a value in JavaScript back to Go
fmt.Printf("addition result: %s", val)
```

### One VM, many contexts

```go
iso := v8.NewIsolate() // creates a new JavaScript VM
ctx1 := v8.NewContext(iso) // new context within the VM
ctx1.RunScript("const multiply = (a, b) => a * b", "math.js")

ctx2 := v8.NewContext(iso) // another context on the same VM
if _, err := ctx2.RunScript("multiply(3, 4)", "main.js"); err != nil {
  // this will error as multiply is not defined in this context
}
```

### JavaScript function with Go callback

```go
iso := v8.NewIsolate() // create a new VM
// a template that represents a JS function
printfn := v8.NewFunctionTemplate(iso, func(info *v8.FunctionCallbackInfo) *v8.Value {
    fmt.Printf("%v", info.Args()) // when the JS function is called this Go callback will execute
    return nil // you can return a value back to the JS caller if required
})
global := v8.NewObjectTemplate(iso) // a template that represents a JS Object
global.Set("print", printfn) // sets the "print" property of the Object to our function
ctx := v8.NewContext(iso, global) // new Context with the global Object set to our object template
ctx.RunScript("print('foo')", "print.js") // will execute the Go callback with a single argunent 'foo'
```

### Update a JavaScript object from Go

```go
ctx := v8.NewContext() // new context with a default VM
obj := ctx.Global() // get the global object from the context
obj.Set("version", "v1.0.0") // set the property "version" on the object
val, _ := ctx.RunScript("version", "version.js") // global object will have the property set within the JS VM
fmt.Printf("version: %s", val)

if obj.Has("version") { // check if a property exists on the object
    obj.Delete("version") // remove the property from the object
}
```

### JavaScript errors

```go
val, err := ctx.RunScript(src, filename)
if err != nil {
  e := err.(*v8.JSError) // JavaScript errors will be returned as the JSError struct
  fmt.Println(e.Message) // the message of the exception thrown
  fmt.Println(e.Location) // the filename, line number and the column where the error occurred
  fmt.Println(e.StackTrace) // the full stack trace of the error, if available

  fmt.Printf("javascript error: %v", e) // will format the standard error message
  fmt.Printf("javascript stack trace: %+v", e) // will format the full error stack trace
}
```

### Pre-compile context-independent scripts to speed-up execution times

For scripts that are large or are repeatedly run in different contexts,
it is beneficial to compile the script once and used the cached data from that
compilation to avoid recompiling every time you want to run it.

```go
source := "const multiply = (a, b) => a * b"
iso1 := v8.NewIsolate() // creates a new JavaScript VM
ctx1 := v8.NewContext(iso1) // new context within the VM
script1, _ := iso1.CompileUnboundScript(source, "math.js", v8.CompileOptions{}) // compile script to get cached data
val, _ := script1.Run(ctx1)

cachedData := script1.CreateCodeCache()

iso2 := v8.NewIsolate() // create a new JavaScript VM
ctx2 := v8.NewContext(iso2) // new context within the VM

script2, _ := iso2.CompileUnboundScript(source, "math.js", v8.CompileOptions{CachedData: cachedData}) // compile script in new isolate with cached data
val, _ = script2.Run(ctx2)
```

### Terminate long running scripts

```go
vals := make(chan *v8.Value, 1)
errs := make(chan error, 1)

go func() {
    val, err := ctx.RunScript(script, "forever.js") // exec a long running script
    if err != nil {
        errs <- err
        return
    }
    vals <- val
}()

select {
case val := <- vals:
    // success
case err := <- errs:
    // javascript error
case <- time.After(200 * time.Millisecond):
    vm := ctx.Isolate() // get the Isolate from the context
    vm.TerminateExecution() // terminate the execution
    err := <- errs // will get a termination error back from the running script
}
```

### Setting memory limits
V8 supports setting a hard limit on Javascript memory usage.
To do so, add a call to `WithResourceConstraints` to the `NewIsolate` invocation.
If the limit is hit, v8go terminates the running script, like `TerminateExecution` above, instead of letting V8 end the process.
The error matches `v8.ErrHeapLimitReached`. The isolate can be used again, with its initial limit restored, but the state the interrupted script left behind is undefined: recycling the isolate is recommended.

```go
vm := v8.NewIsolate(v8.WithResourceConstraints(8*1024*1024, 16*1024*1024))
ctx := v8.NewContext(vm)
val, err = ctx.RunScript(`
    const data = [];
    for (let i = 0; i < 1000 * 1000; i++) {
        data.push("large data chunk ".repeat(1000));
    }
    data.length;
  `, "memory-test.js")
// errors.Is(err, v8.ErrHeapLimitReached) is true.
```

### CPU Profiler

V8 only samples the OS thread that started profiling.
`CPUProfiler.Do` keeps the profiled function on that thread.
When using `StartProfiling` and `StopProfiling` directly, call `runtime.LockOSThread` first, and execute JavaScript on the same goroutine.
Otherwise, samples are silently missing from the profile.

```go
func createProfile() {
	iso := v8.NewIsolate()
	ctx := v8.NewContext(iso)
	cpuProfiler := v8.NewCPUProfiler(iso)

	cpuProfile := cpuProfiler.Do("my-profile", func() {
		ctx.RunScript(profileScript, "script.js") // this script is defined in cpuprofiler_test.go
		val, _ := ctx.Global().Get("start")
		fn, _ := val.AsFunction()
		fn.Call(ctx.Global())
	})

	printTree("", cpuProfile.GetTopDownRoot()) // helper function to print the profile
}

func printTree(nest string, node *v8.CPUProfileNode) {
	fmt.Printf("%s%s %s:%d:%d\n", nest, node.GetFunctionName(), node.GetScriptResourceName(), node.GetLineNumber(), node.GetColumnNumber())
	count := node.GetChildrenCount()
	if count == 0 {
		return
	}
	nest = fmt.Sprintf("%s  ", nest)
	for i := 0; i < count; i++ {
		printTree(nest, node.GetChild(i))
	}
}

// Output
// (root) :0:0
//   (program) :0:0
//   start script.js:23:15
//     foo script.js:15:13
//       delay script.js:12:15
//         loop script.js:1:14
//       bar script.js:13:13
//         delay script.js:12:15
//           loop script.js:1:14
//       baz script.js:14:13
//         delay script.js:12:15
//           loop script.js:1:14
//   (garbage collector) :0:0
```

## Development

Contributors working on v8go itself build its C++ sources instead of the prebuilt bridge. V8 is built
with Chromium's hardened libc++, so v8go must be compiled against the same headers
(`deps/include_libcxx/`), with Clang 21 or newer. `-nostdinc++` isn't allowed in `#cgo` directives,
so it goes in `CGO_CXXFLAGS`:

```sh
CC=clang-21 CXX=clang++-21 CGO_CXXFLAGS=-nostdinc++ go test -tags v8go_source ./...
```

These variables apply to every cgo package of the build. On Windows:
`CC="clang -fuse-ld=lld" CXX="clang++ -fuse-ld=lld" CGO_CXXFLAGS=-nostdinc++`.

The Docker dev image (`tools/docker/dev.sh '<command>'`) has the toolchains (Go, clang 21, gcc,
benchstat, LLVM binutils). After a change to the C++ sources, the prebuilt bridges must be rebuilt
and the `deps/*` modules re-pinned: see [BOTIFY.md](BOTIFY.md). Source mode has limits (no C++
runtime isolation), also listed there.

### Leak checking

`go test -c -tags 'leakcheck v8go_source'` builds the tests with the
[Leak Sanitizer](https://clang.llvm.org/docs/LeakSanitizer.html) (Linux; `botify-ci` runs it). Run
the resulting `./v8go.test`; separate compile and run steps give line numbers in backtraces.

### Formatting

Go has `go fmt`; the `*.h` and `*.cc` files follow `clang-format` with the Chromium style
(`.clang-format`, `go generate`). clang-format rewrites the `//go:build v8go_source` line of the
`.cc` files: see the pitfall in [BOTIFY.md](BOTIFY.md) before formatting.

### Debugging V8

Release builds of V8 have no debug information. To get C++ backtraces with line numbers, build V8
with debug info and checks (`deps/build.py --debug`, see tommie/v8go), build the test binary with
`go test -c -ldflags=-compressdwarf=false`, and run it under a debugger. Output printed with C
`printf` is buffered: `fflush(stdout)` after printing.

## Credits and license

v8go was created by Roger Chapman ([rogchap/v8go](https://github.com/rogchap/v8go)) and is
maintained upstream in [tommie/v8go](https://github.com/tommie/v8go); this fork
tracks tommie/v8go. Upstream's project goal stands: a high quality, idiomatic Go binding to the
[V8 C++ API](https://v8.github.io/api/head/index.html). License: [LICENSE](LICENSE) (BSD 3-Clause).

V8 Gopher image based on original artwork from the amazing [Renee French](http://reneefrench.blogspot.com).
