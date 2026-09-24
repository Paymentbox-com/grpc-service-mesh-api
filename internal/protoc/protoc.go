// Package protoc runs the protoc compiler for the generator: the message
// code of each language and the FileDescriptorSet the mesh generator reads.
package protoc

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Paymentbox-com/grpc-service-mesh-api"
)

// Runner invokes protoc with the definitions directory and the embedded
// specification files on its proto path.
type Runner struct {
	Definitions string    // the definitions directory
	Embedded    string    // directory holding the embedded spec files
	Verbose     io.Writer // each command line is printed here when set
	Protoc      string    // the protoc executable; "protoc" when empty
}

// WriteEmbedded writes the specification's proto files under dir, at their
// import paths, and returns that directory.
func WriteEmbedded(dir string) (string, error) {
	root := filepath.Join(dir, "grpc-service-mesh-api")
	for _, p := range spec.Paths {
		b, err := fs.ReadFile(spec.Files, p)
		if err != nil {
			return "", err
		}
		dst := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return "", err
		}
	}
	return root, nil
}

// FindProtos lists every .proto under the definitions directory, as
// slash-separated paths relative to it, sorted.
func FindProtos(definitions string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(definitions, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".proto") {
			return nil
		}
		rel, err := filepath.Rel(definitions, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no .proto files found", definitions)
	}
	return out, nil
}

// OptionsProto and OptionsGoImport are the specification's options file and
// its compiled Go form, as spec defines them.
const (
	OptionsProto    = spec.OptionsProto
	OptionsGoImport = spec.OptionsGoImport
)

// googleRPC lists the google/rpc files, whose compiled forms come from the
// standard packages of each language: google.golang.org/genproto in Go and
// the googleapis-common-protos-types gem in Ruby.
var googleRPC = map[string]bool{
	"google/rpc/code.proto":          true,
	"google/rpc/status.proto":        true,
	"google/rpc/error_details.proto": true,
}

// GoMessageFiles returns files without the specification's own; their
// compiled forms are OptionsGoImport and genproto.
func GoMessageFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if !googleRPC[f] && f != OptionsProto {
			out = append(out, f)
		}
	}
	return out
}

// RubyMessageFiles returns files without google/rpc, plus OptionsProto,
// whose Ruby form no gem ships; the run writes <out>/ruby/mesh/options_pb.rb
// from the vendored or embedded copy so the message files' require of it
// resolves on the same load path.
func RubyMessageFiles(files []string) []string {
	var out []string
	hasOptions := false
	for _, f := range files {
		if googleRPC[f] {
			continue
		}
		if f == OptionsProto {
			hasOptions = true
		}
		out = append(out, f)
	}
	if !hasOptions {
		out = append(out, OptionsProto)
	}
	return out
}

func (r *Runner) run(args []string, files []string) error {
	name := r.Protoc
	if name == "" {
		name = "protoc"
	}
	all := append([]string{
		"--proto_path=" + r.Definitions,
		"--proto_path=" + r.Embedded,
	}, args...)
	all = append(all, files...)
	if r.Verbose != nil {
		_, _ = fmt.Fprintln(r.Verbose, name, strings.Join(all, " "))
	}
	cmd := exec.Command(name, all...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, msg)
	}
	return nil
}

// GoMessages writes the Go message code with paths=source_relative into out.
// importPaths gives protoc-gen-go the import path of each file that sets no
// go_package, as --go_opt=M<file>=<path>; OptionsProto always maps to
// OptionsGoImport.
func (r *Runner) GoMessages(out string, files []string, importPaths map[string]string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	args := []string{"--go_out=" + out, "--go_opt=paths=source_relative", "--go_opt=M" + OptionsProto + "=" + OptionsGoImport}
	keys := make([]string, 0, len(importPaths))
	for f := range importPaths {
		if f != OptionsProto {
			keys = append(keys, f)
		}
	}
	sort.Strings(keys)
	for _, f := range keys {
		args = append(args, "--go_opt=M"+f+"="+importPaths[f])
	}
	return r.run(args, GoMessageFiles(files))
}

// RubyMessages writes the Ruby message code into out.
func (r *Runner) RubyMessages(out string, files []string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return r.run([]string{"--ruby_out=" + out}, RubyMessageFiles(files))
}

// DescriptorSet writes one FileDescriptorSet for files, with imports and
// source info, to file.
func (r *Runner) DescriptorSet(file string, files []string) error {
	return r.run([]string{"--include_imports", "--include_source_info", "--descriptor_set_out=" + file}, files)
}
