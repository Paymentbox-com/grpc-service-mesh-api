package protoc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProtos_RelativeSlashPathsSorted(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"shop/deployment.proto", "shop/order.proto", "shop/internal/audit.proto", "README.md"} {
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
	want := []string{"shop/deployment.proto", "shop/internal/audit.proto", "shop/order.proto"}
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

func TestCheckOptions_IncludeEntryMayBeAPathList(t *testing.T) {
	r := &Runner{Definitions: t.TempDir(), Include: []string{t.TempDir() + string(os.PathListSeparator) + filepath.Join("..", "..")}}
	if err := r.CheckOptions(); err != nil {
		t.Fatal(err)
	}
}

func TestMessageFiles_SkipsCopiesOfThePublishedFiles(t *testing.T) {
	got := MessageFiles([]string{
		"google/rpc/code.proto", "google/rpc/error_details.proto", "google/rpc/status.proto",
		"mesh/options.proto", "shop/order.proto",
	})
	if len(got) != 1 || got[0] != "shop/order.proto" {
		t.Fatalf("got %v", got)
	}
}
