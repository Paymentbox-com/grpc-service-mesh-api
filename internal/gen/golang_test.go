package gen

import (
	"strings"
	"testing"
)

func TestGoFile_MissingGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      strings.Replace(shopService, "option go_package = \"example.com/definitions/shop\";\n", "", 1),
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != "shop/order.proto: go_package is not set; every definitions file sets go_package when Go is requested" {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_InconsistentGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"shop/other.proto":      "syntax = \"proto3\";\npackage shop;\noption go_package = \"example.com/definitions/other\";\nmessage Other {}\n",
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != `shop: go_package differs: "example.com/definitions/shop" in shop/deployment.proto, "example.com/definitions/other" in shop/other.proto` {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_PathAndPackageName(t *testing.T) {
	p, src := render(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService}, GoFile)
	if p != "shop/shop.grpcmesh.go" {
		t.Fatalf("path %q", p)
	}
	mustContain(t, src, "\npackage shop\n")
	mustContain(t, src, "// source: shop/order.proto\n")
	mustNotContain(t, src, "deployment.proto")
}

func TestGoFile_GoPackageWithExplicitName(t *testing.T) {
	_, src := render(t, map[string]string{
		"shop/deployment.proto": strings.Replace(shopDeployment, "example.com/definitions/shop", "example.com/definitions/shop/v2;shopv2", 1),
		"shop/order.proto":      strings.Replace(shopService, "example.com/definitions/shop", "example.com/definitions/shop/v2;shopv2", 1),
	}, GoFile)
	mustContain(t, src, "\npackage shopv2\n")
}

func TestGoFile_CrossPackageImportOfWellKnownType(t *testing.T) {
	_, src := render(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/ping.proto": header + `package shop;
import "google/protobuf/empty.proto";
option go_package = "example.com/definitions/shop";
service PingService { rpc Ping(google.protobuf.Empty) returns (google.protobuf.Empty); }
`,
	}, GoFile)
	mustContain(t, src, "\t\"google.golang.org/protobuf/types/known/emptypb\"\n")
	mustContain(t, src, "Ping func(context.Context, *emptypb.Empty) (*emptypb.Empty, error)")
	mustContain(t, src, "grpcmesh.Call[*emptypb.Empty, *emptypb.Empty](ctx, PingTargets.Ping, req)")
}

func TestGoFile_CrossDirectoryImportOfAPackageNamedAfterSemicolon(t *testing.T) {
	_, src := render(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/types/id.proto":   "syntax = \"proto3\";\npackage shop.types;\noption go_package = \"example.com/definitions/shop/types;shoptypes\";\nmessage Id { message Inner {} }\n",
		"shop/lookup.proto": header + `package shop;
import "shop/types/id.proto";
option go_package = "example.com/definitions/shop";
service LookupService { rpc Find(shop.types.Id) returns (shop.types.Id.Inner); }
`,
	}, GoFile)
	mustContain(t, src, "\t\"example.com/definitions/shop/types\"\n")
	mustContain(t, src, "Find func(context.Context, *shoptypes.Id) (*shoptypes.Id_Inner, error)")
}

func TestGoFile_TopicOutputIsNotImported(t *testing.T) {
	_, src := render(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService}, GoFile)
	mustNotContain(t, src, "emptypb")
}

func TestGoFile_ReferencedMessageWithoutGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"common/id.proto":       "syntax = \"proto3\";\npackage common;\nmessage Id {}\n",
		"shop/lookup.proto": header + `package shop;
import "common/id.proto";
option go_package = "example.com/definitions/shop";
service LookupService { rpc Find(common.Id) returns (common.Id); }
`,
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != "common/id.proto: go_package is not set; common.Id is referenced by a service" {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_ServiceSuffixStripped(t *testing.T) {
	_, src := render(t, map[string]string{"shop/deployment.proto": shopDeployment, "shop/order.proto": shopService}, GoFile)
	mustContain(t, src, "var OrderTargets = struct {")
	mustContain(t, src, "type OrderService struct {")
	mustContain(t, src, "type orderClient struct{}")
	mustContain(t, src, "var OrderClient orderClient")
}

func TestGoFile_ServiceNameKeptWhenOnlyTheSuffixRemains(t *testing.T) {
	_, src := render(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/service.proto": header + `package shop;
option go_package = "example.com/definitions/shop";
message Req {}
service Service { rpc Do(Req) returns (Req); }
`,
	}, GoFile)
	mustContain(t, src, "var ServiceTargets = struct {")
	mustContain(t, src, "type Service struct {")
	mustContain(t, src, "type serviceClient struct{}")
	mustContain(t, src, "var ServiceClient serviceClient")
}

func TestGoFile_TwoServicesInOneDirectoryShareOneFile(t *testing.T) {
	_, src := render(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"shop/ping.proto": header + `package shop;
option go_package = "example.com/definitions/shop";
message Pong {}
service PingService { rpc Ping(Pong) returns (Pong); }
`,
	}, GoFile)
	mustContain(t, src, "// source: shop/order.proto\n// source: shop/ping.proto\n")
	mustContain(t, src, "var OrderTargets = struct {")
	mustContain(t, src, "var PingTargets = struct {")
	if strings.Count(src, "import (") != 1 {
		t.Fatalf("one import block expected:\n%s", src)
	}
}

func TestGoServiceMaps_SecondTransportGetsSecondVar(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto":    shopDeployment,
		"shop/order.proto":         shopService,
		"billing/deployment.proto": header + "package billing;\noption go_package = \"example.com/definitions/billing\";\noption (mesh.transport) = \"http-json\";\n",
		"billing/invoice.proto": header + `package billing;
option go_package = "example.com/definitions/billing";
message Invoice {}
service InvoiceService { rpc Get(Invoice) returns (Invoice); }
`,
	})
	p, b, err := GoServiceMaps(m)
	if err != nil {
		t.Fatal(err)
	}
	if p != "servicemaps/servicemaps.go" {
		t.Fatalf("path %q", p)
	}
	src := string(b)
	mustContain(t, src, "\t\"example.com/definitions/billing\"\n\t\"example.com/definitions/shop\"\n\t\"github.com/Paymentbox-com/service-mesh-go/mesh\"\n")
	mustContain(t, src, "// HttpJson holds every Target served over transport \"http-json\".\nvar HttpJson = mesh.ServiceMap{Targets: []mesh.Target{\n\tbilling.InvoiceTargets.Get,\n}}\n")
	mustContain(t, src, "var Nats = mesh.ServiceMap{Targets: []mesh.Target{\n\tshop.OrderTargets.Place,\n\tshop.OrderTargets.Placed,\n}}\n")
}

func TestGoServiceMaps_SamePackageNameInTwoDirectoriesIsAliased(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/order.proto":      shopService,
		"shop/mesh/mesh.proto": header + `package shop.mesh;
option go_package = "example.com/definitions/shop/mesh";
message Node {}
service NodeService { rpc Get(Node) returns (Node); }
`,
	})
	_, b, err := GoServiceMaps(m)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(b), "\tmesh2 \"example.com/definitions/shop/mesh\"\n")
	mustContain(t, string(b), "\tmesh2.NodeTargets.Get,\n")
}
