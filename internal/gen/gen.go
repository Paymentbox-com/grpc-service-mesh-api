// Package gen turns a FileDescriptorSet into the gRPC Service Mesh API's
// generated source files.
package gen

import (
	"os"
	"path/filepath"
	"slices"

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

// Output is one generated file. Path is relative to the language's output
// root, such as shop/shop.grpcmesh.go. Root marks a root file, the one
// GoRootPackage or RubyRootModule selects.
type Output struct {
	Lang    Lang
	Path    string
	Content []byte
	Root    bool
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
// first error. When Go is requested, every source sets go_package.
func Render(m *Model, o Options) ([]Output, error) {
	if slices.Contains(o.Langs, Go) {
		for _, src := range m.Sources {
			if src.GoPackage == "" {
				return nil, missingGoPackage(src.Path)
			}
		}
	}
	var outs []Output
	for _, lang := range o.Langs {
		file, maps := RubyFile, RubyServiceMaps
		if lang == Go {
			file, maps = GoFile, GoServiceMaps
		}
		type emitter struct {
			emit func() (string, []byte, error)
			root bool
		}
		var emitters []emitter
		for _, d := range m.Directories {
			emitters = append(emitters, emitter{emit: func() (string, []byte, error) { return file(d) }})
		}
		emitters = append(emitters, emitter{emit: func() (string, []byte, error) { return maps(m) }})
		switch {
		case lang == Go && o.GoRootPackage != "":
			emitters = append(emitters, emitter{emit: func() (string, []byte, error) { return GoRoot(m, o.GoRootPackage) }, root: true})
		case lang == Ruby && o.RubyRootModule != "":
			emitters = append(emitters, emitter{emit: func() (string, []byte, error) { return RubyRoot(m, o.RubyRootModule) }, root: true})
		}
		for _, e := range emitters {
			p, content, err := e.emit()
			if err != nil {
				return nil, err
			}
			outs = append(outs, Output{Lang: lang, Path: p, Content: content, Root: e.root})
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

// Write puts each output under roots[lang]/<path>, or a root file under
// rootDirs[lang]/<path> when rootDirs has that language, and returns the
// paths it wrote, in order.
func Write(roots, rootDirs map[Lang]string, outs []Output) ([]string, error) {
	var paths []string
	for _, o := range outs {
		dir := roots[o.Lang]
		if d, ok := rootDirs[o.Lang]; ok && o.Root {
			dir = d
		}
		p := filepath.Join(dir, filepath.FromSlash(o.Path))
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
