package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGolden_ExamplesPbx generates examples/pbx and compares every file to
// testdata/golden. UPDATE_GOLDEN=1 rewrites the golden files instead.
func TestGolden_ExamplesPbx(t *testing.T) {
	out := filepath.Join(t.TempDir(), "set.pb")
	cmd := exec.Command("protoc",
		"--proto_path="+filepath.Join(repoRoot, "examples"), "--proto_path="+repoRoot,
		"--include_imports", "--include_source_info", "--descriptor_set_out="+out,
		"pbx/api_key.proto", "pbx/deployment.proto")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, b)
	}
	set, err := ReadSet(out)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := Generate(set, []Lang{Go, Ruby})
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if _, err := Write(golden, outs); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]bool{
		"go/pbx/pbx.grpcmesh.go":        true,
		"go/servicemaps/servicemaps.go": true,
		"ruby/pbx/pbx_grpcmesh.rb":      true,
		"ruby/service_maps.rb":          true,
	}
	for _, o := range outs {
		rel := string(o.Lang) + "/" + o.Path
		if !want[rel] {
			t.Errorf("unexpected output %s", rel)
			continue
		}
		delete(want, rel)
		b, err := os.ReadFile(filepath.Join(golden, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(o.Content) {
			t.Errorf("%s differs from golden:\n%s", rel, o.Content)
		}
	}
	for rel := range want {
		t.Errorf("missing output %s", rel)
	}
}
