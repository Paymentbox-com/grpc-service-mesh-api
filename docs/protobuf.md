# Protobuf


The only inputs required for the gRPC Service Mesh API are `.proto` files and the descriptors `protoc` produces from
them. The generator, a Go program, reads those descriptors to write the service mesh code for every language, and the
standard `protoc` plugin or built-in generator for each language writes the message types. The standard elements of a
`.proto` file are translated into Service Mesh API primitives as described below. In addition, a few [options](options.md) are defined to further control how the `.proto` files are
interpreted.

## Protobuf Messages

The standard `protoc` compilers produce message types for each protobuf message type defined in the `.proto` files
they are aimed at. These generated types normally implement their own binary serialization, which is used as the
`Payload` of a Service Mesh API `Message`. Conversion of the generated types to and from Service Mesh API `Message` objects
is handled behind the scenes by the `RPCClient` and `RPCService` types described under [Generated Code](generated-code.md), so that the generated methods
on `RPCClient` types and the `Endpoint` and `Subscriber` handlers held by `RPCService` types only deal with the generated
types.

A Service Mesh API `Message` created from a serialized generated type carries `Content-Type: application/x-protobuf` in
its metadata.

## Protobuf Services

Protobuf services defined in `.proto` files have their `rpc` methods translated into Service Mesh API `Targets`, and the
services themselves are compiled into both an `RPCClient` type and an `RPCService` type.

The `Targets` of every service are compiled into one `ServiceMap` per `transport`, holding a `Target` for every `rpc`
method served over that transport.

An `RPCClient` type holds a client method for each `rpc` method in the proto definition, and an `RPCService` either
accepts or stubs out a handler for each `rpc` method in the proto definition. Each handler is either an `Endpoint` or a
`Subscriber` handler, depending on the `Kind` option set on the method in its proto definition (the default is `ROUTE`).

A method whose `Kind` is `TOPIC` has no reply, but protobuf requires every `rpc` to declare a return type, so a topic
method declares one anyway, conventionally `google.protobuf.Empty` which ships inside `protoc` and is imported with
`import "google/protobuf/empty.proto"`. That type, or any other type that is returned from a method with `TOPIC` as its
`Kind`, is ignored everywhere. A publish sends only the request message without looking or waiting for a reply, the
subscriber's handler returns nothing, and the generated client method returns no value.
