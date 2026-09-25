package specdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckout_InThisModule(t *testing.T) {
	want, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Checkout()
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestCheckout_InAnotherModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/other\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	_, err := Checkout()
	if err == nil || !strings.HasSuffix(err.Error(), "; the working directory is in example.com/other") {
		t.Fatalf("err %v", err)
	}
}

func TestCheckout_OutsideAnyModule(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GOWORK", "off")
	_, err := Checkout()
	if err == nil || !strings.HasSuffix(err.Error(), "; the working directory is in no Go module") {
		t.Fatalf("err %v", err)
	}
}

func TestForVersion_DevelopmentBuildTakesTheCheckout(t *testing.T) {
	want, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ForVersion("(devel)")
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestForVersion_DirtyBuildTakesTheCheckout(t *testing.T) {
	want, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ForVersion("v0.4.1-0.20260925000000-abcdefabcdef+dirty")
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestCheckout_GoListFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("not a go.mod\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	_, err := Checkout()
	if err == nil || !strings.Contains(err.Error(), "in the working directory; go list -m: ") || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("err %v", err)
	}
}
