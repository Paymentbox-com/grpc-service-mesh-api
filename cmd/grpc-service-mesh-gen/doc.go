// Command grpc-service-mesh-gen generates the gRPC Service Mesh API code of a
// definitions project: one <dir>.grpcmesh.go and one <dir>_grpcmesh.rb per
// directory that declares a service, plus servicemaps/servicemaps.go and
// service_maps.rb at each language's output root, and, when asked, one root
// package per language that aliases every generated identifier.
//
// Usage:
//
//	grpc-service-mesh-gen --definitions <dir> --out <dir> --lang go,ruby [--verbose]
//	grpc-service-mesh-gen --descriptors <file> --out <dir> --lang go,ruby
//	grpc-service-mesh-gen --definitions <dir> --go-out <dir> --ruby-out <dir> --lang go,ruby
//	grpc-service-mesh-gen --definitions <dir> --out <dir> --lang go,ruby \
//	    --go-root-package <import path[;name]> --ruby-root-module <Module>
//
// Exactly one of --definitions and --descriptors is given.
//
// --definitions <dir> names the definitions directory. Every *.proto under it
// is compiled, with paths relative to it. protoc runs three ways: the message
// code of each requested language (Go with --go_out=<go out>
// --go_opt=paths=source_relative, Ruby with --ruby_out=<ruby out>), one
// FileDescriptorSet of the whole directory with --include_imports
// --include_source_info, and this generator over that set. Embedded copies of
// mesh/options.proto and google/rpc/*.proto are added as a second
// --proto_path, so a project need not vendor them; when it does, those four
// files are left out of the message runs. protoc and protoc-gen-go are found
// on PATH.
//
// --descriptors <file> names a FileDescriptorSet written by protoc with
// --include_imports, in place of --definitions. No protoc run happens.
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
// Exit status is 0 when every file was written, 1 with one line per error on
// standard error otherwise, and 2 for a usage error.
package main
