// Command grpc-service-mesh-gen generates the gRPC Service Mesh API code of a
// definitions project: one <dir>.grpcmesh.go and one <dir>_grpcmesh.rb per
// directory that declares a service, plus servicemaps/servicemaps.go and
// service_maps.rb at each language's output root, and, when asked, one root
// package per language that aliases every generated identifier.
//
// Usage:
//
//	grpc-service-mesh-gen --definitions <dir> [--go_out=<dir>] [--ruby_out=<dir>] [-I <dir>] [--mesh-only] [--verbose]
//	grpc-service-mesh-gen --definitions <dir> --go_out=<dir> --ruby_out=<dir> \
//	    --go-root-package <import path[;name]> --ruby-root-module <Module>
//	grpc-service-mesh-gen proto-path
//
// At least one of --go_out and --ruby_out is given. For example:
//
//	grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby
//
// proto-path prints the specification directory, the directory holding
// mesh/options.proto at this generator's version, and exits 0, or prints the
// error and exits 1. A release build, installed or run at a version such as
// @v0.5.0, takes the directory of that version of
// github.com/Paymentbox-com/grpc-service-mesh-api from the Go module cache
// with go mod download -json. A development build, whose version is (devel)
// or ends in +dirty, takes the root of the working directory's module from
// go list -m when that module is github.com/Paymentbox-com/grpc-service-mesh-api,
// and fails otherwise. go is found on PATH.
//
// --definitions <dir> names the definitions directory and is required.
// Every *.proto under it is compiled, with paths relative to it. protoc runs
// three ways: the message code of each requested language (Go with
// --go_out=<dir> --go_opt=paths=source_relative, Ruby with
// --ruby_out=<dir>), one FileDescriptorSet of the whole directory with
// --include_imports --include_source_info, and this generator over that set.
// Every run's proto path is the definitions directory, then the
// specification directory, then each -I directory. The message runs list the
// definitions files, leaving out any copy of mesh/options.proto and
// google/rpc/*.proto, and write what plain protoc writes for them. With
// --go_out, every definitions file sets go_package. protoc and protoc-gen-go
// are found on PATH. A tree that declares no service is an error.
//
// -I <dir>, also spelled --proto_path <dir> or --proto_path=<dir>, adds an
// include directory to every protoc run after the specification directory,
// in the order given, and repeats. A project whose files import
// google/rpc/*.proto passes a checkout of github.com/googleapis/googleapis.
// When the specification directory cannot be resolved and neither the
// definitions directory nor any include directory holds mesh/options.proto,
// the generator stops before running protoc with an error saying why
// resolving failed.
//
// --mesh-only runs only the FileDescriptorSet protoc run and writes only the
// mesh code: the per-directory files, the ServiceMaps, and the root files.
//
// --go_out=<dir> and --ruby_out=<dir>, also spelled --go_out <dir> and
// --ruby_out <dir>, each request that language and name its output
// directory, the directory protoc's --go_out or --ruby_out takes. Each
// definitions directory <path> that declares a service gets
// <dir>/<path>/<name>.grpcmesh.go or <dir>/<path>/<name>_grpcmesh.rb, where
// <name> is the last element of <path>, and the per-transport ServiceMaps go
// to servicemaps/servicemaps.go and service_maps.rb in the output directory.
//
// --go-root-package <import path[;name]> also writes <name>.grpcmesh.go in
// the --go_out directory, package <name>, aliasing every generated Go identifier of the definitions
// tree: each message and enum type, each enum value constant, and each
// RPCService type, client value, and targets value. <name> is the last
// element of the import path unless given after ";", as in go_package. Every
// aliased identifier is unique across the tree, and no directory package
// shares the root package's name. Needs --go_out.
//
// --ruby-root-module <Module> also writes <snake_case(Module)>_grpcmesh.rb in
// the --ruby_out directory, which requires every generated Ruby file and defines module <Module> with
// a constant for every top-level message and enum, each RPCService, client,
// and targets constant, and ServiceMaps. Every aliased constant is unique
// across the tree, and no generated module shares the root module's name.
// Needs --ruby_out.
//
// --verbose prints each protoc command line and each file written.
//
// Exit status is 0 when every file was written, 1 with the error on standard
// error otherwise, and 2 for a usage error.
package main
