# Definitions Project Structure


The gRPC Service Mesh API convention for definitions projects is to have a `definitions` directory that the generator is
pointed at, with a top-level directory for each area of the service mesh, such as `shop` or `billing`. The name
`definitions` is a convention; the generator takes the path of any directory. A top-level directory is a namespace. It
names the proto package of its services, and sets the `transport` they are served over, as in the
[example under Options](options.md), if it contains at least one `service`. It does not decide which deployment group
serves them, which is the configuration of each application's `RPCRuntime`.

Nested directories within a top-level directory inherit its `transport`, and do not set it themselves.

A directory that declares no `service`, such as one holding message types several deployment groups share, is valid
and needs no `transport`. Its messages are generated as usual, and it gets no mesh file.

Every file in one directory belongs to one generated package. When Go is generated, every file sets `go_package`, and
every file in a directory sets the same value.

Exactly one file in each top-level directory sets `transport`. The value is the name the application configures in the
[`TransportRouter`](runtime-types.md#transportrouter) for that transport, for example `nats` or `http`. The generator copies the string
into the generated code without interpreting it, and the `TransportRouter` resolves it at runtime against the configured
transports.

```
definitions/
├── shop/                      # the shop namespace
│   ├── deployment.proto       # option (mesh.transport) = "nats"
│   ├── order.proto            # package shop; OrderService
│   └── internal/
│       └── audit.proto        # package shop.internal; inherits transport "nats"
├── billing/                   # the billing namespace
│   ├── deployment.proto       # option (mesh.transport) = "http"
│   └── invoice.proto          # package billing; InvoiceService
└── events/                    # topics many applications subscribe to
    ├── deployment.proto       # option (mesh.transport) = "nats"
    └── order_events.proto     # package events; OrderEvents, TOPIC methods only
```

## Events Directory

A suggested convention is to keep the topics that many applications subscribe to in their own top-level directory, such
as `events/`, apart from the services that own the data. Its services declare only `TOPIC` methods, and set no
`consumer_group`, because each subscribing application chooses its own.

```proto
// definitions/events/order_events.proto
syntax = "proto3";
package events;
import "mesh/options.proto";
import "google/protobuf/empty.proto";
import "shop/order.proto";

service OrderEvents {
  rpc Placed(shop.Order) returns (google.protobuf.Empty) {
    option (mesh.kind) = TOPIC;
  }
}
```

Every application uses the same generated `OrderEvents` service and client. An application that reacts to the event
implements the generated service, registers it, and serves it from its own `RPCRuntime`, so its subscribers join its own
deployment group, and each application receives every event once. Any application publishes through the generated client,
to the one channel every subscriber listens on.

## Using a Definitions Project in Service Mesh Applications

A definitions project is a `definitions/` directory, laid out as described at the [top of this page](#definitions-project-structure), plus
the directories the generated code goes into. It can live inside the repositories of the applications that use it as a
git submodule, or as its own repository that exposes the generated code as packages or libraries that the applications
on a service mesh import. Either way, the application will need to include the language-specific library that implements
this specification at runtime, as the generated code will depend on it. See the [current implementation libraries](../README.md#implementations). Each
implementation will document its own usage and how to integrate it into an application.
