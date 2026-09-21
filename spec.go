// Package spec carries the proto files this specification ships, embedded so
// the generator can add them to protoc's --proto_path without a checkout.
package spec

import "embed"

// Files holds mesh/options.proto and google/rpc/*.proto at their import paths.
//
//go:embed mesh/options.proto google/rpc/code.proto google/rpc/status.proto google/rpc/error_details.proto
var Files embed.FS

// Paths lists every file in Files, as a definitions project imports them.
var Paths = []string{
	"mesh/options.proto",
	"google/rpc/code.proto",
	"google/rpc/status.proto",
	"google/rpc/error_details.proto",
}
