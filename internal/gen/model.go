package gen

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Kind is a method's mesh.kind option.
type Kind int

// The two kinds mesh/options.proto defines.
const (
	Route Kind = iota
	Topic
)

// MessageRef names a message type and the file that defines it.
type MessageRef struct {
	FullName    string // proto full name, such as pbx.ApiKey
	Package     string // proto package of the defining file
	File        string // path of the defining file, relative to the proto path
	GoPackage   string // go_package option of the defining file
	RubyPackage string // ruby_package option of the defining file
}

// Method is one rpc of a service.
type Method struct {
	Name          string
	Kind          Kind
	ConsumerGroup string // empty when the option is absent
	Input         MessageRef
	Output        MessageRef
}

// Service is one proto service and where it was declared.
type Service struct {
	Name        string
	Package     string
	File        string
	RubyPackage string
	Methods     []Method
}

// File is one proto file of a directory.
type File struct {
	Path        string
	GoPackage   string
	RubyPackage string
}

// Directory is a directory of the definitions tree that declares at least
// one service.
type Directory struct {
	Path            string // slash-separated, relative to the definitions root
	Transport       string
	DeploymentGroup string
	Files           []File    // every file in the directory, sorted by path
	Services        []Service // in file order, then declaration order
}

// Source is one proto file of the definitions tree whose message code the
// standard protoc runs write. The specification's own files and the
// google/protobuf files that ship with protoc are not sources.
type Source struct {
	Path        string
	Package     string
	GoPackage   string
	RubyPackage string
	Desc        protoreflect.FileDescriptor
	Services    []Service // the file's services, in declaration order
}

// Model is what the emitters read.
type Model struct {
	Directories []Directory // sorted by path
	Sources     []Source    // sorted by path
	set         *descriptorpb.FileDescriptorSet
}

// isSource reports whether protoc writes message code for the file: every
// file except mesh/options.proto, google/rpc/*.proto, and the
// google/protobuf/*.proto that ship with protoc.
func isSource(p string) bool {
	return p != "mesh/options.proto" && !strings.HasPrefix(p, "google/rpc/") && !strings.HasPrefix(p, "google/protobuf/")
}

// SourceFiles lists the directory's files that declare a service.
func (d Directory) SourceFiles() []string {
	var out []string
	for _, s := range d.Services {
		if len(out) == 0 || out[len(out)-1] != s.File {
			out = append(out, s.File)
		}
	}
	return out
}

// Transports lists every transport in the model, sorted.
func (m *Model) Transports() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range m.Directories {
		if !seen[d.Transport] {
			seen[d.Transport] = true
			out = append(out, d.Transport)
		}
	}
	sort.Strings(out)
	return out
}

// options is the set of mesh.* extension types found in the descriptor set.
type options struct {
	kind, consumerGroup, deploymentGroup, transport protoreflect.ExtensionType
	types                                           *protoregistry.Types
	files                                           *protoregistry.Files
}

func loadOptions(files *protoregistry.Files) (*options, error) {
	if _, err := files.FindFileByPath("mesh/options.proto"); errors.Is(err, protoregistry.NotFound) {
		return nil, errors.New("mesh/options.proto is not in the descriptor set; no definitions file imports it")
	} else if err != nil {
		return nil, err
	}
	o := &options{types: new(protoregistry.Types), files: files}
	for _, x := range []struct {
		name protoreflect.FullName
		dst  *protoreflect.ExtensionType
	}{
		{"mesh.kind", &o.kind},
		{"mesh.consumer_group", &o.consumerGroup},
		{"mesh.deployment_group", &o.deploymentGroup},
		{"mesh.transport", &o.transport},
	} {
		desc, err := files.FindDescriptorByName(x.name)
		if errors.Is(err, protoregistry.NotFound) {
			return nil, fmt.Errorf("%s is not declared by mesh/options.proto", x.name)
		}
		if err != nil {
			return nil, err
		}
		xd, ok := desc.(protoreflect.ExtensionDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s is not an extension", x.name)
		}
		*x.dst = dynamicpb.NewExtensionType(xd)
		if err := o.types.RegisterExtension(*x.dst); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// resolve re-reads opts with the mesh extensions known, so their values move
// out of the unknown fields. The dynamic message is built on the set's own
// descriptor of the options type, the one the extensions extend.
func (o *options) resolve(opts proto.Message) (proto.Message, error) {
	b, err := proto.Marshal(opts)
	if err != nil {
		return nil, err
	}
	desc, err := o.files.FindDescriptorByName(opts.ProtoReflect().Descriptor().FullName())
	if err != nil {
		return nil, err
	}
	dyn := dynamicpb.NewMessage(desc.(protoreflect.MessageDescriptor))
	if err := (proto.UnmarshalOptions{Resolver: o.types}).Unmarshal(b, dyn); err != nil {
		return nil, err
	}
	return dyn, nil
}

func (o *options) str(m proto.Message, xt protoreflect.ExtensionType) (string, bool) {
	if !proto.HasExtension(m, xt) {
		return "", false
	}
	return proto.GetExtension(m, xt).(string), true
}

// fileInfo is one file of the set with its option values read.
type fileInfo struct {
	desc               protoreflect.FileDescriptor
	path, dir, top     string
	transport          string
	hasTransport       bool
	deploymentGroup    string
	hasDeploymentGroup bool
	goPackage          string
	rubyPackage        string
}

// Analyze reads a FileDescriptorSet into a Model, applying the
// specification's directory rules. Every rule violation is reported; the
// returned error joins them one per line.
func Analyze(set *descriptorpb.FileDescriptorSet) (*Model, error) {
	files, err := protodesc.NewFiles(set)
	if err != nil {
		return nil, err
	}
	opts, err := loadOptions(files)
	if err != nil {
		return nil, err
	}

	var infos []*fileInfo
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		infos = append(infos, &fileInfo{desc: fd, path: fd.Path()})
		return true
	})
	sort.Slice(infos, func(i, j int) bool { return infos[i].path < infos[j].path })

	var errs []error
	for _, fi := range infos {
		if fi.desc.Package() == "" {
			errs = append(errs, fmt.Errorf("%s: declares no package", fi.path))
		}
		fi.dir = path.Dir(fi.path)
		fi.top = fi.dir
		if i := strings.IndexByte(fi.dir, '/'); i >= 0 {
			fi.top = fi.dir[:i]
		}
		fo, _ := fi.desc.Options().(*descriptorpb.FileOptions)
		if fo == nil {
			fo = &descriptorpb.FileOptions{}
		}
		fi.goPackage = fo.GetGoPackage()
		fi.rubyPackage = fo.GetRubyPackage()
		resolved, err := opts.resolve(fo)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", fi.path, err)
		}
		fi.transport, fi.hasTransport = opts.str(resolved, opts.transport)
		fi.deploymentGroup, fi.hasDeploymentGroup = opts.str(resolved, opts.deploymentGroup)
	}

	// Top-level directories whose tree declares a service.
	active := map[string]bool{}
	for _, fi := range infos {
		if fi.desc.Services().Len() == 0 {
			continue
		}
		if fi.dir == "." {
			errs = append(errs, fmt.Errorf("%s: a file that declares a service must live in a directory under the definitions root", fi.path))
			continue
		}
		active[fi.top] = true
	}
	for _, fi := range infos {
		if fi.dir == "." && (fi.hasTransport || fi.hasDeploymentGroup) {
			errs = append(errs, fmt.Errorf("%s: transport and deployment_group apply to a top-level directory; a file at the definitions root sets neither", fi.path))
		}
	}

	settings := map[string]*Directory{} // top-level path -> transport and deployment group
	tops := make([]string, 0, len(active))
	for top := range active {
		tops = append(tops, top)
	}
	sort.Strings(tops)
	for _, top := range tops {
		var transportFiles []string
		var transport string
		dgFiles := map[string]string{} // value -> first file setting it
		var dgValues []string
		for _, fi := range infos {
			if fi.top != top {
				continue
			}
			if fi.dir != top {
				if fi.hasTransport {
					errs = append(errs, fmt.Errorf("%s: transport is set in a nested directory; only a file directly in %s/ sets it", fi.path, top))
				}
				if fi.hasDeploymentGroup {
					errs = append(errs, fmt.Errorf("%s: deployment_group is set in a nested directory; only a file directly in %s/ sets it", fi.path, top))
				}
				continue
			}
			if fi.hasTransport {
				transportFiles = append(transportFiles, fi.path)
				transport = fi.transport
			}
			if fi.hasDeploymentGroup {
				if _, seen := dgFiles[fi.deploymentGroup]; !seen {
					dgFiles[fi.deploymentGroup] = fi.path
					dgValues = append(dgValues, fi.deploymentGroup)
				}
			}
		}
		switch len(transportFiles) {
		case 0:
			errs = append(errs, fmt.Errorf("%s: no file sets transport; exactly one file directly in %s/ must set option (mesh.transport)", top, top))
		case 1:
		default:
			errs = append(errs, fmt.Errorf("%s: transport is set in more than one file: %s", top, strings.Join(transportFiles, ", ")))
		}
		if len(dgValues) > 1 {
			var parts []string
			for _, v := range dgValues {
				parts = append(parts, fmt.Sprintf("%q in %s", v, dgFiles[v]))
			}
			errs = append(errs, fmt.Errorf("%s: deployment_group is set to different values: %s", top, strings.Join(parts, ", ")))
		}
		dg := top
		if len(dgValues) == 1 {
			dg = dgValues[0]
		}
		settings[top] = &Directory{Transport: transport, DeploymentGroup: dg}
	}

	// Directories, at any depth, that declare a service.
	byDir := map[string][]*fileInfo{}
	for _, fi := range infos {
		if fi.dir != "." {
			byDir[fi.dir] = append(byDir[fi.dir], fi)
		}
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	// Services of every file, built once; a directory's list reads from here.
	services := map[string][]Service{}
	for _, fi := range infos {
		svcs := fi.desc.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc, err := buildService(fi, svcs.Get(i), opts)
			errs = append(errs, err...)
			services[fi.path] = append(services[fi.path], svc)
		}
	}

	m := &Model{set: set}
	for _, fi := range infos {
		if isSource(fi.path) {
			m.Sources = append(m.Sources, Source{
				Path: fi.path, Package: string(fi.desc.Package()), GoPackage: fi.goPackage, RubyPackage: fi.rubyPackage,
				Desc: fi.desc, Services: services[fi.path],
			})
		}
	}
	for _, dir := range dirs {
		group := byDir[dir]
		hasService := false
		for _, fi := range group {
			if fi.desc.Services().Len() > 0 {
				hasService = true
			}
		}
		if !hasService {
			continue
		}
		top := settings[group[0].top]
		d := Directory{Path: dir, Transport: top.Transport, DeploymentGroup: top.DeploymentGroup}
		for _, fi := range group {
			d.Files = append(d.Files, File{
				Path:        fi.path,
				GoPackage:   fi.goPackage,
				RubyPackage: fi.rubyPackage,
			})
			d.Services = append(d.Services, services[fi.path]...)
		}
		m.Directories = append(m.Directories, d)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return m, nil
}

func buildService(fi *fileInfo, sd protoreflect.ServiceDescriptor, opts *options) (Service, []error) {
	svc := Service{
		Name:        string(sd.Name()),
		Package:     string(fi.desc.Package()),
		File:        fi.path,
		RubyPackage: fi.rubyPackage,
	}
	var errs []error
	methods := sd.Methods()
	for i := 0; i < methods.Len(); i++ {
		md := methods.Get(i)
		if md.IsStreamingClient() || md.IsStreamingServer() {
			errs = append(errs, fmt.Errorf("%s: %s.%s streams; a streaming rpc has no Service Mesh API form", fi.path, sd.Name(), md.Name()))
			continue
		}
		mo, _ := md.Options().(*descriptorpb.MethodOptions)
		if mo == nil {
			mo = &descriptorpb.MethodOptions{}
		}
		resolved, err := opts.resolve(mo)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s.%s: %w", fi.path, sd.Name(), md.Name(), err))
			continue
		}
		method := Method{
			Name:   string(md.Name()),
			Input:  messageRef(md.Input()),
			Output: messageRef(md.Output()),
		}
		method.ConsumerGroup, _ = opts.str(resolved, opts.consumerGroup)
		if proto.HasExtension(resolved, opts.kind) {
			switch n := proto.GetExtension(resolved, opts.kind).(protoreflect.EnumNumber); n {
			case 0:
				method.Kind = Route
			case 1:
				method.Kind = Topic
			default:
				errs = append(errs, fmt.Errorf("%s: %s.%s: mesh.kind %d is neither ROUTE nor TOPIC", fi.path, sd.Name(), md.Name(), n))
				continue
			}
		}
		svc.Methods = append(svc.Methods, method)
	}
	return svc, errs
}

func messageRef(md protoreflect.MessageDescriptor) MessageRef {
	fd := md.ParentFile()
	fo, _ := fd.Options().(*descriptorpb.FileOptions)
	return MessageRef{
		FullName:    string(md.FullName()),
		Package:     string(fd.Package()),
		File:        fd.Path(),
		GoPackage:   fo.GetGoPackage(),
		RubyPackage: fo.GetRubyPackage(),
	}
}
