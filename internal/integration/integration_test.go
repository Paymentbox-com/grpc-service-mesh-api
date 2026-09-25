// Package integration compiles and loads the generator's output for
// examples/shop against the published libraries, and resolves the
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

// generate runs the generator on examples with args added.
func generate(t *testing.T, args ...string) string {
	t.Helper()
	if os.Getenv("GRPC_SERVICE_MESH_GEN_INTEGRATION") == "" {
		t.Skip("set GRPC_SERVICE_MESH_GEN_INTEGRATION=1 to run")
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	args = append([]string{"--definitions", filepath.Join(repoRoot, "examples"), "--go_out=" + filepath.Join(out, "go"), "--ruby_out=" + filepath.Join(out, "ruby")}, args...)
	if code := cli.Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("generator exited %d:\n%s", code, stderr.String())
	}
	return out
}

// generateWithRoots generates examples with both root options set. The
// go_package of examples/shop is a directory package of the root module.
func generateWithRoots(t *testing.T) string {
	t.Helper()
	return generate(t, "--go-root-package", "example.com/definitions;definitions", "--ruby-root-module", "Definitions")
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

// TestGoOutputVets builds the generated shop package as the module its
// go_package names and the servicemaps package as a module beside it.
func TestGoOutputVets(t *testing.T) {
	out := generate(t)
	shop := filepath.Join(out, "go", "shop")
	write(t, filepath.Join(shop, "go.mod"), `module example.com/definitions/shop

go 1.26.6

require (
	github.com/Paymentbox-com/grpc-service-mesh-go v0.8.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
	google.golang.org/protobuf v1.36.12
)
`)
	sh(t, shop, "go", "mod", "tidy")
	sh(t, shop, "go", "vet", "./...")

	maps := filepath.Join(out, "go", "servicemaps")
	write(t, filepath.Join(maps, "go.mod"), `module example.com/servicemaps

go 1.26.6

require (
	example.com/definitions/shop v0.0.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
)

replace example.com/definitions/shop => ../shop
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
raise "no place" unless Shop::OrderClient.respond_to?(:place)
raise "no placed" unless Shop::OrderClient.respond_to?(:placed)
raise "rpcs: #{Shop::OrderService.rpcs.keys}" unless Shop::OrderService.rpcs.keys.sort == [:place, :placed]
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
// package at its root, the shop directory package and servicemaps under it.
func TestGoRootPackageVets(t *testing.T) {
	out := generateWithRoots(t)
	goOut := filepath.Join(out, "go")
	write(t, filepath.Join(goOut, "go.mod"), `module example.com/definitions

go 1.26.6

require (
	github.com/Paymentbox-com/grpc-service-mesh-go v0.8.0
	github.com/Paymentbox-com/service-mesh-go v0.1.0
	google.golang.org/protobuf v1.36.12
)
`)
	write(t, filepath.Join(goOut, "use_test.go"), `package definitions

import "testing"

func TestAliases(t *testing.T) {
	if OrderTargets.Place.Segments[2] != "Place" {
		t.Fatal(OrderTargets.Place)
	}
	var _ OrderService
	var _ *Order
	_ = OrderClient.Place
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
	script := `require "definitions_grpcmesh"
raise "no place" unless Definitions::OrderClient.respond_to?(:place)
raise "Order" unless Definitions::Order.equal?(Shop::Order)
raise "targets" unless Definitions::OrderTargets::PLACE.segments == ["shop", "OrderService", "Place"]
raise "service maps" unless Definitions::ServiceMaps::NATS.targets.size == 2
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
