package gen

import "testing"

func TestAnalyze_TransportUnset(t *testing.T) {
	got := analyzeErr(t, map[string]string{"shop/order.proto": shopService})
	mustContain(t, got, "shop: no file sets transport")
}

func TestAnalyze_TransportSetTwice(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto": shopService,
		"shop/a.proto":     shopDeployment,
		"shop/b.proto":     shopDeployment,
	})
	mustContain(t, got, "shop: transport is set in more than one file: shop/a.proto, shop/b.proto")
}

func TestAnalyze_DeploymentGroupConflict(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto": shopService,
		"shop/a.proto":     shopDeployment + "option (mesh.deployment_group) = \"one\";\n",
		"shop/b.proto":     header + "package shop;\noption (mesh.deployment_group) = \"two\";\n",
	})
	mustContain(t, got, `shop: deployment_group is set to different values: "one" in shop/a.proto, "two" in shop/b.proto`)
}

func TestAnalyze_DeploymentGroupRepeatedWithOneValueIsAllowed(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/order.proto": shopService,
		"shop/a.proto":     shopDeployment + "option (mesh.deployment_group) = \"one\";\n",
		"shop/b.proto":     header + "package shop;\noption (mesh.deployment_group) = \"one\";\n",
	})
	if m.Directories[0].DeploymentGroup != "one" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_TransportInNestedDirectory(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto":             shopService,
		"shop/deployment.proto":        shopDeployment,
		"shop/internal/settings.proto": header + "package shop.internal;\noption (mesh.transport) = \"http\";\n",
	})
	mustContain(t, got, "shop/internal/settings.proto: transport is set in a nested directory; only a file directly in shop/ sets it")
}

func TestAnalyze_DeploymentGroupInNestedDirectory(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto":             shopService,
		"shop/deployment.proto":        shopDeployment,
		"shop/internal/settings.proto": header + "package shop.internal;\noption (mesh.deployment_group) = \"other\";\n",
	})
	mustContain(t, got, "shop/internal/settings.proto: deployment_group is set in a nested directory; only a file directly in shop/ sets it")
}

func TestAnalyze_StreamingRPC(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/deployment.proto": shopDeployment,
		"shop/watch.proto": header + `package shop;
option go_package = "example.com/definitions/shop";
message Event {}
service WatchService { rpc Watch(Event) returns (stream Event); }
`,
	})
	mustContain(t, got, "shop/watch.proto: WatchService.Watch streams")
}

func TestAnalyze_ServiceAtDefinitionsRoot(t *testing.T) {
	got := analyzeErr(t, map[string]string{"order.proto": shopService})
	mustContain(t, got, "order.proto: a file that declares a service must live in a directory under the definitions root")
}

func TestAnalyze_TransportAtDefinitionsRoot(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"deployment.proto":      shopDeployment,
		"shop/order.proto":      shopService,
		"shop/deployment.proto": shopDeployment,
	})
	mustContain(t, got, "deployment.proto: transport and deployment_group apply to a top-level directory; a file at the definitions root sets neither")
}

func TestAnalyze_FileWithoutPackage(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto":      shopService,
		"shop/deployment.proto": shopDeployment,
		"common/id.proto":       "syntax = \"proto3\";\nmessage Id {}\n",
	})
	if got != "common/id.proto: declares no package" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAnalyze_OptionsFileMissingFromTheSet(t *testing.T) {
	got := analyzeErr(t, map[string]string{"common/id.proto": "syntax = \"proto3\";\npackage common;\nmessage Id {}\n"})
	if got != "mesh/options.proto is not in the descriptor set; no definitions file imports it" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAnalyze_MessagesOnlyFileAtRootIsAllowed(t *testing.T) {
	m := analyze(t, map[string]string{
		"shared.proto":          "syntax = \"proto3\";\npackage shared;\nmessage Id { string value = 1; }\n",
		"shop/order.proto":      shopService,
		"shop/deployment.proto": shopDeployment,
	})
	if len(m.Directories) != 1 || m.Directories[0].Path != "shop" {
		t.Fatalf("directories %+v", m.Directories)
	}
}

func TestAnalyze_ReportsEveryErrorOnItsOwnLine(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/order.proto": shopService,
		"billing/invoice.proto": header + `package billing;
option go_package = "example.com/definitions/billing";
message Invoice {}
service InvoiceService { rpc Get(Invoice) returns (Invoice); }
`,
	})
	if got != "billing: no file sets transport; exactly one file directly in billing/ must set option (mesh.transport)\n"+
		"shop: no file sets transport; exactly one file directly in shop/ must set option (mesh.transport)" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAnalyze_NestedDirectoryInheritsTransportAndDeploymentGroup(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/deployment.proto": shopDeployment + "option (mesh.deployment_group) = \"shop-prod\";\n",
		"shop/internal/audit.proto": header + `package shop.internal;
option go_package = "example.com/definitions/shop/internal";
message Entry {}
service AuditService { rpc Record(Entry) returns (Entry); }
`,
	})
	if len(m.Directories) != 1 {
		t.Fatalf("directories %+v", m.Directories)
	}
	d := m.Directories[0]
	if d.Path != "shop/internal" || d.Transport != "nats" || d.DeploymentGroup != "shop-prod" {
		t.Fatalf("directory %+v", d)
	}
}

func TestAnalyze_DeploymentGroupDefaultsToDirectoryName(t *testing.T) {
	m := analyze(t, map[string]string{"shop/order.proto": shopService, "shop/deployment.proto": shopDeployment})
	if m.Directories[0].DeploymentGroup != "shop" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_DeploymentGroupOverride(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/order.proto":      shopService,
		"shop/deployment.proto": shopDeployment + "option (mesh.deployment_group) = \"payments\";\n",
	})
	if m.Directories[0].DeploymentGroup != "payments" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_KindDefaultsToRouteWithoutConsumerGroup(t *testing.T) {
	m := analyze(t, map[string]string{"shop/order.proto": shopService, "shop/deployment.proto": shopDeployment})
	place := m.Directories[0].Services[0].Methods[0]
	if place.Name != "Place" || place.Kind != Route || place.ConsumerGroup != "" {
		t.Fatalf("method %+v", place)
	}
}

func TestAnalyze_KindTopicWithConsumerGroup(t *testing.T) {
	m := analyze(t, map[string]string{"shop/order.proto": shopService, "shop/deployment.proto": shopDeployment})
	placed := m.Directories[0].Services[0].Methods[1]
	if placed.Name != "Placed" || placed.Kind != Topic || placed.ConsumerGroup != "audit" {
		t.Fatalf("method %+v", placed)
	}
}

func TestAnalyze_MessageReferencesCarryTheirFile(t *testing.T) {
	m := analyze(t, map[string]string{"shop/order.proto": shopService, "shop/deployment.proto": shopDeployment})
	out := m.Directories[0].Services[0].Methods[1].Output
	want := MessageRef{FullName: "google.protobuf.Empty", Package: "google.protobuf", File: "google/protobuf/empty.proto", GoPackage: "google.golang.org/protobuf/types/known/emptypb"}
	if out != want {
		t.Fatalf("output %+v", out)
	}
}

func TestAnalyze_MessagesOnlyDirectoryGetsNoDirectory(t *testing.T) {
	m := analyze(t, map[string]string{
		"shop/order.proto":      shopService,
		"shop/deployment.proto": shopDeployment,
		"shop/types/id.proto":   "syntax = \"proto3\";\npackage shop.types;\nmessage Id {}\n",
		"common/id.proto":       "syntax = \"proto3\";\npackage common;\nmessage Id {}\n",
	})
	if len(m.Directories) != 1 || m.Directories[0].Path != "shop" {
		t.Fatalf("directories %+v", m.Directories)
	}
}

func TestModel_TransportsSortedAndUnique(t *testing.T) {
	m := &Model{Directories: []Directory{{Transport: "nats"}, {Transport: "http"}, {Transport: "nats"}}}
	got := m.Transports()
	if len(got) != 2 || got[0] != "http" || got[1] != "nats" {
		t.Fatalf("transports %v", got)
	}
}

func TestAnalyze_RootPrefixSetTwiceInOneDirectory(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/deployment.proto": shopDeployment + "option (mesh.root_prefix) = \"Shop\";\n",
		"shop/order.proto":      shopService + "option (mesh.root_prefix) = \"Shop\";\n",
	})
	if got != "shop: root_prefix is set in more than one file: shop/deployment.proto, shop/order.proto" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAnalyze_RootPrefixThatIsNotAnIdentifier(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"shop/deployment.proto": shopDeployment + "option (mesh.root_prefix) = \"shop_\";\n",
		"shop/order.proto":      shopService,
	})
	if got != `shop/deployment.proto: root_prefix "shop_" is not an identifier; an uppercase ASCII letter followed by ASCII letters and digits is expected` {
		t.Fatalf("got:\n%s", got)
	}
}
