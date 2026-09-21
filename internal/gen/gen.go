// Package gen turns a FileDescriptorSet into the gRPC Service Mesh API's
// generated source files.
package gen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Lang is a generated language.
type Lang string

// The languages the generator emits.
const (
	Go   Lang = "go"
	Ruby Lang = "ruby"
)

// ParseLangs reads a comma-separated --lang value.
func ParseLangs(s string) ([]Lang, error) {
	var out []Lang
	seen := map[Lang]bool{}
	for _, part := range splitComma(s) {
		l := Lang(part)
		switch l {
		case Go, Ruby:
		default:
			return nil, fmt.Errorf("unknown language %q; the languages are go and ruby", part)
		}
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("at least one language is required, as --lang go, --lang ruby, or --lang go,ruby")
	}
	return out, nil
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := s[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

// Output is one generated file. Path is relative to the language's output
// root, such as pbx/pbx.grpcmesh.go.
type Output struct {
	Lang    Lang
	Path    string
	Content []byte
}

// Generate reads the set and renders every file for the requested languages.
// Nothing is rendered when any error is found; the error lists each one.
func Generate(set *descriptorpb.FileDescriptorSet, langs []Lang) ([]Output, error) {
	m, err := Analyze(set)
	if err != nil {
		return nil, err
	}
	return Render(m, langs)
}

// Render emits every file of the model for the requested languages.
func Render(m *Model, langs []Lang) ([]Output, error) {
	var outs []Output
	var errs []error
	for _, lang := range langs {
		file, maps := RubyFile, RubyServiceMaps
		if lang == Go {
			file, maps = GoFile, GoServiceMaps
		}
		for _, d := range m.Directories {
			p, content, err := file(d)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			outs = append(outs, Output{Lang: lang, Path: p, Content: content})
		}
		if len(m.Directories) > 0 {
			p, content, err := maps(m)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			outs = append(outs, Output{Lang: lang, Path: p, Content: content})
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(dedupe(errs)...)
	}
	return outs, nil
}

// dedupe drops repeated errors; the Go emitters report a directory's
// go_package once per file that needs it.
func dedupe(errs []error) []error {
	seen := map[string]bool{}
	var out []error
	for _, e := range errs {
		if !seen[e.Error()] {
			seen[e.Error()] = true
			out = append(out, e)
		}
	}
	return out
}

// ReadSet parses a FileDescriptorSet file.
func ReadSet(file string) (*descriptorpb.FileDescriptorSet, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(b, set); err != nil {
		return nil, fmt.Errorf("%s: not a FileDescriptorSet: %w", file, err)
	}
	return set, nil
}

// Write puts each output under <out>/<lang>/<path> and returns the paths it
// wrote, sorted.
func Write(out string, outs []Output) ([]string, error) {
	var paths []string
	for _, o := range outs {
		p := filepath.Join(out, string(o.Lang), filepath.FromSlash(o.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, o.Content, 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}
