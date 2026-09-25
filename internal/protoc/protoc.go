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
	"slices"
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

// MessageFiles returns files without copies of the specification's own
// files. Those are only on the proto path; their compiled forms ship with the
// language libraries and the standard google/rpc packages.
func MessageFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if !slices.Contains(spec.Paths, f) {
			out = append(out, f)
		}
	}
	return out
}

func (r *Runner) run(args []string, files []string) error {
	all := append([]string{
		"--proto_path=" + r.Definitions,
		"--proto_path=" + r.Embedded,
	}, args...)
	all = append(all, files...)
	if r.Verbose != nil {
		_, _ = fmt.Fprintln(r.Verbose, "protoc", strings.Join(all, " "))
	}
	cmd := exec.Command("protoc", all...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("protoc %s: %w", strings.Join(args, " "), err)
		}
		return fmt.Errorf("protoc %s: %w\n%s", strings.Join(args, " "), err, msg)
	}
	return nil
}

// GoMessages writes the Go message code with paths=source_relative into out.
func (r *Runner) GoMessages(out string, files []string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return r.run([]string{"--go_out=" + out, "--go_opt=paths=source_relative"}, MessageFiles(files))
}

// RubyMessages writes the Ruby message code into out.
func (r *Runner) RubyMessages(out string, files []string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return r.run([]string{"--ruby_out=" + out}, MessageFiles(files))
}

// DescriptorSet writes one FileDescriptorSet for files, with imports and
// source info, to file.
func (r *Runner) DescriptorSet(file string, files []string) error {
	return r.run([]string{"--include_imports", "--include_source_info", "--descriptor_set_out=" + file}, files)
}
