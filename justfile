# Task runner for grpc-service-mesh-api. Run `just` with no arguments to see the menu.
#
# Every Go recipe runs through `mise exec` so it uses the toolchain pinned in
# mise.toml without relying on the shell's mise activation. If you ever run just
# from a bare environment where `mise` is not on PATH, change this to
# "/opt/homebrew/bin/mise exec -- go".
go := "mise exec -- go"

# List all recipes
default:
    @just --list

# Compile everything
[group('build')]
build:
    {{go}} build ./...

# Run the unit tests with the race detector (needs protoc and protoc-gen-go on PATH)
[group('build')]
test:
    {{go}} test -race ./...

# Compile and load the generated examples/pbx code against the published libraries (needs network, go, bundle)
[group('build')]
test-integration:
    GRPC_SERVICE_MESH_GEN_INTEGRATION=1 mise exec -- go test -race -count=1 ./internal/integration/...

# Install the generator into GOBIN
[group('build')]
install:
    {{go}} install ./cmd/grpc-service-mesh-gen

# Regenerate the golden files under internal/gen/testdata/golden from examples/pbx
[group('build')]
gen-example:
    UPDATE_GOLDEN=1 mise exec -- go test -count=1 ./internal/gen -run TestGolden

# Regenerate mesh/options.pb.go, the compiled Go form of mesh/options.proto
[group('build')]
proto:
    mise exec -- protoc --proto_path=. --go_out=. --go_opt=paths=source_relative --go_opt=Mmesh/options.proto=github.com/Paymentbox-com/grpc-service-mesh-api/mesh mesh/options.proto

# Run go vet
[group('checks')]
vet:
    {{go}} vet ./...

# Format the code in place
[group('checks')]
fmt:
    gofmt -w .

# Fail if any file is not gofmt-formatted
[private]
[group('checks')]
fmt-check:
    test -z "$(gofmt -l .)"

# Reconcile go.mod and go.sum
[group('checks')]
tidy:
    {{go}} mod tidy

# Report reachable vulnerabilities (matches CI)
[group('checks')]
vuln:
    {{go}} run golang.org/x/vuln/cmd/govulncheck@latest ./...

# There is no .golangci.yml, so lint runs golangci-lint's default linters. CI
# installs golangci-lint through its GitHub action rather than with `go run`,
# so the two can differ by a release.

# Report lint findings (matches CI)
[group('checks')]
lint:
    {{go}} run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./...

# Everything the pre-push gate checks, in the order CI runs them
[group('checks')]
check: fmt-check vet test test-integration vuln lint
