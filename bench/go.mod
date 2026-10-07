module github.com/botify-labs/v8go/bench

go 1.27

require (
	github.com/botify-labs/v8go v0.0.0-00010101000000-000000000000
	rogchap.com/v8go v0.6.0
)

require (
	github.com/botify-labs/v8go/deps/darwin_amd64 v0.0.0-20261007211632-a0cb852caccf // indirect
	github.com/botify-labs/v8go/deps/darwin_arm64 v0.0.0-20261007211632-a0cb852caccf // indirect
	github.com/botify-labs/v8go/deps/linux_amd64 v0.0.0-20261007211632-a0cb852caccf // indirect
	github.com/botify-labs/v8go/deps/linux_arm64 v0.0.0-20261007211632-a0cb852caccf // indirect
	github.com/botify-labs/v8go/deps/windows_amd64 v0.0.0-20261007211632-a0cb852caccf // indirect
)

replace (
	github.com/botify-labs/v8go => ../
	github.com/botify-labs/v8go/deps/darwin_amd64 => ../deps/darwin_amd64
	github.com/botify-labs/v8go/deps/darwin_arm64 => ../deps/darwin_arm64
	github.com/botify-labs/v8go/deps/linux_amd64 => ../deps/linux_amd64
	github.com/botify-labs/v8go/deps/linux_arm64 => ../deps/linux_arm64
	github.com/botify-labs/v8go/deps/windows_amd64 => ../deps/windows_amd64
	rogchap.com/v8go => ../../v8go-baseline
)
