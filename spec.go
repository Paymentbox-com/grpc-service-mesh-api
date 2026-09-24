// Package spec carries the proto files this specification ships, embedded so
// the generator can add them to protoc's --proto_path without a checkout.
package spec

import "embed"

// Files holds mesh/options.proto and google/rpc/*.proto at their import paths.
//
//go:embed mesh/options.proto google/rpc/code.proto google/rpc/status.proto google/rpc/error_details.proto
var Files embed.FS

// OptionsProto is the specification's options file, imported by every
// definitions file that sets a mesh option.
const OptionsProto = "mesh/options.proto"

// OptionsGoImport is the compiled Go form of OptionsProto, hosted by this
// module. protoc-gen-go adds a blank import of it to every message file that
// imports OptionsProto, so the definitions project's Go module requires this
// module.
const OptionsGoImport = "github.com/Paymentbox-com/grpc-service-mesh-api/mesh"

// Paths lists every file in Files, as a definitions project imports them.
var Paths = []string{
	"mesh/options.proto",
	"google/rpc/code.proto",
	"google/rpc/status.proto",
	"google/rpc/error_details.proto",
}
