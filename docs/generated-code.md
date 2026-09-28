# Generated Code


## Targets

Each `rpc` method becomes one Service Mesh API `Target`, and how the attributes of a `Target` are derived is as follows:

| field      | value                                                                 |
|------------|-----------------------------------------------------------------------|
| `segments` | the proto package split on `.`, then the service name, then the method name |
| `kind`     | `route` for `ROUTE`, `topic` for `TOPIC`                              |
| `metadata` | `deployment_group` from the directory's option or its name, `transport` from the directory's option, and `consumer_group` from the method's option when set |

For the `OrderService` shown under [Options](options.md), in `examples/shop/` with `transport = "nats"`, the two `Targets` would be:

| field      | `Place`                                        | `Placed`                                                              |
|------------|------------------------------------------------|-----------------------------------------------------------------------|
| `segments` | `["shop", "OrderService", "Place"]`            | `["shop", "OrderService", "Placed"]`                                  |
| `kind`     | `route`                                        | `topic`                                                               |
| `metadata` | `deployment_group=shop`, `transport=nats`      | `deployment_group=shop`, `transport=nats`, `consumer_group=audit`     |

`Targets` are built at generation time and emitted as constants. Nothing reads options at runtime.

## ServiceMaps

A `ServiceMap` is simply a list of `Targets`, as defined by the Service Mesh API Specification. The gRPC Service Mesh API
requires one `ServiceMap` per `transport`, each holding only the `Targets` using that `transport`. The transport's map is
what a `Client` for that transport accepts on construction, and a `Runtime` built from that `Client` serves with it.

The per-transport `ServiceMaps` are written to one package at the output root per language. In Go it is
`servicemaps/servicemaps.go`, package `servicemaps`, exporting one `mesh.ServiceMap` var per transport named by
PascalCasing the transport string, such as `servicemaps.Nats`. In Ruby it is `service_maps.rb`, defining
`ServiceMaps::<TRANSPORT>` with the transport string in SCREAMING_SNAKE_CASE, such as `ServiceMaps::NATS`. That package
imports the directory packages for their `Target` values.

## RPCService Types

For each service defined in a `.proto` file, the generator emits an `RPCService` type, which provides a way for each of that service's
`rpc` method handlers to be implemented by application code. It then outputs `Endpoints` and/or `Subscribers` that contain those handlers,
and they are what the transport-specific `Runtime` accepts alongside the transport's `Client`.

The handlers for `Endpoints` and `Subscribers` differ in their return value:

| kind    | handler signature                                                                                                        |
|---------|--------------------------------------------------------------------------------------------------------------------------|
| `ROUTE` | `(context, <RequestType>) -> (<ResponseType>) [May return or raise a MeshError, depending on language/implementation]`   |
| `TOPIC` | `(context, <RequestType>) [May return or raise a MeshError, depending on language/implementation]`                       |

`<RequestType>` and `<ResponseType>` are the compiled message types for each `rpc` method.

An application implements only the method handlers it intends to serve. Decoding the inbound Service Mesh API `Message`
into the proper type, and encoding a response type back into a `Message` is handled automatically so that the
application doesn't need to touch the underlying layer.

An `RPCService` type keeps the name of the service it is generated from, such as `OrderService`. A root file may
re-export it under a prefixed name, as described under [Root Package](#root-package), and the type itself keeps its name.

## RPCClient Types

For each service the generator also emits an `RPCClient` type with one method per `rpc` method, through which
callers reach the service. These methods have different method signatures based on the `Kind` they are defined
with.

| kind    | client method signature                                                                                      | Service Mesh API operation |
|---------|--------------------------------------------------------------------------------------------------------------|----------------------------|
| `ROUTE` | `(context, Request) -> (Response) [May return or raise a MeshError, depending on language/implementation]`   | `Request`                  |
| `TOPIC` | `(context, Request) [May return or raise a MeshError, depending on language/implementation]`                 | `Publish`                  |

A `ROUTE` or `TOPIC` method encodes the standard, generated type into a `Message` addressed to the method's `Target` and sends that
message through the appropriate transport-specific client that implements the Service Mesh API specification. A `ROUTE`
method will get a `Message` back and decode it back into the standard, generated response type, then return that type.

An `RPCClient` holds no connection of its own. On each call it must resolve the transport-specific `Client` that
serves the `Target`'s transport using the [`TransportRouter`](runtime-types.md#transportrouter).

A client type's name is the service's name with a trailing `Service` removed, if it has one, and `Client` appended:
`OrderService` gives `OrderClient`. The generated code also holds each service's `Targets` under the same base name
with `Targets` appended, such as `OrderTargets`; the generated clients and services use them, and applications do not
need to.

## Generated Packages

Mesh code goes into the same package as the directory's message code. In Go that is the directory's `go_package`, so
every file under `definitions` sets `go_package` when Go is generated, and every file in one directory sets the same
value. In Ruby it is `ruby_package` when set, and otherwise the module `protoc` derives from the proto package: each
dot-separated part capitalized and nested, so `shop.internal` is `Shop::Internal`.

## Root Package

The `--go-root-package <import path[;name]>` and `--ruby-root-module <Module>` options specify a root package name
that re-exports every generated message, enum, service, client, and targets value under one namespace, so an
application imports one package for convenience.

```sh
grpc-service-mesh-gen --definitions definitions --go_out=lib/go --ruby_out=lib/ruby \
    --go-root-package "example.com/definitions;definitions" --ruby-root-module Definitions
```

```
lib/go/definitions.grpcmesh.go      type Order = shop.Order, var OrderClient = shop.OrderClient, ...
lib/ruby/definitions_grpcmesh.rb    Definitions::Order = ::Shop::Order, Definitions::OrderClient = ::Shop::OrderClient, ...
```

A re-exported name is the name its directory's package gives it, so two directories that generate the same name
collide and the generator stops with an error. `option (mesh.root_prefix)`, set in one file of a directory, prepends a
prefix to every name that directory adds to the root files, and the directory's own package keeps its names. With
`option (mesh.root_prefix) = "Billing";` in `definitions/billing/deployment.proto`, billing's `Order` is
`definitions.BillingOrder` and `Definitions::BillingOrder` in the root files, while shop's stays `definitions.Order`.
