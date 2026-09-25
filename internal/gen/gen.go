// Package gen turns a FileDescriptorSet into the gRPC Service Mesh API's
// generated source files.
package gen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

// Options selects what Render emits.
type Options struct {
	Langs []Lang
	// GoRootPackage, when set, is the import path of the root package, with
	// an optional ";name" as in go_package. Its <name>.grpcmesh.go at the Go
	// output root aliases every generated Go identifier.
	GoRootPackage string
	// RubyRootModule, when set, is the module whose <snake_case>_grpcmesh.rb
	// at the Ruby output root aliases every generated Ruby constant.
	RubyRootModule string
}

// Render emits every file of the model the options select, stopping at the
// first error.
func Render(m *Model, o Options) ([]Output, error) {
	var outs []Output
	for _, lang := range o.Langs {
		file, maps := RubyFile, RubyServiceMaps
		if lang == Go {
			file, maps = GoFile, GoServiceMaps
		}
		var emitters []func() (string, []byte, error)
		for _, d := range m.Directories {
			emitters = append(emitters, func() (string, []byte, error) { return file(d) })
		}
		emitters = append(emitters, func() (string, []byte, error) { return maps(m) })
		switch {
		case lang == Go && o.GoRootPackage != "":
			emitters = append(emitters, func() (string, []byte, error) { return GoRoot(m, o.GoRootPackage) })
		case lang == Ruby && o.RubyRootModule != "":
			emitters = append(emitters, func() (string, []byte, error) { return RubyRoot(m, o.RubyRootModule) })
		}
		for _, emit := range emitters {
			p, content, err := emit()
			if err != nil {
				return nil, err
			}
			outs = append(outs, Output{Lang: lang, Path: p, Content: content})
		}
	}
	return outs, nil
}

// ReadSet parses a FileDescriptorSet file.
func ReadSet(file string) (*descriptorpb.FileDescriptorSet, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(b, set); err != nil {
		return nil, err
	}
	return set, nil
}

// Write puts each output under roots[lang]/<path> and returns the paths it
// wrote, in order.
func Write(roots map[Lang]string, outs []Output) ([]string, error) {
	var paths []string
	for _, o := range outs {
		p := filepath.Join(roots[o.Lang], filepath.FromSlash(o.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, o.Content, 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}
