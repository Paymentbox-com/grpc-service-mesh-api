# gRPC Service Mesh API

The gRPC Service Mesh API is a service mesh protocol layer specification built on the
[Service Mesh API Specification](https://github.com/Paymentbox-com/service-mesh-api) and driven by Google Protocol
Buffers. It specifies how protobuf message and service definitions are translated into Service Mesh API primitives and
consumed at the application level.

Using this specification, libraries can be implemented across programming languages to compile the same
protobuf definitions into code that is compatible with the Service Mesh API and compatible with each other across
transports and language- and transport-specific runtimes.

![ServiceMeshInterface.drawio.png](ServiceMeshInterface.drawio.png)

## Overview

The gRPC Service Mesh API uses a generator, which reads `protoc` output to generate the necessary code to consume the
Service Mesh API. This specification defines how that generator interprets `.proto` files to generate that code. Currently
it supports Ruby and Go.

## Documentation

For writing and generating definitions:

- [Protobuf](docs/protobuf.md): how `.proto` messages and services map to Service Mesh API primitives
- [Options](docs/options.md): `mesh/options.proto` and how definitions use it
- [Definitions Project Structure](docs/project-structure.md): how a definitions project is laid out and shared
- [Generator](docs/generator.md): installing and running `grpc-service-mesh-gen`, its flags, and its errors

For implementing this specification in a language:

- [Generated Code](docs/generated-code.md): the Targets, ServiceMaps, RPCService, and RPCClient types the generator writes
- [Runtime Types](docs/runtime-types.md): TransportRouter, Registry, RPCRuntime, and MeshError
- [Encoding and Decoding](docs/encoding.md): how generated types become Service Mesh API messages
- [Error Handling](docs/error-handling.md): how failures travel between handlers and callers

## Implementations

- [grpc-service-mesh-go](https://github.com/Paymentbox-com/grpc-service-mesh-go): the Go library, package `grpcmesh`.
- [grpc-service-mesh-ruby](https://github.com/Paymentbox-com/grpc-service-mesh-ruby): the Ruby library, gem `grpc_service_mesh`.
