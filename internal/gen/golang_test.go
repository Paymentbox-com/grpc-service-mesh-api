package gen

import (
	"strings"
	"testing"
)

func TestGoFile_MissingGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    strings.Replace(pbxService, "option go_package = \"github.com/Paymentbox-com/pbx\";\n", "", 1),
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != "pbx/api_key.proto: go_package is not set; every file in a directory that declares a service sets the same go_package" {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_InconsistentGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    pbxService,
		"pbx/other.proto":      "syntax = \"proto3\";\npackage pbx;\noption go_package = \"github.com/Paymentbox-com/other\";\nmessage Other {}\n",
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != `pbx: go_package differs: "github.com/Paymentbox-com/pbx" in pbx/api_key.proto, "github.com/Paymentbox-com/other" in pbx/other.proto` {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_SettingsFileWithoutGoPackageGetsTheDirectoryImportPath(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService})
	if _, _, err := GoFile(m.Directories[0]); err != nil {
		t.Fatal(err)
	}
	got := GoImportOverrides(m)
	if len(got) != 1 || got["pbx/deployment.proto"] != "github.com/Paymentbox-com/pbx" {
		t.Fatalf("overrides %v", got)
	}
}

func TestGoFile_PathAndPackageName(t *testing.T) {
	p, src := render(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService}, GoFile)
	if p != "pbx/pbx.grpcmesh.go" {
		t.Fatalf("path %q", p)
	}
	mustContain(t, src, "\npackage pbx\n")
	mustContain(t, src, "// source: pbx/api_key.proto\n")
	mustNotContain(t, src, "deployment.proto")
}

func TestGoFile_GoPackageWithExplicitName(t *testing.T) {
	_, src := render(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    strings.Replace(pbxService, "github.com/Paymentbox-com/pbx", "github.com/Paymentbox-com/pbx/v2;pbxv2", 1),
	}, GoFile)
	mustContain(t, src, "\npackage pbxv2\n")
}

func TestGoFile_CrossPackageImportOfWellKnownType(t *testing.T) {
	_, src := render(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/ping.proto": header + `package pbx;
import "google/protobuf/empty.proto";
option go_package = "github.com/Paymentbox-com/pbx";
service PingService { rpc Ping(google.protobuf.Empty) returns (google.protobuf.Empty); }
`,
	}, GoFile)
	mustContain(t, src, "\t\"google.golang.org/protobuf/types/known/emptypb\"\n")
	mustContain(t, src, "Ping func(context.Context, *emptypb.Empty) (*emptypb.Empty, error)")
	mustContain(t, src, "grpcmesh.Call[*emptypb.Empty, *emptypb.Empty](ctx, PingTargets.Ping, req)")
}

func TestGoFile_CrossDirectoryImportWithAliasWhenNameDiffers(t *testing.T) {
	_, src := render(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/types/id.proto":   "syntax = \"proto3\";\npackage pbx.types;\noption go_package = \"github.com/Paymentbox-com/pbx/types;pbxtypes\";\nmessage Id { message Inner {} }\n",
		"pbx/lookup.proto": header + `package pbx;
import "pbx/types/id.proto";
option go_package = "github.com/Paymentbox-com/pbx";
service LookupService { rpc Find(pbx.types.Id) returns (pbx.types.Id.Inner); }
`,
	}, GoFile)
	mustContain(t, src, "\tpbxtypes \"github.com/Paymentbox-com/pbx/types\"\n")
	mustContain(t, src, "Find func(context.Context, *pbxtypes.Id) (*pbxtypes.Id_Inner, error)")
}

func TestGoFile_TopicOutputIsNotImported(t *testing.T) {
	_, src := render(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService}, GoFile)
	mustNotContain(t, src, "emptypb")
}

func TestGoFile_ReferencedMessageWithoutGoPackage(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"common/id.proto":      "syntax = \"proto3\";\npackage common;\nmessage Id {}\n",
		"pbx/lookup.proto": header + `package pbx;
import "common/id.proto";
option go_package = "github.com/Paymentbox-com/pbx";
service LookupService { rpc Find(common.Id) returns (common.Id); }
`,
	})
	_, _, err := GoFile(m.Directories[0])
	if err == nil || err.Error() != "common/id.proto: go_package is not set; common.Id is referenced by a service" {
		t.Fatalf("err %v", err)
	}
}

func TestGoFile_ServiceSuffixStripped(t *testing.T) {
	_, src := render(t, map[string]string{"pbx/deployment.proto": pbxDeployment, "pbx/api_key.proto": pbxService}, GoFile)
	mustContain(t, src, "var ApiKeyTargets = struct {")
	mustContain(t, src, "type ApiKeyService struct {")
	mustContain(t, src, "type apiKeyClient struct{}")
	mustContain(t, src, "var ApiKeyClient apiKeyClient")
}

func TestGoFile_ServiceNameKeptWhenOnlyTheSuffixRemains(t *testing.T) {
	_, src := render(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/service.proto": header + `package pbx;
option go_package = "github.com/Paymentbox-com/pbx";
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
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    pbxService,
		"pbx/ping.proto": header + `package pbx;
option go_package = "github.com/Paymentbox-com/pbx";
message Pong {}
service PingService { rpc Ping(Pong) returns (Pong); }
`,
	}, GoFile)
	mustContain(t, src, "// source: pbx/api_key.proto\n// source: pbx/ping.proto\n")
	mustContain(t, src, "var ApiKeyTargets = struct {")
	mustContain(t, src, "var PingTargets = struct {")
	if strings.Count(src, "import (") != 1 {
		t.Fatalf("one import block expected:\n%s", src)
	}
}

func TestGoServiceMaps_SecondTransportGetsSecondVar(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto":     pbxDeployment,
		"pbx/api_key.proto":        pbxService,
		"billing/deployment.proto": header + "package billing;\noption (mesh.transport) = \"http-json\";\n",
		"billing/invoice.proto": header + `package billing;
option go_package = "github.com/Paymentbox-com/billing";
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
	mustContain(t, src, "\t\"github.com/Paymentbox-com/billing\"\n\t\"github.com/Paymentbox-com/pbx\"\n\t\"github.com/Paymentbox-com/service-mesh-go/mesh\"\n")
	mustContain(t, src, "// HttpJson holds every Target served over transport \"http-json\".\nvar HttpJson = mesh.ServiceMap{Targets: []mesh.Target{\n\tbilling.InvoiceTargets.Get,\n}}\n")
	mustContain(t, src, "var Nats = mesh.ServiceMap{Targets: []mesh.Target{\n\tpbx.ApiKeyTargets.Search,\n\tpbx.ApiKeyTargets.Created,\n}}\n")
}

func TestGoServiceMaps_SamePackageNameInTwoDirectoriesIsAliased(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/api_key.proto":    pbxService,
		"pbx/mesh/mesh.proto": header + `package pbx.mesh;
option go_package = "github.com/Paymentbox-com/pbx/mesh";
message Node {}
service NodeService { rpc Get(Node) returns (Node); }
`,
	})
	_, b, err := GoServiceMaps(m)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(b), "\tmesh2 \"github.com/Paymentbox-com/pbx/mesh\"\n")
	mustContain(t, string(b), "\tmesh2.NodeTargets.Get,\n")
}
