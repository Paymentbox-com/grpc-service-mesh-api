package gen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGolden_ExamplesPbx generates examples/pbx twice, without and with the
// root package options, and compares every file to testdata/golden. The
// directory and ServiceMaps files are the same in both runs. UPDATE_GOLDEN=1
// rewrites the golden files instead.
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
	plain, err := Generate(set, Options{Langs: []Lang{Go, Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	rooted, err := Generate(set, Options{
		Langs:          []Lang{Go, Ruby},
		GoRootPackage:  "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh",
		RubyRootModule: "PmtboxMesh",
	})
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if _, err := Write(map[Lang]string{Go: filepath.Join(golden, "go"), Ruby: filepath.Join(golden, "ruby")}, rooted); err != nil {
			t.Fatal(err)
		}
	}
	shared := []string{"go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go", "ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb"}
	compareGolden(t, golden, plain, shared)
	compareGolden(t, golden, rooted, append(shared, "go/pmtboxmesh.grpcmesh.go", "ruby/pmtbox_mesh_grpcmesh.rb"))
}

// compareGolden checks that outs are exactly the files in want and that each
// matches its golden file.
func compareGolden(t *testing.T, golden string, outs []Output, want []string) {
	t.Helper()
	missing := map[string]bool{}
	for _, rel := range want {
		missing[rel] = true
	}
	for _, o := range outs {
		rel := string(o.Lang) + "/" + o.Path
		if !missing[rel] {
			t.Errorf("unexpected output %s", rel)
			continue
		}
		delete(missing, rel)
		b, err := os.ReadFile(filepath.Join(golden, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(o.Content) {
			t.Errorf("%s differs from golden:\n%s", rel, o.Content)
		}
	}
	for rel := range missing {
		t.Errorf("missing output %s", rel)
	}
}
