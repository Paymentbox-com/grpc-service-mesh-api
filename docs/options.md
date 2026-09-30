# Options


The gRPC Service Mesh API uses protobuf's `Options` feature to specify details about the services defined in `.proto`
files that cannot be specified by the protobuf syntax otherwise. These details include the `transport` intended to be used
for any given set of services, the `Kind` a `Target` and `Endpoint`/`Subscriber` generated from a service `rpc` method
should be, and a default `consumer_group` for any given `rpc` method. Deployment groups are not set in definitions. A
`deployment_group` is the configuration of the `RPCRuntime` that serves the services, as described under
[RPCRuntime](runtime-types.md#rpcruntime).

These options are defined in `mesh/options.proto` in this repository, and are as follows:

```proto
syntax = "proto3";
package mesh;
import "google/protobuf/descriptor.proto";

option go_package = "github.com/Paymentbox-com/grpc-service-mesh-go/meshoptions";

enum Kind {
  ROUTE = 0;
  TOPIC = 1;
}

extend google.protobuf.MethodOptions {
  Kind kind = 50001;
  string consumer_group = 50002;
}

extend google.protobuf.FileOptions {
  string transport = 50004;
  string root_prefix = 50005;
}
```

| option             | set on   | meaning                                                                                 | default                |
|--------------------|----------|-----------------------------------------------------------------------------------------|------------------------|
| `kind`             | a method | `ROUTE` replies to each request; `TOPIC` does not reply                                 | `ROUTE`                |
| `consumer_group`   | a method | the default `consumer_group` of the `Endpoint` or `Subscriber` generated from it, which an application can override when it registers the service | the `deployment_group` of the runtime that serves it |
| `transport`        | a file   | the transport the file's top-level directory is served over, as a string a library resolves at runtime; this specification does not enumerate transports | none; required |
| `root_prefix`      | a file   | a prefix for every name the file's directory adds to a root package or module (see [Root Package](generated-code.md#root-package)); a nested directory without its own uses its top-level directory's | none |

No option is defined at the service or method level for `transport`, so a service cannot be split across different
transports, which is by design. The file options are intended to be set in their own file within the top-level directory
to which they apply, such as `deployment.proto`.

Importing and using these options in your own `.proto` files is done like this. The examples also set `go_package`,
which every definitions file sets when Go code is generated from it.

```proto
// definitions/shop/deployment.proto: the directory's settings and nothing else
syntax = "proto3";
package shop;
import "mesh/options.proto";

option go_package = "example.com/definitions/shop";
option (mesh.transport) = "nats";
```

```proto
// definitions/shop/order.proto
syntax = "proto3";
package shop;
import "mesh/options.proto";
import "google/protobuf/empty.proto";

option go_package = "example.com/definitions/shop";

message Order {
  optional string id = 1;
  optional string item = 2;
}

service OrderService {
  rpc Place(Order) returns (Order);                          // ROUTE by default
  rpc Placed(Order) returns (google.protobuf.Empty) {
    option (mesh.kind) = TOPIC;
    option (mesh.consumer_group) = "audit";
  }
}
```

`mesh/options.proto` belongs to this specification and lives in this repository. A definitions project imports it by
that path in its own `.proto` files. Its compiled forms ship with each language-specific implementation of this
specification. The generator puts the directory of this repository at its own version on the import path of every
`protoc` run, and `grpc-service-mesh-gen proto-path` prints that directory for plain `protoc` runs, as described under
[Generator](generator.md#import-path).

Until version 1.0, `mesh/options.proto` may change or remove options in a minor version. The latest version of this
specification and the generator always holds the latest version of the options it supports.

This specification uses certain proto definitions from `github.com/googleapis/googleapis`. Those definitions belong to
[googleapis](https://github.com/googleapis/googleapis), and their compiled forms are included in the language-specific
implementations of this specification. In the rare case when the definitions themselves need to be imported in a
definitions project, their path inside a checkout of `github.com/googleapis/googleapis` can be passed to `protoc` with
`-I`.
