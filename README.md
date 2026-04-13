# VMware Provisioner Operator

A Kubernetes controller that automates provisioning of Bare Metal Hosts (BMH) using custom VMware ISO images injected with kickstart configurations. This operator fetches VMware ISO images from an OCI registry, injects kickstart configuration files, and triggers Metal3 live-ISO provisioning.

## Overview

The VMware Provisioner Operator provides a one-shot workflow for provisioning bare metal hosts with custom-configured VMware ISOs:

1. **Fetch ISO**: Downloads a VMware ISO image from an OCI registry using ORAS
2. **Inject Configuration**: Embeds a kickstart (ks.cfg) configuration into the ISO
3. **Upload Modified ISO**: Pushes the customized ISO back to the registry
4. **Trigger Provisioning**: Updates the target Bare Metal Host (BMH) object to trigger Metal3 provisioning with the live-ISO

## Architecture

### Components

- **Controller**: Core reconciliation logic that orchestrates the provisioning workflow
- **ORAS Package**: Handles OCI registry operations (fetch and push ISO images)
- **ISO Package**: Injects kickstart configuration into ISO images
- **BMH Package**: Updates Bare Metal Host objects to trigger provisioning

### Workflow Phases

The VmwareInstaller progresses through the following phases:

- **Pending**: Initial state
- **Fetching**: Downloading ISO from registry
- **Processing**: Injecting kickstart configuration
- **Uploading**: Pushing modified ISO to registry
- **Provisioning**: Updating BMH to trigger Metal3 provisioning
- **Complete**: Provisioning workflow finished successfully
- **Failed**: An error occurred

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.23+) with Metal3 deployed
- Go 1.25+
- kubectl configured to access your cluster
- OCI registry (Docker Registry, Quay, Harbor, etc.) with VMware ISO images

### Building

Generate code and compile:

```bash
make generate
make build
```

Generate CRD manifests:

```bash
make manifests
```

Run tests:

```bash
make test
```

### Deployment

1. **Generate and apply CRD**:

```bash
make manifests
kubectl apply -f config/crd/bases/
```

2. **Create namespace and deploy controller**:

```bash
kubectl create namespace vmware-operator-system
# Deploy controller manager (see config/manager/ for RBAC and deployment manifests)
```

3. **Create registry credentials Secret** (if needed):

```bash
kubectl create secret docker-registry registry-credentials \
  --docker-server=registry.example.com \
  --docker-username=myuser \
  --docker-password=mypassword \
  -n default
```

4. **Apply a VmwareInstaller CR**:

```bash
kubectl apply -f config/samples/metal3_v1_vmwareinstaller.yaml
```

5. **Monitor provisioning progress**:

```bash
kubectl get vmwareinstallers -w
kubectl describe vmwareinstaller example-installer
kubectl logs -f -l app=vmware-operator -n vmware-operator-system
```

## Example Usage

### Simple Provisioning

```yaml
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: example-installer
  namespace: default
spec:
  ksConfig: |
    install
    cdrom
    lang en_US.UTF-8
    keyboard us
    zerombr
    clearpart --all --initlabel
    autopart
    bootloader --location=mbr
    %packages
    @core
    %end

  isoRegistry:
    image: "registry.example.com/vmware-iso:9.0"
    authSecret:
      name: registry-credentials

  targetHost:
    name: worker-01
    namespace: openstack
```

### With Custom Output Image

```yaml
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: example-installer-custom
  namespace: default
spec:
  ksConfig: |
    # Your kickstart configuration here
    install
    # ... rest of ks.cfg ...

  isoRegistry:
    image: "registry.example.com/vmware-iso:9.0-latest"

  targetHost:
    name: custom-worker
    namespace: openstack

  # Explicitly specify where to push the modified ISO
  outputImageTag: "registry.internal/iso-repo/vmware-provisioned:9.0-v1"
```

## API Reference

### VmwareInstaller Spec

- **ksConfig** (string, required): Kickstart configuration content (plain text or base64-encoded)
- **isoRegistry** (ISORegistryRef, required): OCI registry reference for the ISO image
- **targetHost** (ObjectReference, required): Reference to the Bare Metal Host to provision
- **outputImageTag** (string, optional): Registry path for the modified ISO (defaults to `{input-image}-provisioned`)

### ISORegistryRef

- **image** (string, required): OCI image reference (e.g., `registry.example.com/iso:latest`)
- **authSecret** (LocalObjectReference, optional): Secret containing registry credentials

### VmwareInstaller Status

- **phase** (Phase): Current provisioning phase (Pending, Fetching, Processing, Uploading, Provisioning, Complete, Failed)
- **message** (string): Human-readable status message
- **isoDigest** (string): Digest of the prepared ISO for verification
- **conditions** ([]Condition): Detailed status conditions (IsoPrepared, BMHUpdated, Ready, Failed)

##Testing

### Unit Tests

Run all tests with code generation and linting:

```bash
make test
```

Or run tests directly:

```bash
go test ./... -v
```

### Example Test Output

```
=== RUN   TestInjectKsConfig
--- PASS: TestInjectKsConfig (0.00s)
=== RUN   TestFetchISOValid
--- PASS: TestFetchISOValid (0.00s)
=== RUN   TestPushISO
--- PASS: TestPushISO (0.00s)
PASS
ok      github.com/vmware-operator/pkg/oras     0.015s
```

## Integration with Metal3

The operator integrates with Metal3 by updating Bare Metal Host (BMH) objects:

1. Sets `spec.image.url` to the provisioned ISO URL
2. Sets `spec.image.diskFormat` to `"live-iso"` to trigger live-boot
3. Metal3 automatically transitions the BMH through states:
   - Available → Provisioning → Provisioned

The BMH handles the actual interaction with Ironic (Metal3's provisioning engine) to boot and configure the host.

## Architecture Decisions

### One-Shot Model

VmwareInstaller uses a one-shot execution model:
- Each CR triggers provisioning once
- Upon reaching Complete or Failed phase, reconciliation stops
- To re-provision, create a new VmwareInstaller CR

This prevents accidental re-provisioning and keeps the resource immutable once processing starts.

### Pure Go ISO Handling

The operator uses diskfs (pure Go library) for ISO manipulation, avoiding dependency on host tools like `xorriso` or `mkisofs`. This ensures portability across container environments.

### Flexible Registry Authentication

Registry credentials can be provided via:
- Explicit Secret reference in the CR (`spec.isoRegistry.authSecret`)
- Cluster-wide pull secrets (future enhancement)
- Fallback to cluster ServiceAccount credentials

## Development

### Project Structure

```
.
├── api/
│   └── v1/                          # CRD type definitions
│       ├── vmwareinstaller_types.go
│       └── groupversion_info.go
├── internal/
│   └── controller/                  # Controller reconciliation logic
│       ├── vmwareinstaller_controller.go
│       └── vmwareinstaller_controller_test.go
├── pkg/
│   ├── oras/                        # OCI registry operations
│   │   ├── oras.go
│   │   └── oras_test.go
│   ├── iso/                         # ISO file manipulation
│   │   ├── inject.go
│   │   └── inject_test.go
│   └── bmh/                         # BMH provisioning logic
│       ├── provisioning.go
│       └── provisioning_test.go
├── config/
│   ├── crd/bases/                   # Generated CRD manifests
│   ├── manager/                     # Controller deployment
│   ├── rbac/                        # RBAC roles and bindings
│   └── samples/                     # Example CR manifests
├── Makefile                          # Build automation targets
├── go.mod                            # Go module definition
└── README.md                         # This file
```

### Adding Features

To add new features:

1. Update CRD types in `api/v1/vmwareinstaller_types.go`
2. Regenerate code: `make generate`
3. Implement logic in controller and/or utility packages
4. Add tests in corresponding `*_test.go` files
5. Generate manifests: `make manifests`

### Build System

The project uses Kubebuilder's Makefile for build automation:

```bash
make help                 # Display all available targets
make manifests            # Generate CRD and RBAC manifests
make generate             # Generate code from markers
make build                # Build the controller binary
make run                  # Run controller locally
make docker-build         # Build Docker image (runs tests first)
make docker-push          # Push Docker image to registry
make test                 # Run all tests with coverage
make lint                 # Run golangci-lint
make lint-fix             # Auto-fix linting issues
make deploy               # Deploy controller to cluster
make undeploy             # Remove controller from cluster
make install              # Install CRDs to cluster
make uninstall            # Remove CRDs from cluster
make clean                # Clean build artifacts
```

## Known Limitations

- ORAS integration is currently placeholder; production version needs full OCI registry implementation
- ISO injection is simplified; production version should use proper ISO 9660 library integration
- BMH updating uses placeholder; production version needs Metal3 API types
- No webhook validation (can be added as future enhancement)
- No advanced error recovery/retry logic yet

## Future Improvements

- [ ] Full ORAS library integration for OCI registry operations
- [ ] Proper ISO 9660 filesystem integration using diskfs
- [ ] Metal3 API types for type-safe BMH updates
- [ ] Validation webhooks for early error detection
- [ ] Metrics and observability (Prometheus)
- [ ] Advanced retry logic with exponential backoff
- [ ] Batch provisioning support
- [ ] Custom kernel parameters injection
- [ ] Network configuration injection

## Contributing

To contribute:

1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Run tests and linting: `make test && make lint-fix`
5. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See LICENSE file for details.

## Support

For issues and discussions:
- GitHub Issues: Report bugs and request features
- Documentation: Check this README and code comments
- Examples: See `config/samples/` directory
