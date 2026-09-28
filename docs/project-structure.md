# Definitions Project Structure


The gRPC Service Mesh API convention for definitions projects is to have a `definitions` directory that the generator is
pointed at, with a nested directory for each `deployment_group`. The name `definitions` is a convention; the generator
takes the path of any directory. Each top-level directory within `definitions/` must
have a `transport` defined for it, as in the [example under Options](options.md), if it contains at least one `service`. Likewise,
`deployment_group` is only meaningful in a directory that contains at least one `service`, but it takes a default from
the directory name.

Nested directories within a top-level directory inherit its `deployment_group` and `transport`, and set neither
themselves.

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
├── shop/                      # deployment group "shop"
│   ├── deployment.proto       # option (mesh.transport) = "nats"
│   ├── order.proto            # package shop; OrderService
│   └── internal/
│       └── audit.proto        # package shop.internal; inherits deployment group "shop" and transport "nats"
└── billing/                   # deployment group "billing"
    ├── deployment.proto       # option (mesh.transport) = "http"
    └── invoice.proto          # package billing; InvoiceService
```

## Using a Definitions Project in Service Mesh Applications

A definitions project is a `definitions/` directory, laid out as described at the [top of this page](#definitions-project-structure), plus
the directories the generated code goes into. It can live inside the repositories of the applications that use it as a
git submodule, or as its own repository that exposes the generated code as packages or libraries that the applications
on a service mesh import. Either way, the application will need to include the language-specific library that implements
this specification at runtime, as the generated code will depend on it. See the [current implementation libraries](../README.md#implementations). Each
implementation will document its own usage and how to integrate it into an application.
