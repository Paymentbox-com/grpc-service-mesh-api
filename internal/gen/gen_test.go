package gen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLangs_Both(t *testing.T) {
	got, err := ParseLangs("go,ruby")
	if err != nil || len(got) != 2 || got[0] != Go || got[1] != Ruby {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestParseLangs_RepeatedLanguageCountsOnce(t *testing.T) {
	got, err := ParseLangs("ruby,ruby")
	if err != nil || len(got) != 1 || got[0] != Ruby {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestParseLangs_Unknown(t *testing.T) {
	_, err := ParseLangs("go,java")
	if err == nil || err.Error() != `unknown language "java"; the languages are go and ruby` {
		t.Fatalf("err %v", err)
	}
}

func TestParseLangs_Empty(t *testing.T) {
	_, err := ParseLangs(",")
	if err == nil || err.Error() != "at least one language is required, as --lang go, --lang ruby, or --lang go,ruby" {
		t.Fatalf("err %v", err)
	}
}

func TestGenerate_OneFilePerDirectoryPlusServiceMapsPerLanguage(t *testing.T) {
	set := compile(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    pbxService,
		"pbx/internal/audit.proto": header + `package pbx.internal;
option go_package = "github.com/Paymentbox-com/pbx/internal";
message Entry {}
service AuditService { rpc Record(Entry) returns (Entry); }
`,
	})
	outs, err := Generate(set, Options{Langs: []Lang{Go, Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range outs {
		got = append(got, string(o.Lang)+"/"+o.Path)
	}
	want := []string{
		"go/pbx/pbx.grpcmesh.go", "go/pbx/internal/internal.grpcmesh.go", "go/servicemaps/servicemaps.go",
		"ruby/pbx/pbx_grpcmesh.rb", "ruby/pbx/internal/internal_grpcmesh.rb", "ruby/service_maps.rb",
	}
	if len(got) != len(want) {
		t.Fatalf("outputs %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("outputs %v", got)
		}
	}
}

func TestGenerate_OnlyRequestedLanguage(t *testing.T) {
	set := compile(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService})
	outs, err := Generate(set, Options{Langs: []Lang{Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 || outs[0].Lang != Ruby || outs[1].Lang != Ruby {
		t.Fatalf("outputs %+v", outs)
	}
}

func TestGenerate_RootFilesAtEachLanguageRoot(t *testing.T) {
	set := compile(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService})
	outs, err := Generate(set, Options{Langs: []Lang{Go, Ruby}, GoRootPackage: "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh", RubyRootModule: "PmtboxMesh"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range outs {
		got = append(got, string(o.Lang)+"/"+o.Path)
	}
	want := []string{
		"go/pbx/pbx.grpcmesh.go", "go/servicemaps/servicemaps.go", "go/pmtboxmesh.grpcmesh.go",
		"ruby/pbx/pbx_grpcmesh.rb", "ruby/service_maps.rb", "ruby/pmtbox_mesh_grpcmesh.rb",
	}
	if len(got) != len(want) {
		t.Fatalf("outputs %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("outputs %v", got)
		}
	}
}

func TestGenerate_RootFileOnlyForItsOwnLanguage(t *testing.T) {
	set := compile(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService})
	outs, err := Generate(set, Options{Langs: []Lang{Ruby}, GoRootPackage: "github.com/Paymentbox-com/pmtbox_mesh"})
	if err != nil || len(outs) != 2 {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestGenerate_RootErrorStopsEveryOutput(t *testing.T) {
	set := compile(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService})
	outs, err := Generate(set, Options{Langs: []Lang{Go, Ruby}, GoRootPackage: "github.com/Paymentbox-com/pbx", RubyRootModule: "PmtboxMesh"})
	if outs != nil || err == nil || err.Error() != "root package pbx: pbx/api_key.proto generates a package with the same name\n"+
		"root package pbx: pbx/deployment.proto generates a package with the same name" {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestGenerate_NothingWithoutServices(t *testing.T) {
	set := compile(t, map[string]string{"common/id.proto": "syntax = \"proto3\";\npackage common;\nmessage Id {}\n"})
	outs, err := Generate(set, Options{Langs: []Lang{Go, Ruby}})
	if err != nil || len(outs) != 0 {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestGenerate_NoOutputOnAnalysisError(t *testing.T) {
	set := compile(t, map[string]string{"pbx/api_key.proto": pbxService})
	outs, err := Generate(set, Options{Langs: []Lang{Go, Ruby}})
	if err == nil || outs != nil {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestGenerate_GoPackageErrorReportedOnceAcrossFiles(t *testing.T) {
	set := compile(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    header + "package pbx;\nmessage ApiKey {}\nservice ApiKeyService { rpc Search(ApiKey) returns (ApiKey); }\n",
	})
	_, err := Generate(set, Options{Langs: []Lang{Go, Ruby}})
	if err == nil || err.Error() != "pbx/api_key.proto: go_package is not set; every file in a directory that declares a service sets the same go_package" {
		t.Fatalf("err %v", err)
	}
}

func TestGenerate_RubyDoesNotNeedGoPackage(t *testing.T) {
	set := compile(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    header + "package pbx;\nmessage ApiKey {}\nservice ApiKeyService { rpc Search(ApiKey) returns (ApiKey); }\n",
	})
	outs, err := Generate(set, Options{Langs: []Lang{Ruby}})
	if err != nil || len(outs) != 2 {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestWrite_PutsFilesUnderEachLanguageRoot(t *testing.T) {
	out := t.TempDir()
	roots := map[Lang]string{Go: filepath.Join(out, "golang"), Ruby: filepath.Join(out, "rb")}
	paths, err := Write(roots, []Output{
		{Lang: Ruby, Path: "service_maps.rb", Content: []byte("ruby")},
		{Lang: Go, Path: "pbx/pbx.grpcmesh.go", Content: []byte("go")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != filepath.Join(out, "golang", "pbx", "pbx.grpcmesh.go") || paths[1] != filepath.Join(out, "rb", "service_maps.rb") {
		t.Fatalf("paths %v", paths)
	}
	b, err := os.ReadFile(paths[0])
	if err != nil || string(b) != "go" {
		t.Fatalf("content %q, %v", b, err)
	}
}

func TestWrite_LanguageWithoutARootIsAnError(t *testing.T) {
	_, err := Write(map[Lang]string{Go: t.TempDir()}, []Output{{Lang: Ruby, Path: "service_maps.rb"}})
	if err == nil || err.Error() != "no output root for ruby" {
		t.Fatalf("err %v", err)
	}
}

func TestReadSet_NotADescriptorSet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.pb")
	if err := os.WriteFile(p, []byte("\xff\xff\xff"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSet(p)
	if err == nil {
		t.Fatal("no error")
	}
	mustContain(t, err.Error(), "not a FileDescriptorSet")
}
