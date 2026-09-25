// Package cli is grpc-service-mesh-gen's command line.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/gen"
	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/protoc"
)

// Usage is the --help text.
const Usage = `grpc-service-mesh-gen generates the gRPC Service Mesh API code for a
definitions project.

Usage:
  grpc-service-mesh-gen --definitions <dir> -I <dir> --out <dir> --lang go,ruby [--verbose]

Flags:
  --definitions <dir>
      The definitions directory. Required. Every *.proto under it is
      compiled, with paths relative to it. protoc runs three ways: the
      message code of each requested language (Go with --go_out=<go out>
      --go_opt=paths=source_relative, Ruby with --ruby_out=<ruby out>), one
      FileDescriptorSet of the whole directory with --include_imports
      --include_source_info, and this generator over that set. The
      definitions directory is the first --proto_path of every run. The
      message runs list the definitions files, leaving out any copy of
      mesh/options.proto and google/rpc/*.proto, and write what plain protoc
      writes for them. With go in --lang, every definitions file sets
      go_package. protoc and protoc-gen-go are found on PATH. A tree that
      declares no service is an error.
  -I <dir>, --proto_path <dir>, --proto_path=<dir>
      An include directory, passed to every protoc run after the definitions
      directory, in the order given. Repeatable. The generator and plain
      protoc read the same include paths. The library a project depends on
      supplies the specification's protos, mesh/options.proto and
      google/rpc/*.proto, under its proto/ directory at the version its
      compiled options were built from:
        -I "$(go list -m -f '{{.Dir}}' github.com/Paymentbox-com/grpc-service-mesh-go)/proto"
        -I "$(bundle info --path grpc_service_mesh)/proto"
      When neither the definitions directory nor any include directory
      holds mesh/options.proto, the generator stops with an error before
      running protoc.
  --out <dir>
      The output root. Generated code goes to <out>/go and <out>/ruby. Each
      directory of the definitions that declares a service gets
      <dir>/<name>.grpcmesh.go or <dir>/<name>_grpcmesh.rb in its generated
      package, and the per-transport ServiceMaps go to
      <out>/go/servicemaps/servicemaps.go and <out>/ruby/service_maps.rb.
      Required unless every requested language has its own output root.
  --go-out <dir>
      The Go output root, in place of <out>/go. Needs go in --lang.
  --ruby-out <dir>
      The Ruby output root, in place of <out>/ruby. Needs ruby in --lang.
  --go-root-package <import path[;name]>
      Also write <go out>/<name>.grpcmesh.go, package <name>, aliasing every
      generated Go identifier of the definitions tree: each message and enum
      type, each enum value, and each RPCService type, client, and targets
      value. <name> is the last element of the import path unless given after
      ";", as in go_package. Every aliased identifier must be unique across
      the tree, and no directory package may share the root package's name.
      Needs go in --lang.
  --ruby-root-module <Module>
      Also write <ruby out>/<snake_case(Module)>_grpcmesh.rb, requiring every
      generated Ruby file and defining module <Module> with a constant for
      every top-level message and enum, each RPCService, client, and targets
      constant, and ServiceMaps. Every aliased constant must be unique across
      the tree, and no generated module may share the root module's name.
      Needs ruby in --lang.
  --lang <list>
      Comma-separated languages to generate, from go and ruby. Required.
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
	fs := flag.NewFlagSet("grpc-service-mesh-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { say(stderr, "%s", Usage) }
	definitions := fs.String("definitions", "", "")
	out := fs.String("out", "", "")
	goOut := fs.String("go-out", "", "")
	rubyOut := fs.String("ruby-out", "", "")
	goRoot := fs.String("go-root-package", "", "")
	rubyRoot := fs.String("ruby-root-module", "", "")
	lang := fs.String("lang", "", "")
	var include includes
	fs.Var(&include, "I", "")
	fs.Var(&include, "proto_path", "")
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
	langs, err := gen.ParseLangs(*lang)
	if err != nil {
		return usage(err.Error())
	}
	has := map[gen.Lang]bool{}
	for _, l := range langs {
		has[l] = true
	}
	for _, f := range []struct {
		name, value string
		lang        gen.Lang
	}{
		{"--go-out", *goOut, gen.Go}, {"--go-root-package", *goRoot, gen.Go},
		{"--ruby-out", *rubyOut, gen.Ruby}, {"--ruby-root-module", *rubyRoot, gen.Ruby},
	} {
		if f.value != "" && !has[f.lang] {
			return usage(fmt.Sprintf("%s applies to %s, which is not in --lang", f.name, f.lang))
		}
	}
	roots := map[gen.Lang]string{}
	for _, l := range langs {
		own := *goOut
		if l == gen.Ruby {
			own = *rubyOut
		}
		switch {
		case own != "":
			roots[l] = own
		case *out != "":
			roots[l] = filepath.Join(*out, string(l))
		default:
			return usage(fmt.Sprintf("--out is required unless --%s-out is given", l))
		}
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

	var log io.Writer
	if *verbose {
		log = stdout
	}
	r := &protoc.Runner{Definitions: *definitions, Include: include, Verbose: log}
	written, err := generate(r, roots, opts)
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
// the mesh generator, and the message runs, and writes the generated files.
func generate(r *protoc.Runner, roots map[gen.Lang]string, opts gen.Options) ([]string, error) {
	files, err := protoc.FindProtos(r.Definitions)
	if err != nil {
		return nil, err
	}
	if err := r.CheckOptions(); err != nil {
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
