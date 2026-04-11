//go:build ignore
// +build ignore

package main

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	ControllerGenVersion = "v0.20.1"
	LocalBinDir          = "bin"
	ControllerGenPath    = LocalBinDir + "/controller-gen"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "generate":
		generate()
	case "manifests":
		manifests()
	case "build":
		buildController()
	case "docker-build":
		dockerBuild()
	case "test":
		runTests()
	case "clean":
		clean()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: go run build.go <command>

Commands:
  generate       - Generate DeepCopy and other code from markers
  manifests      - Generate CRD and RBAC manifests
  build          - Build the controller binary
  docker-build   - Build the Docker image
  test           - Run tests
  clean          - Clean build artifacts

Examples:
  go run build.go generate
  go run build.go manifests
  go run build.go build
  go run build.go test`)
}

// ensureControllerGen downloads controller-gen if not present
func ensureControllerGen() error {
	if _, err := os.Stat(ControllerGenPath); err == nil {
		return nil // Already exists
	}

	fmt.Printf("Downloading controller-gen@%s...\n", ControllerGenVersion)
	cmd := exec.Command("go", "install",
		fmt.Sprintf("sigs.k8s.io/controller-tools/cmd/controller-gen@%s", ControllerGenVersion))
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install controller-gen: %w", err)
	}

	// Copy from GOBIN to local bin directory
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		// Try GOPATH/bin
		gopath := os.Getenv("GOPATH")
		if gopath != "" {
			gobin = filepath.Join(gopath, "bin")
		} else {
			// Try default location
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("could not determine GOBIN: %w", err)
			}
			gobin = filepath.Join(home, "go", "bin")
		}
	}

	src := filepath.Join(gobin, "controller-gen")
	dst := ControllerGenPath

	// Ensure local bin directory exists
	if err := os.MkdirAll(LocalBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", LocalBinDir, err)
	}

	// Copy controller-gen binary
	srcData, err := ioutil.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read controller-gen from %s: %w", src, err)
	}
	if err := ioutil.WriteFile(dst, srcData, 0755); err != nil {
		return fmt.Errorf("failed to write controller-gen to %s: %w", dst, err)
	}

	fmt.Printf("Downloaded controller-gen to %s\n", dst)
	return nil
}

// generate runs code generation
func generate() {
	fmt.Println("Generating code...")

	if err := ensureControllerGen(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(ControllerGenPath, "object:headerFile=hack/boilerplate.go.txt", "paths=./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Code generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Code generation complete")
}

// manifests generates CRD and RBAC manifests
func manifests() {
	fmt.Println("Generating manifests...")

	if err := ensureControllerGen(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(ControllerGenPath,
		"rbac:roleName=manager-role",
		"crd",
		"webhook",
		"paths=./...",
		"output:crd:artifacts:config=config/crd/bases")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Manifest generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Manifests generated to config/crd/bases/")
}

// buildController builds the manager binary
func buildController() {
	fmt.Println("Building controller...")

	// First generate code
	generate()

	cmd := exec.Command("go", "build", "-o", "bin/manager", "./cmd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Build complete: bin/manager")
}

// dockerBuild builds the Docker image
func dockerBuild() {
	fmt.Println("Building Docker image...")

	// First build the binary
	buildController()

	// Determine image name
	img := os.Getenv("IMG")
	if img == "" {
		img = "controller:latest"
	}

	cmd := exec.Command("docker", "build", "-t", img, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Docker build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Docker image built: %s\n", img)
}

// runTests runs unit tests
func runTests() {
	fmt.Println("Running tests...")

	// First generate code
	generate()

	cmd := exec.Command("go", "test", "./...", "-v", "-cover")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Tests failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Tests complete")
}

// clean removes build artifacts
func clean() {
	fmt.Println("Cleaning build artifacts...")

	paths := []string{"bin", "config/crd/bases"}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not remove %s: %v\n", path, err)
		} else {
			fmt.Printf("Removed %s\n", path)
		}
	}

	fmt.Println("Clean complete")
}
