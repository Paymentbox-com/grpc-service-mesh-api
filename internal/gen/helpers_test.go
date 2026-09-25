package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
)

// repoRoot is the checkout, whose mesh/ directory is on the proto path of
// every fixture.
var repoRoot = filepath.Join("..", "..")

// compile writes the fixture files into a temporary definitions directory and
// returns the FileDescriptorSet protoc produces from them, with imports.
func compile(t *testing.T, files map[string]string) *descriptorpb.FileDescriptorSet {
	t.Helper()
	dir := t.TempDir()
	var names []string
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	out := filepath.Join(dir, "set.pb")
	args := append([]string{
		"--proto_path=" + dir, "--proto_path=" + repoRoot,
		"--include_imports", "--include_source_info", "--descriptor_set_out=" + out,
	}, names...)
	if b, err := exec.Command("protoc", args...).CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, b)
	}
	set, err := ReadSet(out)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// analyze compiles and analyzes, failing the test on any error.
func analyze(t *testing.T, files map[string]string) *Model {
	t.Helper()
	m, err := Analyze(compile(t, files))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return m
}

// analyzeErr compiles and analyzes, failing the test unless an error results.
func analyzeErr(t *testing.T, files map[string]string) string {
	t.Helper()
	_, err := Analyze(compile(t, files))
	if err == nil {
		t.Fatal("Analyze returned no error")
	}
	return err.Error()
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("missing %q in:\n%s", want, got)
	}
}

func mustNotContain(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("unexpected %q in:\n%s", want, got)
	}
}

const header = "syntax = \"proto3\";\nimport \"mesh/options.proto\";\n"

// shopDeployment is the settings file of the shop directory used by most fixtures.
const shopDeployment = header + "package shop;\noption go_package = \"example.com/definitions/shop\";\noption (mesh.transport) = \"nats\";\n"

// shopService declares shop.OrderService with a ROUTE and a TOPIC method.
const shopService = header + `package shop;
import "google/protobuf/empty.proto";
option go_package = "example.com/definitions/shop";
message Order { string id = 1; }
service OrderService {
  rpc Place(Order) returns (Order);
  rpc Placed(Order) returns (google.protobuf.Empty) {
    option (mesh.kind) = TOPIC;
    option (mesh.consumer_group) = "audit";
  }
}
`

// render runs one emitter on the only directory of the model.
func render(t *testing.T, files map[string]string, emit func(Directory) (string, []byte, error)) (string, string) {
	t.Helper()
	m := analyze(t, files)
	if len(m.Directories) != 1 {
		t.Fatalf("want one directory, got %d", len(m.Directories))
	}
	p, b, err := emit(m.Directories[0])
	if err != nil {
		t.Fatal(err)
	}
	return p, string(b)
}
