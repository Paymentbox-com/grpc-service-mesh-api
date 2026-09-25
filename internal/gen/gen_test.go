package gen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRender_OneFilePerDirectoryPlusServiceMapsPerLanguage(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"shop/internal/audit.proto": header + `package shop.internal;
option go_package = "example.com/definitions/shop/internal";
message Entry {}
service AuditService { rpc Record(Entry) returns (Entry); }
`,
	})
	outs, err := Render(m, Options{Langs: []Lang{Go, Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range outs {
		got = append(got, string(o.Lang)+"/"+o.Path)
	}
	want := []string{
		"go/shop/shop.grpcmesh.go", "go/shop/internal/internal.grpcmesh.go", "go/servicemaps/servicemaps.go",
		"ruby/shop/shop_grpcmesh.rb", "ruby/shop/internal/internal_grpcmesh.rb", "ruby/service_maps.rb",
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

func TestRender_OnlyRequestedLanguage(t *testing.T) {
	m := analyze(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService})
	outs, err := Render(m, Options{Langs: []Lang{Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 || outs[0].Lang != Ruby || outs[1].Lang != Ruby {
		t.Fatalf("outputs %+v", outs)
	}
}

func TestRender_RootFilesAtEachLanguageRoot(t *testing.T) {
	m := analyze(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService})
	outs, err := Render(m, Options{Langs: []Lang{Go, Ruby}, GoRootPackage: "example.com/definitions;definitions", RubyRootModule: "Definitions"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range outs {
		got = append(got, string(o.Lang)+"/"+o.Path)
	}
	want := []string{
		"go/shop/shop.grpcmesh.go", "go/servicemaps/servicemaps.go", "go/definitions.grpcmesh.go",
		"ruby/shop/shop_grpcmesh.rb", "ruby/service_maps.rb", "ruby/definitions_grpcmesh.rb",
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

func TestRender_RootFileOnlyForItsOwnLanguage(t *testing.T) {
	m := analyze(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService})
	outs, err := Render(m, Options{Langs: []Lang{Ruby}, GoRootPackage: "example.com/definitions"})
	if err != nil || len(outs) != 2 {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestRender_RootErrorStopsEveryOutput(t *testing.T) {
	m := analyze(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService})
	outs, err := Render(m, Options{Langs: []Lang{Go, Ruby}, GoRootPackage: "example.com/definitions/shop", RubyRootModule: "Definitions"})
	if outs != nil || err == nil || err.Error() != "root package shop: shop/deployment.proto generates a package with the same name\n"+
		"root package shop: shop/order.proto generates a package with the same name" {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestRender_GoNeedsGoPackageInEveryDefinitionsFile(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"common/id.proto":       "syntax = \"proto3\";\npackage common;\nmessage Id {}\n",
	})
	outs, err := Render(m, Options{Langs: []Lang{Ruby, Go}})
	if outs != nil || err == nil || err.Error() != "common/id.proto: go_package is not set; every definitions file sets go_package when Go is requested" {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestRender_RubyDoesNotNeedGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      header + "package shop;\nmessage Order {}\nservice OrderService { rpc Place(Order) returns (Order); }\n",
	})
	outs, err := Render(m, Options{Langs: []Lang{Ruby}})
	if err != nil || len(outs) != 2 {
		t.Fatalf("outputs %+v, err %v", outs, err)
	}
}

func TestWrite_PutsFilesUnderEachLanguageRoot(t *testing.T) {
	out := t.TempDir()
	roots := map[Lang]string{Go: filepath.Join(out, "golang"), Ruby: filepath.Join(out, "rb")}
	paths, err := Write(roots, []Output{
		{Lang: Ruby, Path: "service_maps.rb", Content: []byte("ruby")},
		{Lang: Go, Path: "shop/shop.grpcmesh.go", Content: []byte("go")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != filepath.Join(out, "rb", "service_maps.rb") || paths[1] != filepath.Join(out, "golang", "shop", "shop.grpcmesh.go") {
		t.Fatalf("paths %v", paths)
	}
	b, err := os.ReadFile(paths[1])
	if err != nil || string(b) != "go" {
		t.Fatalf("content %q, %v", b, err)
	}
}

func TestRender_RootPrefixResolvesARootCollision(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"billing/deployment.proto": header + `package billing;
option go_package = "example.com/definitions/billing";
option (mesh.transport) = "nats";
option (mesh.root_prefix) = "Billing";
message Order {}
service OrderService { rpc Get(Order) returns (Order); }
`,
	})
	outs, err := Render(m, Options{Langs: []Lang{Go, Ruby}, GoRootPackage: "example.com/definitions", RubyRootModule: "Definitions"})
	if err != nil {
		t.Fatal(err)
	}
	var goSrc, rubySrc string
	for _, o := range outs {
		switch o.Path {
		case "definitions.grpcmesh.go":
			goSrc = string(o.Content)
		case "definitions_grpcmesh.rb":
			rubySrc = string(o.Content)
		}
	}
	mustContain(t, goSrc, "type BillingOrder = billing.Order\n")
	mustContain(t, goSrc, "type Order = shop.Order\n")
	mustContain(t, goSrc, "var BillingOrderClient = billing.OrderClient\n")
	mustContain(t, goSrc, "var OrderClient = shop.OrderClient\n")
	mustContain(t, rubySrc, "  BillingOrder = ::Billing::Order\n")
	mustContain(t, rubySrc, "  Order = ::Shop::Order\n")
	mustContain(t, rubySrc, "  BillingOrderClient = ::Billing::OrderClient\n")
	mustContain(t, rubySrc, "  OrderClient = ::Shop::OrderClient\n")
}

func TestRender_RootPrefixWithoutARootFileHasNoEffect(t *testing.T) {
	plain := analyze(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService})
	prefixed := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment + "option (mesh.root_prefix) = \"Shop\";\n",
		"shop/order.proto":      shopService,
	})
	want, err := Render(plain, Options{Langs: []Lang{Go, Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Render(prefixed, Options{Langs: []Lang{Go, Ruby}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("outputs %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Path != want[i].Path || string(got[i].Content) != string(want[i].Content) {
			t.Fatalf("%s differs:\n%s", got[i].Path, got[i].Content)
		}
	}
}
