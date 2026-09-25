// Package protoc runs the protoc compiler for the generator: the message
// code of each language and the FileDescriptorSet the mesh generator reads.
package protoc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// OptionsProto is the specification's options file, imported by every
// definitions file that sets a mesh option.
const OptionsProto = "mesh/options.proto"

// PublishedFiles lists the proto files whose compiled forms are published
// packages: mesh/options.proto in the language libraries, and google/rpc in
// google.golang.org/genproto and the googleapis-common-protos-types gem.
var PublishedFiles = []string{
	OptionsProto,
	"google/rpc/code.proto",
	"google/rpc/status.proto",
	"google/rpc/error_details.proto",
}

// ErrOptionsNotFound reports that no include path holds mesh/options.proto.
var ErrOptionsNotFound = errors.New(OptionsProto + " was not found on any -I path")

// Runner invokes protoc with the definitions directory and then each
// include directory on its proto path.
type Runner struct {
	Definitions string    // the definitions directory
	Include     []string  // further proto path entries, in order
	Verbose     io.Writer // each command line is printed here when set
}

// CheckOptions returns ErrOptionsNotFound unless the definitions directory
// or an include directory holds mesh/options.proto. An include entry may be a
// list joined with the OS path list separator, as protoc accepts.
func (r *Runner) CheckOptions() error {
	dirs := []string{r.Definitions}
	for _, entry := range r.Include {
		dirs = append(dirs, filepath.SplitList(entry)...)
	}
	for _, dir := range dirs {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(OptionsProto))); err == nil && !info.IsDir() {
			return nil
		}
	}
	return ErrOptionsNotFound
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

// MessageFiles returns files without copies of PublishedFiles.
func MessageFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if !slices.Contains(PublishedFiles, f) {
			out = append(out, f)
		}
	}
	return out
}

func (r *Runner) run(args []string, files []string) error {
	all := []string{"--proto_path=" + r.Definitions}
	for _, dir := range r.Include {
		all = append(all, "--proto_path="+dir)
	}
	all = append(all, args...)
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
