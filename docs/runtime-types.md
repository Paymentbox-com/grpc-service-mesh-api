# Runtime Types


Each language-specific implementation of this specification will need to implement the following types, which are not
generated off of `.proto` files.

## TransportRouter

The `TransportRouter` contains, for each `transport` name the definitions use, the transport-specific `Client` for that transport.
It is a single, process-wide object, and the application process is responsible for configuring it at boot. Generated
code for `RPCService` types and `RPCClient` types whose `transport` has no `Client` in the router fails at runtime. Nothing
generated takes the `TransportRouter` as an argument; a generated client is called directly and takes the `Client` for its
`Target`'s transport from the process's router on each call. The router's `Close` closes every `Client` it holds, and a
process that only calls runs it before exit so the transport implementation closes appropriately. Each language-specific
implementation of this specification documents how the `TransportRouter` is configured, and how clients are
retrieved from it using the `transport` set for each directory in a definitions project.

## Registry

The `Registry` is a single, process-wide object that collects every implemented `Endpoint` and `Subscriber` a process
will serve. The application is responsible for registering its implemented `RPCService` values in it at boot, and nothing
generated takes the `Registry` as an argument. This specification defines what it does; each language-specific implementation
of this specification documents how services are registered.

## RPCRuntime

The `RPCRuntime` is what a service mesh application calls to begin listening for messages, and it is a wrapper around a
transport-specific Service Mesh API `Runtime` implementation. It should either wrap or expose the underlying implementation's
functionality.

An `RPCRuntime` is constructed for a single `transport` and a single `deployment_group`. It accepts a constructor
function for building the transport-specific `Runtime` and a `Hash<String, String>` configuration object. It takes the
transport's `Client` from the `TransportRouter`. By default it takes every implemented `Endpoint` and `Subscriber`
whose `Target` carries its `deployment_group` from the `Registry`, and it also accepts a list of `Endpoints` and a list
of `Subscribers` in place of those, so a process may serve only part of a deployment group if needed. Each list given
replaces the `Registry`'s list of that kind, and the other still comes from the `Registry`. Every `Target` in a given
list carries the `RPCRuntime`'s `deployment_group` and `transport`, and one that does not is an error.

The `Runtime` constructor provided must accept four arguments in this order: the transport's `Client`, a `Hash<String, String>`
configuration object, a list of `Endpoints`, and a list of `Subscribers`. It returns a Service Mesh API `Runtime` for the given
`transport` serving those `Endpoints` and `Subscribers`. The `RPCRuntime` passes the `Client` it looks up from the `TransportRouter`,
the configuration it was given with its `deployment_group` set, and the `Endpoints` and `Subscribers` it serves.

## MeshError

`MeshError` is the error type every implementation provides for application failures. An `RPCService` handler
produces one to report a failure, and an `RPCClient` method produces one when the reply carries a failure. It wraps the
standard `google.rpc.Status` message and adds what the language needs for the calling code to treat it as an error.

| member       | meaning                                                                                  |
|--------------|------------------------------------------------------------------------------------------|
| `code`       | a `google.rpc.Code`                                                                      |
| `message`    | human-readable text                                                                      |
| `details`    | the wrapped message's `details`, any number of packed messages of any type               |
| `proto`      | the wrapped `google.rpc.Status`                                                          |

A `MeshError` is constructed from a code, a message, and zero or more detail messages, or from a
`google.rpc.Status` message. It is the language's error type: in Go it implements `Error`, in Ruby it descends from
`StandardError`, and so on. Each language-specific implementation documents its language-specific features and construction.

The compiled classes for `google.rpc.Status`, `google.rpc.Code`, and the detail types in `google/rpc/error_details.proto`
are a dependency of each implementation, taken from the standard published packages for the language. Their `.proto`
sources are in `github.com/googleapis/googleapis`, which a definitions project whose files import them puts on its
import path with `-I`, as described under [Options](options.md).
