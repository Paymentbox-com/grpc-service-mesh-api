// Package specdir finds the directory of this module that holds
// mesh/options.proto, the directory the generator puts on every protoc run's
// proto path and grpc-service-mesh-gen proto-path prints.
package specdir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// Module is the path of the module that holds the specification's protos.
const Module = "github.com/Paymentbox-com/grpc-service-mesh-api"

// optionsProto is the file the directory must hold.
const optionsProto = "mesh/options.proto"

// Resolve returns the specification directory for the running binary's
// module version.
func Resolve() (string, error) {
	version := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
	}
	return ForVersion(version)
}

// ForVersion returns the module directory of version from the module cache,
// downloading it when needed. A development build, whose version is empty,
// "(devel)", or ends in "+dirty", takes the checkout the working directory
// is in.
func ForVersion(version string) (string, error) {
	if version == "" || version == "(devel)" || strings.HasSuffix(version, "+dirty") {
		return Checkout()
	}
	return Download(version)
}

// Download returns the directory of Module@version in the module cache, as
// go mod download reports it.
func Download(version string) (string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", Module+"@"+version)
	cmd.Dir = os.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	var mod struct{ Dir, Error string }
	_ = json.Unmarshal(stdout.Bytes(), &mod)
	switch {
	case mod.Error != "":
		return "", fmt.Errorf("go mod download %s@%s: %s", Module, version, mod.Error)
	case runErr != nil:
		return "", fmt.Errorf("go mod download %s@%s: %w\n%s", Module, version, runErr, strings.TrimSpace(stderr.String()))
	}
	return checked(mod.Dir)
}

// Checkout returns the root of the working directory's module when that
// module is Module, as go list -m reports it.
func Checkout() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}\t{{.Dir}}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("a development build takes the specification directory from a checkout of %s in the working directory; go list -m: %s", Module, msg)
	}
	var found []string
	for line := range strings.Lines(stdout.String()) {
		path, dir, _ := strings.Cut(strings.TrimSpace(line), "\t")
		if dir == "" {
			continue
		}
		if path == Module {
			return checked(dir)
		}
		found = append(found, path)
	}
	where := "in no Go module"
	if len(found) > 0 {
		where = "in " + strings.Join(found, ", ")
	}
	return "", fmt.Errorf("a development build takes the specification directory from a checkout of %s in the working directory; the working directory is %s", Module, where)
}

// checked returns dir when it holds mesh/options.proto.
func checked(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(optionsProto))); err != nil {
		return "", err
	}
	return dir, nil
}
