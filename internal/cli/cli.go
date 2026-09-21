// Package cli is grpc-service-mesh-gen's command line.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/gen"
	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/protoc"
)

// Usage is the --help text.
const Usage = `grpc-service-mesh-gen generates the gRPC Service Mesh API code for a
definitions project.

Usage:
  grpc-service-mesh-gen --definitions <dir> --out <dir> --lang go,ruby [--verbose]
  grpc-service-mesh-gen --descriptors <file> --out <dir> --lang go,ruby

Exactly one of --definitions and --descriptors is given.

Flags:
  --definitions <dir>
      The definitions directory. Every *.proto under it is compiled, with
      paths relative to it. protoc runs three ways: the message code of each
      requested language (Go with --go_out=<out>/go --go_opt=paths=source_relative,
      Ruby with --ruby_out=<out>/ruby), one FileDescriptorSet of the whole
      directory with --include_imports --include_source_info, and this
      generator over that set. Embedded copies of mesh/options.proto and
      google/rpc/*.proto are added as a second --proto_path, so a project need
      not vendor them; when it does, those four files are left out of the
      message runs. protoc and protoc-gen-go are found on PATH.
  --descriptors <file>
      A FileDescriptorSet written by protoc with --include_imports, in place
      of --definitions. No protoc run happens; the message code is the
      project's own concern.
  --out <dir>
      The output root. Generated code goes to <out>/go and <out>/ruby. Each
      directory of the definitions that declares a service gets
      <dir>/<name>.grpcmesh.go or <dir>/<name>_grpcmesh.rb in its generated
      package, and the per-transport ServiceMaps go to
      <out>/go/servicemaps/servicemaps.go and <out>/ruby/service_maps.rb.
      Required.
  --lang <list>
      Comma-separated languages to generate, from go and ruby. Required.
  --verbose
      Print each protoc command line and each file written.
  --help
      Print this text.

Exit status is 0 when every file was written, 1 with one line per error on
standard error otherwise, and 2 for a usage error.
`

// Run executes the command with args (without the program name) and returns
// the exit status.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("grpc-service-mesh-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { say(stderr, "%s", Usage) }
	definitions := fs.String("definitions", "", "")
	descriptors := fs.String("descriptors", "", "")
	out := fs.String("out", "", "")
	lang := fs.String("lang", "", "")
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
	if (*definitions == "") == (*descriptors == "") {
		return usage("exactly one of --definitions and --descriptors is required")
	}
	if *out == "" {
		return usage("--out is required")
	}
	langs, err := gen.ParseLangs(*lang)
	if err != nil {
		return usage(err.Error())
	}

	var log io.Writer
	if *verbose {
		log = stdout
	}
	var written []string
	if *descriptors != "" {
		written, err = fromDescriptors(*descriptors, *out, langs)
	} else {
		written, err = fromDefinitions(*definitions, *out, langs, log)
	}
	if err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			say(stderr, "grpc-service-mesh-gen: %s\n", line)
		}
		return 1
	}
	if log != nil {
		for _, p := range written {
			say(log, "wrote %s\n", p)
		}
	}
	return 0
}

// say writes to a terminal stream, whose write errors have nowhere to go.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func fromDescriptors(file, out string, langs []gen.Lang) ([]string, error) {
	set, err := gen.ReadSet(file)
	if err != nil {
		return nil, err
	}
	outs, err := gen.Generate(set, langs)
	if err != nil {
		return nil, err
	}
	return gen.Write(out, outs)
}

func fromDefinitions(definitions, out string, langs []gen.Lang, log io.Writer) ([]string, error) {
	files, err := protoc.FindProtos(definitions)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "grpc-service-mesh-gen-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	embedded, err := protoc.WriteEmbedded(tmp)
	if err != nil {
		return nil, err
	}
	r := &protoc.Runner{Definitions: definitions, Embedded: embedded, Verbose: log}

	setFile := filepath.Join(tmp, "definitions.pb")
	if err := r.DescriptorSet(setFile, files); err != nil {
		return nil, err
	}
	set, err := gen.ReadSet(setFile)
	if err != nil {
		return nil, err
	}
	model, err := gen.Analyze(set)
	if err != nil {
		return nil, err
	}
	outs, err := gen.Render(model, langs)
	if err != nil {
		return nil, err
	}
	for _, l := range langs {
		switch l {
		case gen.Go:
			err = r.GoMessages(filepath.Join(out, "go"), files, gen.GoImportOverrides(model))
		case gen.Ruby:
			err = r.RubyMessages(filepath.Join(out, "ruby"), files)
		}
		if err != nil {
			return nil, err
		}
	}
	return gen.Write(out, outs)
}
