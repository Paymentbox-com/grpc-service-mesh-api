package cli

import (
	"bytes"
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

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun_Help(t *testing.T) {
	code, _, stderr := run(t, "--help")
	if code != 0 || !strings.Contains(stderr, "--definitions <dir>") ||
		!strings.Contains(stderr, "--out <dir>") || !strings.Contains(stderr, "--go-out <dir>") || !strings.Contains(stderr, "--ruby-out <dir>") ||
		!strings.Contains(stderr, "--go-root-package <import path[;name]>") || !strings.Contains(stderr, "--ruby-root-module <Module>") ||
		!strings.Contains(stderr, "--lang <list>") || !strings.Contains(stderr, "-I <dir>, --proto_path <dir>, --proto_path=<dir>") || !strings.Contains(stderr, "--verbose") {
		t.Fatalf("code %d, help:\n%s", code, stderr)
	}
}

func TestRun_DefinitionsRequired(t *testing.T) {
	code, _, stderr := run(t, "--out", t.TempDir(), "--lang", "go")
	if code != 2 || !strings.Contains(stderr, "--definitions is required") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_OutRequired(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--lang", "go")
	if code != 2 || !strings.Contains(stderr, "--out is required unless --go-out is given") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_OutRequiredForTheLanguageWithoutItsOwnRoot(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--lang", "go,ruby", "--go-out", t.TempDir())
	if code != 2 || !strings.Contains(stderr, "--out is required unless --ruby-out is given") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_GoOutWithoutGoIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "ruby", "--go-out", t.TempDir())
	if code != 2 || !strings.Contains(stderr, "--go-out applies to go, which is not in --lang") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_RubyRootModuleWithoutRubyIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "go", "--ruby-root-module", "PmtboxMesh")
	if code != 2 || !strings.Contains(stderr, "--ruby-root-module applies to ruby, which is not in --lang") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_InvalidGoRootPackageIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "go", "--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtbox-mesh")
	if code != 2 || !strings.Contains(stderr, `--go-root-package: "pmtbox-mesh" is not a Go package name`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_InvalidRubyRootModuleIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "ruby", "--ruby-root-module", "pmtbox_mesh")
	if code != 2 || !strings.Contains(stderr, `--ruby-root-module: "pmtbox_mesh" is not a Ruby module name`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_UnknownLanguageIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "go,java")
	if code != 2 || !strings.Contains(stderr, `unknown language "java"`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_PositionalArgumentIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", examples, "--out", t.TempDir(), "--lang", "go", "extra")
	if code != 2 || !strings.Contains(stderr, "unexpected argument extra") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_OnlyRequestedLanguage(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--lang", "ruby")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "go")); !os.IsNotExist(err) {
		t.Fatalf("go directory written: %v", err)
	}
}

func TestRun_GoOutReplacesTheGoRoot(t *testing.T) {
	out, goOut := t.TempDir(), t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--go-out", goOut, "--lang", "go,ruby")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, p := range []string{filepath.Join(goOut, "pbx", "pbx.grpcmesh.go"), filepath.Join(goOut, "servicemaps", "servicemaps.go"), filepath.Join(out, "ruby", "service_maps.rb")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "go")); !os.IsNotExist(err) {
		t.Errorf("<out>/go written: %v", err)
	}
}

func TestRun_RubyOutReplacesTheRubyRoot(t *testing.T) {
	out, rubyOut := t.TempDir(), t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--ruby-out", rubyOut, "--lang", "go,ruby")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, p := range []string{filepath.Join(rubyOut, "pbx", "pbx_grpcmesh.rb"), filepath.Join(rubyOut, "service_maps.rb"), filepath.Join(out, "go", "servicemaps", "servicemaps.go")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "ruby")); !os.IsNotExist(err) {
		t.Errorf("<out>/ruby written: %v", err)
	}
}

func TestRun_BothLanguageRootsNeedNoOut(t *testing.T) {
	goOut, rubyOut := t.TempDir(), t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--go-out", goOut, "--ruby-out", rubyOut, "--lang", "go,ruby")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, p := range []string{filepath.Join(goOut, "servicemaps", "servicemaps.go"), filepath.Join(rubyOut, "service_maps.rb")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%v", err)
		}
	}
}

func TestRun_RootFilesWrittenAtTheLanguageRoots(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--lang", "go,ruby",
		"--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh", "--ruby-root-module", "PmtboxMesh")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	for _, p := range []string{"go/pmtboxmesh.grpcmesh.go", "ruby/pmtbox_mesh_grpcmesh.rb"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestRun_RootPackageErrorExitsOne(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--lang", "go", "--go-root-package", "github.com/Paymentbox-com/pbx")
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
	code, _, stderr := run(t, "--definitions", defs, "-I", repoRoot, "--out", out, "--lang", "go,ruby")
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
	code, _, stderr := run(t, "--definitions", defs, "-I", repoRoot, "--out", t.TempDir(), "--lang", "go,ruby")
	if code != 1 || stderr != "grpc-service-mesh-gen: no service declared under "+defs+"\n" {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_DefinitionsRunsProtocAndWritesEverything(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--lang", "go,ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	for _, p := range []string{
		"go/pbx/api_key.pb.go", "go/pbx/deployment.pb.go", "go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go",
		"ruby/pbx/api_key_pb.rb", "ruby/pbx/deployment_pb.rb", "ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	for _, p := range []string{"go/mesh", "ruby/mesh"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("code for mesh/options.proto was written to %s: %v", p, err)
		}
	}
	for _, want := range []string{
		"--include_imports --include_source_info --descriptor_set_out=",
		"--go_out=" + filepath.Join(out, "go") + " --go_opt=paths=source_relative pbx/api_key.proto pbx/deployment.proto\n",
		"--ruby_out=" + filepath.Join(out, "ruby") + " pbx/api_key.proto pbx/deployment.proto\n",
		"wrote " + filepath.Join(out, "ruby", "service_maps.rb"),
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in verbose output:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, "protoc ") != 3 {
		t.Errorf("three protoc runs expected:\n%s", stdout)
	}
}

func TestRun_DefinitionsMessageRunsUseTheLanguageRoots(t *testing.T) {
	goOut, rubyOut := t.TempDir(), t.TempDir()
	code, stdout, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--go-out", goOut, "--ruby-out", rubyOut, "--lang", "go,ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"--go_out=" + goOut + " ", "--ruby_out=" + rubyOut + " "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in verbose output:\n%s", want, stdout)
		}
	}
	for _, p := range []string{filepath.Join(goOut, "pbx", "api_key.pb.go"), filepath.Join(rubyOut, "pbx", "api_key_pb.rb")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%v", err)
		}
	}
}

// TestRun_VendoredSpecificationFilesAreSkippedInMessageRuns passes no -I, so
// mesh/options.proto resolves from the definitions directory.
func TestRun_VendoredSpecificationFilesAreSkippedInMessageRuns(t *testing.T) {
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
	for _, p := range []string{"mesh/options.proto", "google/rpc/code.proto", "google/rpc/status.proto", "google/rpc/error_details.proto"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(defs, filepath.FromSlash(p))), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(defs, filepath.FromSlash(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	code, stdout, stderr := run(t, "--definitions", defs, "--out", out, "--lang", "go,ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "--go_opt=paths=source_relative pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Go run did not skip the specification files:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--ruby_out="+filepath.Join(out, "ruby")+" pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Ruby run did not skip the specification files:\n%s", stdout)
	}
	for _, p := range []string{"go/google", "go/mesh", "ruby/google", "ruby/mesh"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("code for a specification file was written to %s: %v", p, err)
		}
	}
}

// TestRun_MessageOutputIsWhatPlainProtocWrites compiles examples/pbx with
// protoc directly, with the same -I as the generator, and compares every
// message file the generator wrote to protoc's.
func TestRun_MessageOutputIsWhatPlainProtocWrites(t *testing.T) {
	out := t.TempDir()
	if code, _, stderr := run(t, "--definitions", examples, "-I", repoRoot, "--out", out, "--lang", "go,ruby"); code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	plain := t.TempDir()
	for _, p := range []string{"go", "ruby"} {
		if err := os.MkdirAll(filepath.Join(plain, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("protoc", "-I", examples, "-I", repoRoot,
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

func TestRun_IncludeDirectoriesFollowTheDefinitionsInOrder(t *testing.T) {
	first := t.TempDir()
	out := t.TempDir()
	code, stdout, stderr := run(t, "--definitions", examples, "-I", first, "-I", repoRoot, "--out", out, "--lang", "go,ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	want := "protoc --proto_path=" + examples + " --proto_path=" + first + " --proto_path=" + repoRoot + " --"
	if got := strings.Count(stdout, want); got != 3 {
		t.Fatalf("%d of 3 protoc runs start with %q:\n%s", got, want, stdout)
	}
}

func TestRun_ProtoPathWithSeparateValue(t *testing.T) {
	code, stdout, stderr := run(t, "--definitions", examples, "--proto_path", repoRoot, "--out", t.TempDir(), "--lang", "ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "protoc --proto_path="+examples+" --proto_path="+repoRoot+" --ruby_out=") {
		t.Fatalf("verbose output:\n%s", stdout)
	}
}

func TestRun_ProtoPathWithEquals(t *testing.T) {
	code, stdout, stderr := run(t, "--definitions", examples, "--proto_path="+repoRoot, "--out", t.TempDir(), "--lang", "ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "protoc --proto_path="+examples+" --proto_path="+repoRoot+" --ruby_out=") {
		t.Fatalf("verbose output:\n%s", stdout)
	}
}

func TestRun_MissingOptionsProtoIsOneErrorBeforeProtoc(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := run(t, "--definitions", examples, "-I", t.TempDir(), "--out", out, "--lang", "go,ruby", "--verbose")
	want := "grpc-service-mesh-gen: mesh/options.proto was not found on any -I path; pass -I " +
		`"$(go list -m -f '{{.Dir}}' github.com/Paymentbox-com/grpc-service-mesh-go)/proto" or -I "$(bundle info --path grpc_service_mesh)/proto"` + "\n"
	if code != 1 || stderr != want {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if strings.Contains(stdout, "protoc ") {
		t.Fatalf("protoc ran:\n%s", stdout)
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
	code, _, stderr := run(t, "--definitions", defs, "-I", repoRoot, "--out", t.TempDir(), "--lang", "go,ruby")
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
