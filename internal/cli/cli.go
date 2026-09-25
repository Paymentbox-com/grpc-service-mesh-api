// Package cli is grpc-service-mesh-gen's command line.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/gen"
	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/protoc"
	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/specdir"
)

// Usage is the --help text.
const Usage = `grpc-service-mesh-gen generates the gRPC Service Mesh API code for a
definitions project.

Usage:
  grpc-service-mesh-gen --definitions <dir> [--go_out=<dir>] [--ruby_out=<dir>] [-I <dir>] [--mesh-only] [--verbose]
  grpc-service-mesh-gen proto-path

At least one of --go_out and --ruby_out is given.

Commands:
  proto-path
      Print the specification directory, the directory holding
      mesh/options.proto at this generator's version, and exit 0. On
      failure, print the error and exit 1. Plain protoc takes it as
        -I "$(grpc-service-mesh-gen proto-path)"
      A release build, one installed or run at a version such as
      github.com/Paymentbox-com/grpc-service-mesh-api/cmd/grpc-service-mesh-gen@v0.5.1,
      takes the directory of that module version from the Go module cache
      with go mod download -json, downloading it when needed. A development
      build, whose version is (devel) or ends in +dirty, such as go run
      ./cmd/grpc-service-mesh-gen in a checkout, takes the root of the
      working directory's module from go list -m when that module is
      github.com/Paymentbox-com/grpc-service-mesh-api, and fails otherwise.
      go is found on PATH.

Flags:
  --definitions <dir>
      The definitions directory. Required. Every *.proto under it is
      compiled, with paths relative to it. protoc runs three ways: the
      message code of each requested language (Go with --go_out=<dir>
      --go_opt=paths=source_relative, Ruby with --ruby_out=<dir>), one
      FileDescriptorSet of the whole directory with --include_imports
      --include_source_info, and this generator over that set. Every run's
      proto path is the definitions directory, then the specification
      directory that proto-path prints, then each -I directory. The message
      runs list the definitions files, leaving out any copy of
      mesh/options.proto and google/rpc/*.proto, and write what plain protoc
      writes for them. With --go_out, every definitions file sets
      go_package. protoc and protoc-gen-go are found on PATH. A tree that
      declares no service is an error.
  -I <dir>, --proto_path <dir>, --proto_path=<dir>
      An include directory, passed to every protoc run after the
      specification directory, in the order given. Repeatable. A project
      whose files import google/rpc/*.proto passes a checkout of
      github.com/googleapis/googleapis here. When the specification
      directory cannot be resolved and neither the definitions directory
      nor any include directory holds mesh/options.proto, the generator
      stops before running protoc with an error saying why resolving
      failed.
  --go_out=<dir>, --go_out <dir>
      Generate Go into <dir>, the directory protoc's --go_out takes. Each
      definitions directory <path> that declares a service gets
      <dir>/<path>/<name>.grpcmesh.go in its generated package, where
      <name> is the last element of <path>, and the per-transport
      ServiceMaps go to <dir>/servicemaps/servicemaps.go.
  --ruby_out=<dir>, --ruby_out <dir>
      Generate Ruby into <dir>, the directory protoc's --ruby_out takes. Each
      definitions directory <path> that declares a service gets
      <dir>/<path>/<name>_grpcmesh.rb, where <name> is the last element of
      <path>, and the per-transport ServiceMaps go to <dir>/service_maps.rb.
  --go-root-package <import path[;name]>
      Also write <name>.grpcmesh.go in the --go_out directory, package
      <name>, aliasing every generated Go identifier of the definitions tree: each message and enum
      type, each enum value, and each RPCService type, client, and targets
      value. <name> is the last element of the import path unless given after
      ";", as in go_package. Every aliased identifier must be unique across
      the tree, and no directory package may share the root package's name.
      Needs --go_out.
  --ruby-root-module <Module>
      Also write <snake_case(Module)>_grpcmesh.rb in the --ruby_out
      directory, requiring every generated Ruby file and defining module <Module> with a constant for
      every top-level message and enum, each RPCService, client, and targets
      constant, and ServiceMaps. Every aliased constant must be unique across
      the tree, and no generated module may share the root module's name.
      Needs --ruby_out.
  --mesh-only
      Run only the FileDescriptorSet protoc run and write only the mesh
      code: the per-directory files, the ServiceMaps, and the root files.
      For a project that compiles its message code with its own protoc
      command.
  --verbose
      Print each protoc command line and each file written.
  --help
      Print this text.

Exit status is 0 when every file was written, 1 with the error on standard
error otherwise, and 2 for a usage error.
`

// Run executes the command with args (without the program name) and returns
// the exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, specdir.Resolve)
}

// run is Run with the specification directory resolved by spec.
func run(args []string, stdout, stderr io.Writer, spec func() (string, error)) int {
	if len(args) > 0 && args[0] == "proto-path" {
		return protoPath(args[1:], stdout, stderr, spec)
	}
	fs := flag.NewFlagSet("grpc-service-mesh-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { say(stderr, "%s", Usage) }
	definitions := fs.String("definitions", "", "")
	goOut := fs.String("go_out", "", "")
	rubyOut := fs.String("ruby_out", "", "")
	goRoot := fs.String("go-root-package", "", "")
	rubyRoot := fs.String("ruby-root-module", "", "")
	var include includes
	fs.Var(&include, "I", "")
	fs.Var(&include, "proto_path", "")
	meshOnly := fs.Bool("mesh-only", false, "")
	verbose := fs.Bool("verbose", false, "")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	usage := func(msg string) int {
		say(stderr, "grpc-service-mesh-gen: %s (see --help)\n", msg)
		return 2
	}
	if fs.NArg() > 0 {
		return usage("unexpected argument " + fs.Arg(0))
	}
	if *definitions == "" {
		return usage("--definitions is required")
	}
	roots := map[gen.Lang]string{}
	var langs []gen.Lang
	if *goOut != "" {
		roots[gen.Go] = *goOut
		langs = append(langs, gen.Go)
	}
	if *rubyOut != "" {
		roots[gen.Ruby] = *rubyOut
		langs = append(langs, gen.Ruby)
	}
	if len(langs) == 0 {
		return usage("pass at least one of --go_out=<dir> and --ruby_out=<dir>")
	}
	if *goRoot != "" && *goOut == "" {
		return usage("--go-root-package applies to Go, which needs --go_out")
	}
	if *rubyRoot != "" && *rubyOut == "" {
		return usage("--ruby-root-module applies to Ruby, which needs --ruby_out")
	}
	if *goRoot != "" {
		if _, _, err := gen.ParseGoRootPackage(*goRoot); err != nil {
			return usage(err.Error())
		}
	}
	if *rubyRoot != "" {
		if _, err := gen.ParseRubyRootModule(*rubyRoot); err != nil {
			return usage(err.Error())
		}
	}
	opts := gen.Options{Langs: langs, GoRootPackage: *goRoot, RubyRootModule: *rubyRoot}

	r := &protoc.Runner{Definitions: *definitions, Include: include}
	dir, specErr := spec()
	if specErr == nil {
		r.Include = append([]string{dir}, include...)
	}

	var log io.Writer
	if *verbose {
		log = stdout
	}
	r.Verbose = log
	written, err := generate(r, roots, opts, *meshOnly, specErr)
	if err != nil {
		say(stderr, "grpc-service-mesh-gen: %s\n", err)
		return 1
	}
	if log != nil {
		for _, p := range written {
			say(log, "wrote %s\n", p)
		}
	}
	return 0
}

// protoPath prints the specification directory.
func protoPath(args []string, stdout, stderr io.Writer, spec func() (string, error)) int {
	if len(args) > 0 {
		say(stderr, "grpc-service-mesh-gen: proto-path takes no arguments (see --help)\n")
		return 2
	}
	dir, err := spec()
	if err != nil {
		say(stderr, "grpc-service-mesh-gen: %s\n", err)
		return 1
	}
	say(stdout, "%s\n", dir)
	return 0
}

// includes collects -I and --proto_path values in the order given.
type includes []string

func (i *includes) String() string { return strings.Join(*i, " ") }

func (i *includes) Set(dir string) error {
	*i = append(*i, dir)
	return nil
}

// say writes to a terminal stream, whose write errors have nowhere to go.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// generate checks the include paths, then runs the descriptor-set protoc run,
// the mesh generator, and, unless meshOnly, the message runs, and writes the
// generated files. specErr is why the specification directory is not on the
// include path, when it is not.
func generate(r *protoc.Runner, roots map[gen.Lang]string, opts gen.Options, meshOnly bool, specErr error) ([]string, error) {
	files, err := protoc.FindProtos(r.Definitions)
	if err != nil {
		return nil, err
	}
	if err := r.CheckOptions(); err != nil {
		if specErr != nil {
			return nil, fmt.Errorf("%w, and the specification directory could not be resolved: %v", err, specErr)
		}
		return nil, err
	}
	set, err := descriptorSet(r, files)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(set.GetFile(), func(f *descriptorpb.FileDescriptorProto) bool { return len(f.GetService()) > 0 }) {
		return nil, fmt.Errorf("no service declared under %s", r.Definitions)
	}
	model, err := gen.Analyze(set)
	if err != nil {
		return nil, err
	}
	outs, err := gen.Render(model, opts)
	if err != nil {
		return nil, err
	}
	if !meshOnly {
		for _, l := range opts.Langs {
			switch l {
			case gen.Go:
				err = r.GoMessages(roots[l], files)
			case gen.Ruby:
				err = r.RubyMessages(roots[l], files)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return gen.Write(roots, outs)
}

// descriptorSet runs the descriptor-set protoc run into a temporary file and
// reads the set back.
func descriptorSet(r *protoc.Runner, files []string) (*descriptorpb.FileDescriptorSet, error) {
	f, err := os.CreateTemp("", "grpc-service-mesh-gen-*.pb")
	if err != nil {
		return nil, err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := r.DescriptorSet(name, files); err != nil {
		return nil, err
	}
	return gen.ReadSet(name)
}
