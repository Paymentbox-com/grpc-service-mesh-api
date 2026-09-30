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

Registering a service can override the `consumer_group` of any of its `Endpoints` and `Subscribers`, by `Target`. The
override replaces the `consumer_group` the generated code carries from the method's option. An override of `""` removes
it, so the runtime's `deployment_group` applies, and `none` means no group. When one `Target` is given more than once,
the last value wins. A `Target` that is not one of the service's own is a mistake, and registering fails at boot: an
implementation panics or raises, as it documents.

A `consumer_group` is resolved from, in order:

1. The override given when the service was registered.
2. The `consumer_group` option on the method in the definitions.
3. The `deployment_group` of the `RPCRuntime` that serves it.

## RPCRuntime

The `RPCRuntime` is what a service mesh application calls to begin listening for messages, and it is a wrapper around a
transport-specific Service Mesh API `Runtime` implementation. It should either wrap or expose the underlying implementation's
functionality.

An `RPCRuntime` is constructed for a single `transport` and a single `deployment_group`. It accepts a constructor
function for building the transport-specific `Runtime` and a `Hash<String, String>` configuration object. It takes the
transport's `Client` from the `TransportRouter`, and every `Endpoint` and `Subscriber` in the `Registry` whose `Target`'s
`transport` is its own. The others are left for the `RPCRuntime` of their own transport, so a process registers everything
it serves at boot, whatever the transport. The `deployment_group` is the `RPCRuntime`'s own configuration, and nothing in
the definitions or the `Registry` narrows it.

A process has one `Registry` and one `RPCRuntime` per transport it serves, all under the process's `deployment_group`.
A registered service whose transport has no `RPCRuntime` in the process is not served. Each `ROUTE` service is served by
one deployment group, because two groups serving one `ROUTE` method would both reply.

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
