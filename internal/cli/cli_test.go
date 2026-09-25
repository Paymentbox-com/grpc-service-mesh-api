package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var (
	repoRoot = filepath.Join("..", "..")
	examples = filepath.Join(repoRoot, "examples")
)

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// invokeUnresolved runs the command with a specification directory that
// cannot be resolved.
func invokeUnresolved(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, func() (string, error) { return "", errors.New("go mod download failed") })
	return code, stdout.String(), stderr.String()
}

// specDir is the directory the specification resolves to in a test, the
// root of this checkout.
func specDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRun_Help(t *testing.T) {
	code, _, stderr := invoke(t, "--help")
	if code != 0 || !strings.Contains(stderr, "--definitions <dir>") ||
		!strings.Contains(stderr, "--go_out=<dir>, --go_out <dir>") || !strings.Contains(stderr, "--ruby_out=<dir>, --ruby_out <dir>") ||
		!strings.Contains(stderr, "--go-root-package <import path[;name]>") || !strings.Contains(stderr, "--ruby-root-module <Module>") ||
		!strings.Contains(stderr, "-I <dir>, --proto_path <dir>, --proto_path=<dir>") || !strings.Contains(stderr, "--verbose") ||
		!strings.Contains(stderr, "--mesh-only") || !strings.Contains(stderr, "  proto-path\n") {
		t.Fatalf("code %d, help:\n%s", code, stderr)
	}
}

func TestRun_DefinitionsRequired(t *testing.T) {
	code, _, stderr := invoke(t, "--go_out="+t.TempDir())
	if code != 2 || !strings.Contains(stderr, "--definitions is required") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_NoOutputDirectoryIsAUsageError(t *testing.T) {
	code, stdout, stderr := invoke(t, "--definitions", examples, "--verbose")
	if code != 2 || stderr != "grpc-service-mesh-gen: pass at least one of --go_out=<dir> and --ruby_out=<dir> (see --help)\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout %q", stdout)
	}
}

func TestRun_GoOutWithSeparateValue(t *testing.T) {
	goOut := t.TempDir()
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out", goOut)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(goOut, "pbx", "api_key.pb.go")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_RubyOutWithSeparateValue(t *testing.T) {
	rubyOut := t.TempDir()
	code, _, stderr := invoke(t, "--definitions", examples, "--ruby_out", rubyOut)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(rubyOut, "pbx", "api_key_pb.rb")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_GoRootPackageWithoutGoOutIsAUsageError(t *testing.T) {
	code, _, stderr := invoke(t, "--definitions", examples, "--ruby_out="+t.TempDir(), "--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh")
	if code != 2 || stderr != "grpc-service-mesh-gen: --go-root-package applies to Go, which needs --go_out (see --help)\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_RubyRootModuleWithoutRubyOutIsAUsageError(t *testing.T) {
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+t.TempDir(), "--ruby-root-module", "PmtboxMesh")
	if code != 2 || stderr != "grpc-service-mesh-gen: --ruby-root-module applies to Ruby, which needs --ruby_out (see --help)\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_InvalidGoRootPackageIsAUsageError(t *testing.T) {
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+t.TempDir(), "--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtbox-mesh")
	if code != 2 || !strings.Contains(stderr, `--go-root-package: "pmtbox-mesh" is not a Go package name`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_InvalidRubyRootModuleIsAUsageError(t *testing.T) {
	code, _, stderr := invoke(t, "--definitions", examples, "--ruby_out="+t.TempDir(), "--ruby-root-module", "pmtbox_mesh")
	if code != 2 || !strings.Contains(stderr, `--ruby-root-module: "pmtbox_mesh" is not a Ruby module name`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_PositionalArgumentIsAUsageError(t *testing.T) {
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+t.TempDir(), "extra")
	if code != 2 || !strings.Contains(stderr, "unexpected argument extra") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_GoOutAloneGeneratesOnlyGo(t *testing.T) {
	parent := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--go_out="+filepath.Join(parent, "go"), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	want := []string{"go/pbx/api_key.pb.go", "go/pbx/deployment.pb.go", "go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go"}
	if got := files(t, parent); !slices.Equal(got, want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}
	if strings.Contains(stdout, "--ruby_out") || strings.Contains(stdout, ".rb\n") {
		t.Fatalf("Ruby generated:\n%s", stdout)
	}
}

func TestRun_RubyOutAloneGeneratesOnlyRuby(t *testing.T) {
	parent := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--ruby_out="+filepath.Join(parent, "ruby"), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	want := []string{"ruby/pbx/api_key_pb.rb", "ruby/pbx/deployment_pb.rb", "ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb"}
	if got := files(t, parent); !slices.Equal(got, want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}
	if strings.Contains(stdout, "--go_out") || strings.Contains(stdout, ".go\n") {
		t.Fatalf("Go generated:\n%s", stdout)
	}
}

func TestRun_RootFilesWrittenAtTheOutputDirectories(t *testing.T) {
	goOut, rubyOut := t.TempDir(), t.TempDir()
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+goOut, "--ruby_out="+rubyOut,
		"--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh", "--ruby-root-module", "PmtboxMesh")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, p := range []string{filepath.Join(goOut, "pmtboxmesh.grpcmesh.go"), filepath.Join(rubyOut, "pmtbox_mesh_grpcmesh.rb")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestRun_RootPackageErrorExitsOne(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+out, "--go-root-package", "github.com/Paymentbox-com/pbx")
	want := "grpc-service-mesh-gen: root package pbx: pbx/api_key.proto generates a package with the same name\n" +
		"root package pbx: pbx/deployment.proto generates a package with the same name\n"
	if code != 1 || stderr != want {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
}

func TestRun_GeneratorErrorExitsOne(t *testing.T) {
	defs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(defs, "pbx"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `syntax = "proto3";
package pbx;
import "mesh/options.proto";
message ApiKey {}
service ApiKeyService { rpc Watch(ApiKey) returns (stream ApiKey); }
`
	if err := os.WriteFile(filepath.Join(defs, "pbx", "api_key.proto"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	code, _, stderr := invoke(t, "--definitions", defs, "--go_out="+filepath.Join(out, "go"), "--ruby_out="+filepath.Join(out, "ruby"))
	want := "grpc-service-mesh-gen: pbx: no file sets transport; exactly one file directly in pbx/ must set option (mesh.transport)\n" +
		"pbx/api_key.proto: ApiKeyService.Watch streams; a streaming rpc has no Service Mesh API form\n"
	if code != 1 || stderr != want {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
}

func TestRun_NoServiceIsAnError(t *testing.T) {
	defs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(defs, "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defs, "common", "id.proto"), []byte("syntax = \"proto3\";\npackage common;\nmessage Id {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := invoke(t, "--definitions", defs, "--go_out="+t.TempDir(), "--ruby_out="+t.TempDir())
	if code != 1 || stderr != "grpc-service-mesh-gen: no service declared under "+defs+"\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_DefinitionsRunsProtocAndWritesEverything(t *testing.T) {
	goOut, rubyOut := t.TempDir(), t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--go_out="+goOut, "--ruby_out="+rubyOut, "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if got, want := files(t, goOut), []string{"pbx/api_key.pb.go", "pbx/deployment.pb.go", "pbx/pbx.grpcmesh.go", "servicemaps/servicemaps.go"}; !slices.Equal(got, want) {
		t.Errorf("Go wrote %v, want %v", got, want)
	}
	if got, want := files(t, rubyOut), []string{"pbx/api_key_pb.rb", "pbx/deployment_pb.rb", "pbx/pbx_grpcmesh.rb", "service_maps.rb"}; !slices.Equal(got, want) {
		t.Errorf("Ruby wrote %v, want %v", got, want)
	}
	for _, want := range []string{
		"--include_imports --include_source_info --descriptor_set_out=",
		"--go_out=" + goOut + " --go_opt=paths=source_relative pbx/api_key.proto pbx/deployment.proto\n",
		"--ruby_out=" + rubyOut + " pbx/api_key.proto pbx/deployment.proto\n",
		"wrote " + filepath.Join(rubyOut, "service_maps.rb"),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in verbose output:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "protoc ") != 3 {
		t.Errorf("three protoc runs expected:\n%s", stdout)
	}
}

func TestRun_CopiesOfPublishedFilesAreSkippedInMessageRuns(t *testing.T) {
	defs := t.TempDir()
	for _, p := range []string{"pbx/api_key.proto", "pbx/deployment.proto"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, "examples", filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(defs, "pbx"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(defs, filepath.FromSlash(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	options, err := os.ReadFile(filepath.Join(repoRoot, "mesh", "options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	for p, src := range map[string]string{
		"mesh/options.proto":             string(options),
		"google/rpc/code.proto":          "syntax = \"proto3\";\npackage google.rpc;\nenum Code { OK = 0; }\n",
		"google/rpc/status.proto":        "syntax = \"proto3\";\npackage google.rpc;\nmessage Status {}\n",
		"google/rpc/error_details.proto": "syntax = \"proto3\";\npackage google.rpc;\nmessage ErrorInfo {}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(defs, filepath.FromSlash(p))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(defs, filepath.FromSlash(p)), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", defs, "--go_out="+filepath.Join(out, "go"), "--ruby_out="+filepath.Join(out, "ruby"), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "--go_opt=paths=source_relative pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Go run did not skip the published files:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--ruby_out="+filepath.Join(out, "ruby")+" pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Ruby run did not skip the published files:\n%s", stdout)
	}
	for _, p := range []string{"go/google", "go/mesh", "ruby/google", "ruby/mesh"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("code for a published file was written to %s: %v", p, err)
		}
	}
}

// TestRun_MessageOutputIsWhatPlainProtocWrites compiles examples/pbx with
// protoc directly, with -I set to the directory proto-path prints, and
// compares every message file the generator wrote to protoc's.
func TestRun_MessageOutputIsWhatPlainProtocWrites(t *testing.T) {
	code, protoPath, stderr := invoke(t, "proto-path")
	if code != 0 {
		t.Fatalf("proto-path: code %d, stderr %q", code, stderr)
	}
	out := t.TempDir()
	if code, _, stderr := invoke(t, "--definitions", examples, "--go_out="+filepath.Join(out, "go"), "--ruby_out="+filepath.Join(out, "ruby")); code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	plain := t.TempDir()
	for _, p := range []string{"go", "ruby"} {
		if err := os.MkdirAll(filepath.Join(plain, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("protoc", "-I", examples, "-I", strings.TrimSuffix(protoPath, "\n"),
		"--go_out="+filepath.Join(plain, "go"), "--go_opt=paths=source_relative",
		"--ruby_out="+filepath.Join(plain, "ruby"),
		"pbx/api_key.proto", "pbx/deployment.proto")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, b)
	}

	want := []string{"go/pbx/api_key.pb.go", "go/pbx/deployment.pb.go", "ruby/pbx/api_key_pb.rb", "ruby/pbx/deployment_pb.rb"}
	if got := files(t, plain); !slices.Equal(got, want) {
		t.Fatalf("plain protoc wrote %v, want %v", got, want)
	}
	generated := files(t, out)
	for _, p := range want {
		a, err := os.ReadFile(filepath.Join(plain, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs from plain protoc's", p)
		}
	}
	messages := slices.DeleteFunc(generated, func(p string) bool {
		return strings.HasSuffix(p, ".grpcmesh.go") || strings.HasSuffix(p, "_grpcmesh.rb") ||
			p == "go/servicemaps/servicemaps.go" || p == "ruby/service_maps.rb"
	})
	if !slices.Equal(messages, want) {
		t.Errorf("generator wrote message files %v, want %v", messages, want)
	}
}

func TestRun_SpecificationDirectoryPrecedesIncludeDirectories(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "-I", first, "-I", second, "--go_out="+t.TempDir(), "--ruby_out="+t.TempDir(), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	want := "protoc --proto_path=" + examples + " --proto_path=" + specDir(t) + " --proto_path=" + first + " --proto_path=" + second + " --"
	if got := strings.Count(stdout, want); got != 3 {
		t.Fatalf("%d of 3 protoc runs start with %q:\n%s", got, want, stdout)
	}
}

func TestRun_ProtoPathWithSeparateValue(t *testing.T) {
	include := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--proto_path", include, "--ruby_out="+t.TempDir(), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "protoc --proto_path="+examples+" --proto_path="+specDir(t)+" --proto_path="+include+" --ruby_out=") {
		t.Fatalf("verbose output:\n%s", stdout)
	}
}

func TestRun_ProtoPathWithEquals(t *testing.T) {
	include := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--proto_path="+include, "--ruby_out="+t.TempDir(), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "protoc --proto_path="+examples+" --proto_path="+specDir(t)+" --proto_path="+include+" --ruby_out=") {
		t.Fatalf("verbose output:\n%s", stdout)
	}
}

func TestRun_UnresolvedSpecificationDirectoryWithOptionsOnAnIncludePath(t *testing.T) {
	code, stdout, stderr := invokeUnresolved(t, "--definitions", examples, "-I", repoRoot, "--ruby_out="+t.TempDir(), "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "protoc --proto_path="+examples+" --proto_path="+repoRoot+" --ruby_out=") {
		t.Fatalf("verbose output:\n%s", stdout)
	}
}

func TestRun_MissingOptionsProtoIsOneErrorBeforeProtoc(t *testing.T) {
	code, stdout, stderr := invokeUnresolved(t, "--definitions", examples, "-I", t.TempDir(), "--go_out="+t.TempDir(), "--ruby_out="+t.TempDir(), "--verbose")
	want := "grpc-service-mesh-gen: mesh/options.proto was not found on any -I path, and the specification directory could not be resolved: go mod download failed\n"
	if code != 1 || stderr != want {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if strings.Contains(stdout, "protoc ") {
		t.Fatalf("protoc ran:\n%s", stdout)
	}
}

func TestRun_MeshOnlyWritesOnlyTheMeshCode(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := invoke(t, "--definitions", examples, "--go_out="+filepath.Join(out, "go"), "--ruby_out="+filepath.Join(out, "ruby"), "--mesh-only", "--verbose",
		"--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh", "--ruby-root-module", "PmtboxMesh")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	want := []string{
		"go/pbx/pbx.grpcmesh.go", "go/pmtboxmesh.grpcmesh.go", "go/servicemaps/servicemaps.go",
		"ruby/pbx/pbx_grpcmesh.rb", "ruby/pmtbox_mesh_grpcmesh.rb", "ruby/service_maps.rb",
	}
	if got := files(t, out); !slices.Equal(got, want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}
	if strings.Count(stdout, "protoc ") != 1 || !strings.Contains(stdout, "--descriptor_set_out=") {
		t.Fatalf("one descriptor-set protoc run expected:\n%s", stdout)
	}
}

func TestRun_ProtoPathPrintsTheSpecificationDirectory(t *testing.T) {
	code, stdout, stderr := invoke(t, "proto-path")
	if code != 0 || stderr != "" || stdout != specDir(t)+"\n" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(strings.TrimSuffix(stdout, "\n"), "mesh", "options.proto")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_ProtoPathFailureExitsOne(t *testing.T) {
	code, stdout, stderr := invokeUnresolved(t, "proto-path")
	if code != 1 || stdout != "" || stderr != "grpc-service-mesh-gen: go mod download failed\n" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRun_ProtoPathTakesNoArguments(t *testing.T) {
	code, stdout, stderr := invoke(t, "proto-path", "extra")
	if code != 2 || stdout != "" || stderr != "grpc-service-mesh-gen: proto-path takes no arguments (see --help)\n" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

func TestRun_ProtocErrorPassesThrough(t *testing.T) {
	defs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(defs, "pbx"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defs, "pbx", "api_key.proto"), []byte("syntax = \"proto3\";\nimport \"pbx/missing.proto\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := invoke(t, "--definitions", defs, "--go_out="+t.TempDir(), "--ruby_out="+t.TempDir())
	if code != 1 || !strings.Contains(stderr, "pbx/missing.proto: File not found.") {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
}

// files lists every file under dir as a sorted slash path relative to it.
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}
