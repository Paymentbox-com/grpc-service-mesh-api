package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var repoRoot = filepath.Join("..", "..")

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRun_Help(t *testing.T) {
	code, _, stderr := run(t, "--help")
	if code != 0 || !strings.Contains(stderr, "--definitions <dir>") || !strings.Contains(stderr, "--descriptors <file>") ||
		!strings.Contains(stderr, "--out <dir>") || !strings.Contains(stderr, "--lang <list>") || !strings.Contains(stderr, "--verbose") {
		t.Fatalf("code %d, help:\n%s", code, stderr)
	}
}

func TestRun_NeitherInputIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--out", t.TempDir(), "--lang", "go")
	if code != 2 || !strings.Contains(stderr, "exactly one of --definitions and --descriptors is required") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_BothInputsIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--definitions", "x", "--descriptors", "y", "--out", t.TempDir(), "--lang", "go")
	if code != 2 || !strings.Contains(stderr, "exactly one of --definitions and --descriptors is required") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_OutRequired(t *testing.T) {
	code, _, stderr := run(t, "--descriptors", "x.pb", "--lang", "go")
	if code != 2 || !strings.Contains(stderr, "--out is required") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_UnknownLanguageIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--descriptors", "x.pb", "--out", t.TempDir(), "--lang", "go,java")
	if code != 2 || !strings.Contains(stderr, `unknown language "java"`) {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_PositionalArgumentIsAUsageError(t *testing.T) {
	code, _, stderr := run(t, "--descriptors", "x.pb", "--out", t.TempDir(), "--lang", "go", "extra")
	if code != 2 || !strings.Contains(stderr, "unexpected argument extra") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func TestRun_MissingDescriptorsFileExitsOne(t *testing.T) {
	code, _, stderr := run(t, "--descriptors", filepath.Join(t.TempDir(), "none.pb"), "--out", t.TempDir(), "--lang", "go")
	if code != 1 || !strings.HasPrefix(stderr, "grpc-service-mesh-gen: ") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}

func pbxSet(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "set.pb")
	cmd := exec.Command("protoc",
		"--proto_path="+filepath.Join(repoRoot, "examples"), "--proto_path="+repoRoot,
		"--include_imports", "--include_source_info", "--descriptor_set_out="+out,
		"pbx/api_key.proto", "pbx/deployment.proto")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, b)
	}
	return out
}

func TestRun_DescriptorsWritesGeneratedFilesOnly(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := run(t, "--descriptors", pbxSet(t), "--out", out, "--lang", "go,ruby")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	for _, p := range []string{"go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go", "ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "go", "pbx", "api_key.pb.go")); !os.IsNotExist(err) {
		t.Errorf("message code was written without --definitions: %v", err)
	}
}

func TestRun_DescriptorsOnlyRequestedLanguage(t *testing.T) {
	out := t.TempDir()
	code, _, stderr := run(t, "--descriptors", pbxSet(t), "--out", out, "--lang", "ruby")
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "go")); !os.IsNotExist(err) {
		t.Fatalf("go directory written: %v", err)
	}
}

func TestRun_GeneratorErrorsOneLineEachExitOne(t *testing.T) {
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
	code, _, stderr := run(t, "--definitions", defs, "--out", out, "--lang", "go,ruby")
	want := "grpc-service-mesh-gen: pbx: no file sets transport; exactly one file directly in pbx/ must set option (mesh.transport)\n" +
		"grpc-service-mesh-gen: pbx/api_key.proto: ApiKeyService.Watch streams; a streaming rpc has no Service Mesh API form\n"
	if code != 1 || stderr != want {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Fatalf("output written despite errors: %v", entries)
	}
}

func TestRun_DefinitionsRunsProtocAndWritesEverything(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := run(t, "--definitions", filepath.Join(repoRoot, "examples"), "--out", out, "--lang", "go,ruby", "--verbose")
	if code != 0 {
		t.Fatalf("code %d, stderr:\n%s", code, stderr)
	}
	for _, p := range []string{
		"go/pbx/api_key.pb.go", "go/pbx/deployment.pb.go", "go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go",
		"ruby/mesh/options_pb.rb", "ruby/pbx/api_key_pb.rb", "ruby/pbx/deployment_pb.rb", "ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb",
	} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "go", "mesh")); !os.IsNotExist(err) {
		t.Errorf("Go code for mesh/options.proto was written: %v", err)
	}
	for _, want := range []string{
		"--include_imports --include_source_info --descriptor_set_out=",
		"--go_out=" + filepath.Join(out, "go") + " --go_opt=paths=source_relative --go_opt=Mmesh/options.proto=github.com/Paymentbox-com/grpc-service-mesh-api/mesh --go_opt=Mpbx/deployment.proto=github.com/Paymentbox-com/pbx pbx/api_key.proto pbx/deployment.proto",
		"--ruby_out=" + filepath.Join(out, "ruby") + " pbx/api_key.proto pbx/deployment.proto mesh/options.proto",
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
	if !strings.Contains(stdout, "--go_opt=Mpbx/deployment.proto=github.com/Paymentbox-com/pbx pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Go run did not skip the specification files:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--ruby_out="+filepath.Join(out, "ruby")+" mesh/options.proto pbx/api_key.proto pbx/deployment.proto\n") {
		t.Errorf("Ruby run did not skip google/rpc or dropped the vendored options file:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(out, "go", "google")); !os.IsNotExist(err) {
		t.Errorf("Go code for google/rpc was written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "ruby", "google")); !os.IsNotExist(err) {
		t.Errorf("Ruby code for google/rpc was written: %v", err)
	}
}
