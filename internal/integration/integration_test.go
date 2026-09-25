// Package integration compiles and loads the generator's output for
// examples/pbx against the published libraries, and resolves the
// specification directory of a published version. The tests need network
// access, protoc, protoc-gen-go, go, and bundle, so they run only with
// GRPC_SERVICE_MESH_GEN_INTEGRATION set.
package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/cli"
	"github.com/Paymentbox-com/grpc-service-mesh-api/internal/specdir"
)

var repoRoot = filepath.Join("..", "..")

func generate(t *testing.T) string {
	t.Helper()
	if os.Getenv("GRPC_SERVICE_MESH_GEN_INTEGRATION") == "" {
		t.Skip("set GRPC_SERVICE_MESH_GEN_INTEGRATION=1 to run")
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"--definitions", filepath.Join(repoRoot, "examples"), "--out", out, "--lang", "go,ruby"}, &stdout, &stderr); code != 0 {
		t.Fatalf("generator exited %d:\n%s", code, stderr.String())
	}
	return out
}

// generateWithRoots generates a copy of examples/pbx whose go_package is a
// directory package of the root module, with both root options set.
func generateWithRoots(t *testing.T) string {
	t.Helper()
	if os.Getenv("GRPC_SERVICE_MESH_GEN_INTEGRATION") == "" {
		t.Skip("set GRPC_SERVICE_MESH_GEN_INTEGRATION=1 to run")
	}
	defs := filepath.Join(t.TempDir(), "definitions")
	if err := os.MkdirAll(filepath.Join(defs, "pbx"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"api_key.proto", "deployment.proto"} {
		b, err := os.ReadFile(filepath.Join(repoRoot, "examples", "pbx", name))
		if err != nil {
			t.Fatal(err)
		}
		src := strings.Replace(string(b), `option go_package = "github.com/Paymentbox-com/pbx";`, `option go_package = "github.com/Paymentbox-com/pmtbox_mesh/pbx";`, 1)
		write(t, filepath.Join(defs, "pbx", name), src)
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	args := []string{"--definitions", defs, "--out", out, "--lang", "go,ruby",
		"--go-root-package", "github.com/Paymentbox-com/pmtbox_mesh;pmtboxmesh", "--ruby-root-module", "PmtboxMesh"}
	if code := cli.Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("generator exited %d:\n%s", code, stderr.String())
	}
	return out
}

func sh(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, b)
	}
	return string(b)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGoOutputVets builds the generated pbx package as the module its
// go_package names and the servicemaps package as a module beside it.
func TestGoOutputVets(t *testing.T) {
	out := generate(t)
	pbx := filepath.Join(out, "go", "pbx")
	write(t, filepath.Join(pbx, "go.mod"), `module github.com/Paymentbox-com/pbx

go 1.26.6

require (
	github.com/Paymentbox-com/grpc-service-mesh-go v0.8.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
	google.golang.org/protobuf v1.36.12
)
`)
	sh(t, pbx, "go", "mod", "tidy")
	sh(t, pbx, "go", "vet", "./...")

	maps := filepath.Join(out, "go", "servicemaps")
	write(t, filepath.Join(maps, "go.mod"), `module example.com/servicemaps

go 1.26.6

require (
	github.com/Paymentbox-com/pbx v0.0.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
)

replace github.com/Paymentbox-com/pbx => ../pbx
`)
	sh(t, maps, "go", "mod", "tidy")
	sh(t, maps, "go", "vet", "./...")
}

// TestRubyOutputLoads requires the generated files against the published
// gems and reads the map and a client method back. mesh/options_pb comes
// from the grpc_service_mesh gem.
func TestRubyOutputLoads(t *testing.T) {
	out := generate(t)
	ruby := filepath.Join(out, "ruby")
	if _, err := os.Stat(filepath.Join(ruby, "mesh", "options_pb.rb")); !os.IsNotExist(err) {
		t.Fatalf("mesh/options_pb.rb is in the output: %v", err)
	}
	write(t, filepath.Join(ruby, "Gemfile"), `source "https://rubygems.org"

gem "grpc_service_mesh", git: "https://github.com/Paymentbox-com/grpc-service-mesh-ruby", tag: "v0.7.0"
gem "service_mesh", git: "https://github.com/Paymentbox-com/service-mesh-ruby", tag: "v0.4.0"
gem "google-protobuf"
gem "googleapis-common-protos-types"
`)
	sh(t, ruby, "bundle", "install", "--quiet")
	script := `require "service_maps"
raise "targets: #{ServiceMaps::NATS.targets.size}" unless ServiceMaps::NATS.targets.size == 2
raise "no search" unless Pbx::ApiKeyClient.respond_to?(:search)
raise "no created" unless Pbx::ApiKeyClient.respond_to?(:created)
raise "rpcs: #{Pbx::ApiKeyService.rpcs.keys}" unless Pbx::ApiKeyService.rpcs.keys.sort == [:created, :search]
options = $LOADED_FEATURES.grep(%r{/mesh/options_pb\.rb\z})
raise "mesh/options_pb from #{options}" unless options.size == 1 && options[0].end_with?("/lib/mesh/options_pb.rb") && options[0].include?("grpc-service-mesh-ruby")
puts "loaded"
`
	got := sh(t, ruby, "bundle", "exec", "ruby", "-I"+ruby, "-e", script)
	if strings.TrimSpace(got) != "loaded" {
		t.Fatalf("ruby output:\n%s", got)
	}
}

// TestGoRootPackageVets builds the Go output as one module: the root
// package at its root, the pbx directory package and servicemaps under it.
func TestGoRootPackageVets(t *testing.T) {
	out := generateWithRoots(t)
	goOut := filepath.Join(out, "go")
	write(t, filepath.Join(goOut, "go.mod"), `module github.com/Paymentbox-com/pmtbox_mesh

go 1.26.6

require (
	github.com/Paymentbox-com/grpc-service-mesh-go v0.8.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
	google.golang.org/protobuf v1.36.12
)
`)
	write(t, filepath.Join(goOut, "use_test.go"), `package pmtboxmesh

import "testing"

func TestAliases(t *testing.T) {
	if ApiKeyTargets.Search.Segments[2] != "Search" {
		t.Fatal(ApiKeyTargets.Search)
	}
	var _ ApiKeyService
	var _ *ApiKey
	_ = ApiKeyClient.Search
}
`)
	sh(t, goOut, "go", "mod", "tidy")
	sh(t, goOut, "go", "vet", "./...")
	sh(t, goOut, "go", "test", "./...")
}

// TestRubyRootModuleLoads requires the root file against the published gems
// and reads the aliases back.
func TestRubyRootModuleLoads(t *testing.T) {
	out := generateWithRoots(t)
	ruby := filepath.Join(out, "ruby")
	write(t, filepath.Join(ruby, "Gemfile"), `source "https://rubygems.org"

gem "grpc_service_mesh", git: "https://github.com/Paymentbox-com/grpc-service-mesh-ruby", tag: "v0.7.0"
gem "service_mesh", git: "https://github.com/Paymentbox-com/service-mesh-ruby", tag: "v0.4.0"
gem "google-protobuf"
gem "googleapis-common-protos-types"
`)
	sh(t, ruby, "bundle", "install", "--quiet")
	script := `require "pmtbox_mesh_grpcmesh"
raise "no search" unless PmtboxMesh::ApiKeyClient.respond_to?(:search)
raise "ApiKey" unless PmtboxMesh::ApiKey.equal?(Pbx::ApiKey)
raise "targets" unless PmtboxMesh::ApiKeyTargets::SEARCH.segments == ["pbx", "ApiKeyService", "Search"]
raise "service maps" unless PmtboxMesh::ServiceMaps::NATS.targets.size == 2
puts "loaded"
`
	got := sh(t, ruby, "bundle", "exec", "ruby", "-W", "-I"+ruby, "-e", script)
	if strings.TrimSpace(got) != "loaded" {
		t.Fatalf("ruby output:\n%s", got)
	}
}

func TestForVersion_ReleaseTakesTheModuleCache(t *testing.T) {
	if os.Getenv("GRPC_SERVICE_MESH_GEN_INTEGRATION") == "" {
		t.Skip("set GRPC_SERVICE_MESH_GEN_INTEGRATION=1 to run")
	}
	dir, err := specdir.ForVersion("v0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(dir), "/github.com/!paymentbox-com/grpc-service-mesh-api@v0.3.0") {
		t.Fatalf("dir %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "mesh", "options.proto")); err != nil {
		t.Fatal(err)
	}
}

func TestForVersion_UnknownReleaseIsAnError(t *testing.T) {
	if os.Getenv("GRPC_SERVICE_MESH_GEN_INTEGRATION") == "" {
		t.Skip("set GRPC_SERVICE_MESH_GEN_INTEGRATION=1 to run")
	}
	_, err := specdir.ForVersion("v0.0.99")
	if err == nil || !strings.HasPrefix(err.Error(), "go mod download github.com/Paymentbox-com/grpc-service-mesh-api@v0.0.99: ") {
		t.Fatalf("err %v", err)
	}
}
