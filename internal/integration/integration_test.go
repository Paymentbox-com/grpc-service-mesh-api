// Package integration compiles and loads the generator's output for
// examples/pbx against the published libraries. The tests need network
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
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	pbx := filepath.Join(out, "go", "pbx")
	write(t, filepath.Join(pbx, "go.mod"), `module github.com/Paymentbox-com/pbx

go 1.26.6

require (
	github.com/Paymentbox-com/grpc-service-mesh-api v0.0.0
	github.com/Paymentbox-com/grpc-service-mesh-go v0.1.1
	github.com/Paymentbox-com/service-mesh-go v0.1.0
	google.golang.org/protobuf v1.36.12
)

replace github.com/Paymentbox-com/grpc-service-mesh-api => `+root+`
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

replace github.com/Paymentbox-com/grpc-service-mesh-api => `+root+`
`)
	sh(t, maps, "go", "mod", "tidy")
	sh(t, maps, "go", "vet", "./...")
}

// TestRubyOutputLoads requires the generated files against the published
// gems and reads the map and a client method back.
func TestRubyOutputLoads(t *testing.T) {
	out := generate(t)
	ruby := filepath.Join(out, "ruby")
	write(t, filepath.Join(ruby, "Gemfile"), `source "https://rubygems.org"

gem "grpc_service_mesh", git: "https://github.com/Paymentbox-com/grpc-service-mesh-ruby", tag: "v0.1.0"
gem "service_mesh", git: "https://github.com/Paymentbox-com/service-mesh-ruby", tag: "v0.2.0"
gem "google-protobuf"
gem "googleapis-common-protos-types"
`)
	sh(t, ruby, "bundle", "install", "--quiet")
	script := `require "service_maps"
raise "targets: #{ServiceMaps::NATS.targets.size}" unless ServiceMaps::NATS.targets.size == 2
raise "no search" unless Pbx::ApiKeyClient.respond_to?(:search)
raise "no created" unless Pbx::ApiKeyClient.respond_to?(:created)
raise "rpcs: #{Pbx::ApiKeyService.rpcs.keys}" unless Pbx::ApiKeyService.rpcs.keys.sort == [:created, :search]
puts "loaded"
`
	got := sh(t, ruby, "bundle", "exec", "ruby", "-I"+ruby, "-e", script)
	if strings.TrimSpace(got) != "loaded" {
		t.Fatalf("ruby output:\n%s", got)
	}
}
