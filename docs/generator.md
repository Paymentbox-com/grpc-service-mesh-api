# Generator


The generator is the Go program `grpc-service-mesh-gen` in this repository:

```sh
go install github.com/Paymentbox-com/grpc-service-mesh-api/cmd/grpc-service-mesh-gen@v0.7.0
grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby
```

It compiles each language's message code with `protoc`, then writes the mesh code. Each directory that declares a
`service` gets one mesh file per language beside its message code, and each language's output root gets the service
maps. Every generated file starts with a `DO NOT EDIT` header and is overwritten on each run. Generated code depends on
the language library and the Service Mesh API contract, and never on a transport. The tools it runs are listed under
[Prerequisites](#prerequisites).

| flag | meaning |
|---|---|
| `--definitions <dir>` | the definitions directory |
| `--go_out=<dir>`, `--ruby_out=<dir>` | a language's output directory, as `protoc` takes it; each one given requests that language |
| `-I <dir>` | an extra import directory, repeatable; also spelled `--proto_path` |
| `--mesh-only` | write only the mesh code and skip the message code |
| `--go-root-package <path[;name]>`, `--ruby-root-module <Module>` | add a root file, described under [Root Package](generated-code.md#root-package) |
| `--verbose` | print each `protoc` command and each file written |

## Import Path

Every `protoc` run imports from `definitions` first, then the specification directory, then each `-I` directory in
order, and compiles only the files under `definitions`. The specification directory is this repository's module at the
generator's own version, taken from the Go module cache, and holds `mesh/options.proto`. `grpc-service-mesh-gen
proto-path` prints it. A development build run from a checkout of this repository uses the checkout. A project whose
files import `google/rpc/*.proto` adds `-I` for a checkout of `github.com/googleapis/googleapis`.

## Compiling Message Code with Plain protoc

The generator's message code is exactly what plain `protoc` writes, so a project can compile its message code itself
and run the generator with `--mesh-only` for the rest:

```sh
spec="$(grpc-service-mesh-gen proto-path)"
protoc -I definitions -I "$spec" --go_out=lib/go --go_opt=paths=source_relative $(find definitions -name '*.proto')
protoc -I definitions -I "$spec" --ruby_out=lib/ruby $(find definitions -name '*.proto')
grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby --mesh-only
```

The commands list only the files under `definitions`, because the compiled form of `mesh/options.proto` comes from
the language's implementation of this specification, as described under [Options](options.md).

## Generator Errors

The generator stops with an error when:

* the definitions tree declares no `service`.
* a `.proto` file declares no `package`.
* a top-level directory with a `service` sets `transport` in no file or in more than one.
* a top-level directory sets `deployment_group` to two different values.
* a nested directory sets `transport` or `deployment_group`.
* a file at the definitions root declares a `service` or sets `transport` or `deployment_group`.
* an `rpc` method uses `stream` on its request or its response.
* Go is requested and a file sets no `go_package`, which `protoc-gen-go` requires.
* two files in one directory set different `go_package` values; the directory's mesh file lives in one package.
* a root package is requested and two files generate the same name.
* the Go root package's name equals a directory package's name, or the Ruby root module equals a generated module or
  `ServiceMaps`.
* two files in one directory set `root_prefix`, or its value is not an identifier starting with an uppercase letter.
* Ruby is requested and a message an `rpc` method takes or returns declares a field named `mesh_metadata`, which the
  Ruby message metadata accessor hides.

## Using the Generator

### Prerequisites

| tool | needed for | install |
|---|---|---|
| Go 1.26 or newer | installing and running the generator | [go.dev/dl](https://go.dev/dl) |
| `protoc` 3.15 or newer | every run | macOS: `brew install protobuf`. Linux: `apt install protobuf-compiler` or `dnf install protobuf-compiler`. Any platform, including Windows: unzip `protoc-<version>-<os>.zip` from [protocolbuffers/protobuf releases](https://github.com/protocolbuffers/protobuf/releases) and put its `bin/` on `PATH`. |
| `protoc-gen-go` | generating Go | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |

Ruby needs no plugin; Ruby support is built into `protoc` itself.

### Installing and Running

```sh
go install github.com/Paymentbox-com/grpc-service-mesh-api/cmd/grpc-service-mesh-gen@v0.7.0
grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby
```

`go run github.com/Paymentbox-com/grpc-service-mesh-api/cmd/grpc-service-mesh-gen@v0.7.0` runs it without installing.
The flags are described at the [top of this page](#generator).

Each output directory mirrors the definitions tree. For `definitions/shop/order.proto` the generator writes:

```
lib/go/shop/order.pb.go             message code
lib/go/shop/shop.grpcmesh.go        OrderTargets, OrderService, OrderClient
lib/go/servicemaps/servicemaps.go   one ServiceMap per transport
lib/ruby/shop/order_pb.rb           message code
lib/ruby/shop/shop_grpcmesh.rb      Shop::OrderTargets, Shop::OrderService, Shop::OrderClient
lib/ruby/service_maps.rb            one ServiceMap per transport
```

### Regenerating

The generator overwrites the files it writes but doesn't delete anything if it becomes orphaned. A project's own automation
can delete its generated files before each run, so that the output of a renamed or removed `.proto` file does not linger.
Every generated file begins with a `DO NOT EDIT` header, which makes them easy to find:

```sh
grep -rl --include='*.go' --include='*.rb' 'DO NOT EDIT' lib | xargs rm -f
grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby
```

Generated code should be committed, so the applications that use it need neither `protoc` nor the generator at runtime.
