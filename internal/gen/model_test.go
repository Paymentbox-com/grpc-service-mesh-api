package gen

import "testing"

func TestAnalyze_TransportUnset(t *testing.T) {
	got := analyzeErr(t, map[string]string{"pbx/api_key.proto": pbxService})
	mustContain(t, got, "pbx: no file sets transport")
}

func TestAnalyze_TransportSetTwice(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto": pbxService,
		"pbx/a.proto":       pbxDeployment,
		"pbx/b.proto":       pbxDeployment,
	})
	mustContain(t, got, "pbx: transport is set in more than one file: pbx/a.proto, pbx/b.proto")
}

func TestAnalyze_DeploymentGroupConflict(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto": pbxService,
		"pbx/a.proto":       pbxDeployment + "option (mesh.deployment_group) = \"one\";\n",
		"pbx/b.proto":       header + "package pbx;\noption (mesh.deployment_group) = \"two\";\n",
	})
	mustContain(t, got, `pbx: deployment_group is set to different values: "one" in pbx/a.proto, "two" in pbx/b.proto`)
}

func TestAnalyze_DeploymentGroupRepeatedWithOneValueIsAllowed(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/api_key.proto": pbxService,
		"pbx/a.proto":       pbxDeployment + "option (mesh.deployment_group) = \"one\";\n",
		"pbx/b.proto":       header + "package pbx;\noption (mesh.deployment_group) = \"one\";\n",
	})
	if m.Directories[0].DeploymentGroup != "one" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_TransportInNestedDirectory(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto":           pbxService,
		"pbx/deployment.proto":        pbxDeployment,
		"pbx/internal/settings.proto": header + "package pbx.internal;\noption (mesh.transport) = \"http\";\n",
	})
	mustContain(t, got, "pbx/internal/settings.proto: transport is set in a nested directory; only a file directly in pbx/ sets it")
}

func TestAnalyze_DeploymentGroupInNestedDirectory(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto":           pbxService,
		"pbx/deployment.proto":        pbxDeployment,
		"pbx/internal/settings.proto": header + "package pbx.internal;\noption (mesh.deployment_group) = \"other\";\n",
	})
	mustContain(t, got, "pbx/internal/settings.proto: deployment_group is set in a nested directory; only a file directly in pbx/ sets it")
}

func TestAnalyze_StreamingRPC(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment,
		"pbx/watch.proto": header + `package pbx;
option go_package = "github.com/Paymentbox-com/pbx";
message Event {}
service WatchService { rpc Watch(Event) returns (stream Event); }
`,
	})
	mustContain(t, got, "pbx/watch.proto: WatchService.Watch streams")
}

func TestAnalyze_ServiceAtDefinitionsRoot(t *testing.T) {
	got := analyzeErr(t, map[string]string{"api_key.proto": pbxService})
	mustContain(t, got, "api_key.proto: a file that declares a service must live in a directory under the definitions root")
}

func TestAnalyze_TransportAtDefinitionsRoot(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"deployment.proto":     pbxDeployment,
		"pbx/api_key.proto":    pbxService,
		"pbx/deployment.proto": pbxDeployment,
	})
	mustContain(t, got, "deployment.proto: transport and deployment_group apply to a top-level directory; a file at the definitions root sets neither")
}

func TestAnalyze_FileWithoutPackage(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto":    pbxService,
		"pbx/deployment.proto": pbxDeployment,
		"common/id.proto":      "syntax = \"proto3\";\nmessage Id {}\n",
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
		"shared.proto":         "syntax = \"proto3\";\npackage shared;\nmessage Id { string value = 1; }\n",
		"pbx/api_key.proto":    pbxService,
		"pbx/deployment.proto": pbxDeployment,
	})
	if len(m.Directories) != 1 || m.Directories[0].Path != "pbx" {
		t.Fatalf("directories %+v", m.Directories)
	}
}

func TestAnalyze_ReportsEveryErrorOnItsOwnLine(t *testing.T) {
	got := analyzeErr(t, map[string]string{
		"pbx/api_key.proto": pbxService,
		"billing/invoice.proto": header + `package billing;
option go_package = "github.com/Paymentbox-com/billing";
message Invoice {}
service InvoiceService { rpc Get(Invoice) returns (Invoice); }
`,
	})
	if got != "billing: no file sets transport; exactly one file directly in billing/ must set option (mesh.transport)\n"+
		"pbx: no file sets transport; exactly one file directly in pbx/ must set option (mesh.transport)" {
		t.Fatalf("got:\n%s", got)
	}
}

func TestAnalyze_NestedDirectoryInheritsTransportAndDeploymentGroup(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/deployment.proto": pbxDeployment + "option (mesh.deployment_group) = \"pbx-prod\";\n",
		"pbx/internal/audit.proto": header + `package pbx.internal;
option go_package = "github.com/Paymentbox-com/pbx/internal";
message Entry {}
service AuditService { rpc Record(Entry) returns (Entry); }
`,
	})
	if len(m.Directories) != 1 {
		t.Fatalf("directories %+v", m.Directories)
	}
	d := m.Directories[0]
	if d.Path != "pbx/internal" || d.Transport != "nats" || d.DeploymentGroup != "pbx-prod" {
		t.Fatalf("directory %+v", d)
	}
}

func TestAnalyze_DeploymentGroupDefaultsToDirectoryName(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/api_key.proto": pbxService, "pbx/deployment.proto": pbxDeployment})
	if m.Directories[0].DeploymentGroup != "pbx" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_DeploymentGroupOverride(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/api_key.proto":    pbxService,
		"pbx/deployment.proto": pbxDeployment + "option (mesh.deployment_group) = \"payments\";\n",
	})
	if m.Directories[0].DeploymentGroup != "payments" {
		t.Fatalf("deployment group %q", m.Directories[0].DeploymentGroup)
	}
}

func TestAnalyze_KindDefaultsToRouteWithoutConsumerGroup(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/api_key.proto": pbxService, "pbx/deployment.proto": pbxDeployment})
	search := m.Directories[0].Services[0].Methods[0]
	if search.Name != "Search" || search.Kind != Route || search.ConsumerGroup != "" {
		t.Fatalf("method %+v", search)
	}
}

func TestAnalyze_KindTopicWithConsumerGroup(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/api_key.proto": pbxService, "pbx/deployment.proto": pbxDeployment})
	created := m.Directories[0].Services[0].Methods[1]
	if created.Name != "Created" || created.Kind != Topic || created.ConsumerGroup != "audit" {
		t.Fatalf("method %+v", created)
	}
}

func TestAnalyze_MessageReferencesCarryTheirFile(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/api_key.proto": pbxService, "pbx/deployment.proto": pbxDeployment})
	out := m.Directories[0].Services[0].Methods[1].Output
	want := MessageRef{FullName: "google.protobuf.Empty", Package: "google.protobuf", File: "google/protobuf/empty.proto", GoPackage: "google.golang.org/protobuf/types/known/emptypb"}
	if out != want {
		t.Fatalf("output %+v", out)
	}
}

func TestAnalyze_MessagesOnlyDirectoryGetsNoDirectory(t *testing.T) {
	m := analyze(t, map[string]string{
		"pbx/api_key.proto":    pbxService,
		"pbx/deployment.proto": pbxDeployment,
		"pbx/types/id.proto":   "syntax = \"proto3\";\npackage pbx.types;\nmessage Id {}\n",
		"common/id.proto":      "syntax = \"proto3\";\npackage common;\nmessage Id {}\n",
	})
	if len(m.Directories) != 1 || m.Directories[0].Path != "pbx" {
		t.Fatalf("directories %+v", m.Directories)
	}
}

func TestAnalyze_SettingsFileIsMarkedEmpty(t *testing.T) {
	m := analyze(t, map[string]string{"pbx/api_key.proto": pbxService, "pbx/deployment.proto": pbxDeployment})
	files := m.Directories[0].Files
	if files[0].Path != "pbx/api_key.proto" || files[0].Empty || files[1].Path != "pbx/deployment.proto" || !files[1].Empty {
		t.Fatalf("files %+v", files)
	}
}

func TestModel_TransportsSortedAndUnique(t *testing.T) {
	m := &Model{Directories: []Directory{{Transport: "nats"}, {Transport: "http"}, {Transport: "nats"}}}
	got := m.Transports()
	if len(got) != 2 || got[0] != "http" || got[1] != "nats" {
		t.Fatalf("transports %v", got)
	}
}
