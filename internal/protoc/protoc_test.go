package protoc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProtos_RelativeSlashPathsSorted(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"pbx/deployment.proto", "pbx/api_key.proto", "pbx/internal/audit.proto", "README.md"} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := FindProtos(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pbx/api_key.proto", "pbx/deployment.proto", "pbx/internal/audit.proto"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}

func TestFindProtos_NoneIsAnError(t *testing.T) {
	_, err := FindProtos(t.TempDir())
	if err == nil {
		t.Fatal("no error")
	}
}

func TestWriteEmbedded_WritesEveryFileAtItsImportPath(t *testing.T) {
	root, err := WriteEmbedded(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"mesh/options.proto", "google/rpc/code.proto", "google/rpc/status.proto", "google/rpc/error_details.proto"} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil || len(b) == 0 {
			t.Fatalf("%s: %v", p, err)
		}
	}
}

func TestGoMessageFiles_SkipsTheSpecificationFiles(t *testing.T) {
	got := GoMessageFiles([]string{
		"google/rpc/code.proto", "google/rpc/error_details.proto", "google/rpc/status.proto",
		"mesh/options.proto", "pbx/api_key.proto",
	})
	if len(got) != 1 || got[0] != "pbx/api_key.proto" {
		t.Fatalf("got %v", got)
	}
}

func TestRubyMessageFiles_SkipsGoogleRPCAndAddsOptions(t *testing.T) {
	got := RubyMessageFiles([]string{"google/rpc/status.proto", "pbx/api_key.proto"})
	if len(got) != 2 || got[0] != "pbx/api_key.proto" || got[1] != "mesh/options.proto" {
		t.Fatalf("got %v", got)
	}
}

func TestRubyMessageFiles_KeepsAVendoredOptionsFileOnce(t *testing.T) {
	got := RubyMessageFiles([]string{"mesh/options.proto", "pbx/api_key.proto"})
	if len(got) != 2 || got[0] != "mesh/options.proto" || got[1] != "pbx/api_key.proto" {
		t.Fatalf("got %v", got)
	}
}
