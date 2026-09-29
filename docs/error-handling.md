# Error Handling

Every call through an `RPCClient` may produce a [`MeshError`](runtime-types.md#mesherror). Errors from the Service Mesh API
and from the transport, such as no receiver for a `Target` or a request timeout, are surfaced to the caller unchanged;
this specification does not map them into a `MeshError`.

**Reporting.** A `ROUTE` handler reports an application failure by producing a `MeshError` in place of a response, and a
language-specific implementation should provide an idiomatic way of doing this. The `RPCService` then sends a reply whose
payload is the encoded `google.rpc.Status` and whose metadata carries `Grpc-Status` set to the code as a decimal integer.
A request that does not decode as the method's input type, and a response that does not encode, are reported the same way
as `INTERNAL`, and the handler is not called for a request that does not decode. A `ROUTE` handler that fails in any
other way is reported as `UNKNOWN` with the failure's text as the message.

A `TOPIC` handler has no caller, so it handles application errors itself. Nothing it returns reaches the publisher, and
what an implementation does when one fails, or when a message does not decode as the method's input type, is documented
by that implementation.

**Decoding.** An `RPCClient` reads `Grpc-Status` from the reply metadata before touching the payload. If it is set, it assumes
the payload is a `google.rpc.Status` and a `MeshError` is produced and surfaced accordingly. If it is not set, it assumes the
payload is the method's response type and decodes it normally. A payload that does not decode as the expected type produces
an `INTERNAL` `MeshError`, and so does a request that does not encode.

**Encoding and decoding failures.** Every payload that fails to encode or decode is reported as an `INTERNAL`
`MeshError`, on the serving side and on the calling side, with a message that names the expected message type. The one
exception is a `TOPIC` message that does not decode on the serving side, which has no caller to report to and is handled
as described under Reporting.

**Custom errors.** A consumer may define its own error types as ordinary `.proto` messages in its definitions and attach them
to a `MeshError` through `details`, and a language-specific implementation may provide helper functions or other support
for doing this. `google/rpc/error_details.proto` provides common detail types such as `ErrorInfo`, `BadRequest`, and
`RetryInfo`. Custom errors are defined as protobuf messages so that they can be used across languages.
