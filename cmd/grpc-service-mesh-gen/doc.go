// Command grpc-service-mesh-gen generates the gRPC Service Mesh API code of a
// definitions project: one <dir>.grpcmesh.go and one <dir>_grpcmesh.rb per
// directory that declares a service, plus servicemaps/servicemaps.go and
// service_maps.rb at each language's output root, and, when asked, one root
// package per language that aliases every generated identifier.
//
// Usage:
//
//	grpc-service-mesh-gen --definitions <dir> -I <dir> --out <dir> --lang go,ruby [--verbose]
//	grpc-service-mesh-gen --definitions <dir> -I <dir> --go-out <dir> --ruby-out <dir> --lang go,ruby
//	grpc-service-mesh-gen --definitions <dir> -I <dir> --out <dir> --lang go,ruby \
//	    --go-root-package <import path[;name]> --ruby-root-module <Module>
//
// For example:
//
//	grpc-service-mesh-gen --definitions definitions \
//	    -I "$(go list -m -f '{{.Dir}}' github.com/Paymentbox-com/grpc-service-mesh-go)/proto" \
//	    --out lib --lang go,ruby
//
// --definitions <dir> names the definitions directory and is required.
// Every *.proto under it is compiled, with paths relative to it. protoc runs
// three ways: the message code of each requested language (Go with
// --go_out=<go out> --go_opt=paths=source_relative, Ruby with
// --ruby_out=<ruby out>), one FileDescriptorSet of the whole directory with
// --include_imports --include_source_info, and this generator over that set.
// The definitions directory is the first --proto_path of every run. The
// message runs list the definitions files, leaving out any copy of
// mesh/options.proto and google/rpc/*.proto, and write what plain protoc
// writes for them. With go in --lang, every definitions file sets go_package.
// protoc and protoc-gen-go are found on PATH. A tree that declares no service
// is an error.
//
// -I <dir>, also spelled --proto_path <dir> or --proto_path=<dir>, adds an
// include directory to every protoc run after the definitions directory, in
// the order given, and repeats. The generator and plain protoc read the same
// include paths. The library a project depends on supplies the
// specification's protos, mesh/options.proto and google/rpc/*.proto, under
// its proto/ directory at the version its compiled options were built from:
// "$(go list -m -f '{{.Dir}}' github.com/Paymentbox-com/grpc-service-mesh-go)/proto"
// for grpc-service-mesh-go and "$(bundle info --path grpc_service_mesh)/proto"
// for the grpc_service_mesh gem. When neither the definitions directory nor
// any include directory holds mesh/options.proto, the generator stops with an
// error naming both before running protoc.
//
// --out <dir> is the output root; generated code goes to <out>/go and
// <out>/ruby. It is required unless every requested language has its own
// output root.
//
// --go-out <dir> and --ruby-out <dir> are the Go and Ruby output roots, in
// place of <out>/go and <out>/ruby. Each needs its language in --lang.
//
// --go-root-package <import path[;name]> also writes <go out>/<name>.grpcmesh.go,
// package <name>, aliasing every generated Go identifier of the definitions
// tree: each message and enum type, each enum value constant, and each
// RPCService type, client value, and targets value. <name> is the last
// element of the import path unless given after ";", as in go_package. Every
// aliased identifier is unique across the tree, and no directory package
// shares the root package's name. Needs go in --lang.
//
// --ruby-root-module <Module> also writes <ruby out>/<snake_case(Module)>_grpcmesh.rb,
// which requires every generated Ruby file and defines module <Module> with
// a constant for every top-level message and enum, each RPCService, client,
// and targets constant, and ServiceMaps. Every aliased constant is unique
// across the tree, and no generated module shares the root module's name.
// Needs ruby in --lang.
//
// --lang <list> is the comma-separated set of languages, from go and ruby.
//
// --verbose prints each protoc command line and each file written.
//
// Exit status is 0 when every file was written, 1 with the error on standard
// error otherwise, and 2 for a usage error.
package main
