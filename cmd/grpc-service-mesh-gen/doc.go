// Command grpc-service-mesh-gen generates the gRPC Service Mesh API code of a
// definitions project: one <dir>.grpcmesh.go and one <dir>_grpcmesh.rb per
// directory that declares a service, plus servicemaps/servicemaps.go and
// service_maps.rb at each language's output root.
//
// Usage:
//
//	grpc-service-mesh-gen --definitions <dir> --out <dir> --lang go,ruby [--verbose]
//	grpc-service-mesh-gen --descriptors <file> --out <dir> --lang go,ruby
//
// Exactly one of --definitions and --descriptors is given.
//
// --definitions <dir> names the definitions directory. Every *.proto under it
// is compiled, with paths relative to it. protoc runs three ways: the message
// code of each requested language (Go with --go_out=<out>/go
// --go_opt=paths=source_relative, Ruby with --ruby_out=<out>/ruby), one
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
// <out>/ruby.
//
// --lang <list> is the comma-separated set of languages, from go and ruby.
//
// --verbose prints each protoc command line and each file written.
//
// Exit status is 0 when every file was written, 1 with one line per error on
// standard error otherwise, and 2 for a usage error.
package main
