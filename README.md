# VMware Provisioner Operator

A Kubernetes controller that automates provisioning of Bare Metal Hosts (BMH) using custom VMware ISO images injected with kickstart configurations. This operator fetches VMware ISO images from an OCI registry, injects kickstart configuration files, and triggers Metal3 live-ISO provisioning.

## Overview

The VMware Provisioner Operator provides a one-shot workflow for provisioning bare metal hosts with custom-configured VMware ISOs:

1. **Fetch ISO**: Downloads a VMware ISO image from an OCI registry using ORAS v2
2. **Inject Configuration**: Embeds a kickstart (ks.cfg) configuration into the ISO using pure Go (diskfs)
3. **Upload Modified ISO**: Pushes the customized ISO back to the registry with a digest-pinned reference
4. **Trigger Provisioning**: Updates the target Bare Metal Host (BMH) object to trigger Metal3 live-ISO provisioning via Ironic

## Architecture

### Components

- **Controller**: Core reconciliation logic that orchestrates the provisioning workflow
- **ORAS Package** (`pkg/oras`): Handles OCI registry operations (fetch and push ISO images) using `oras.land/oras-go/v2`
- **ISO Package** (`pkg/iso`): Injects kickstart configuration into ISO images using `github.com/diskfs/go-diskfs`
- **BMH Package** (`pkg/bmh`): Updates Bare Metal Host objects using the Metal3 API to trigger provisioning

### Workflow Phases

The VmwareInstaller progresses through the following phases:

| Phase | Description |
|---|---|
| **Pending** | Initial state, not yet reconciled |
| **Fetching** | Downloading ISO from the OCI registry |
| **Processing** | Injecting kickstart configuration into ISO |
| **Uploading** | Pushing the modified ISO back to the registry |
| **Provisioning** | Updating the BMH to trigger Metal3 provisioning |
| **Complete** | Provisioning workflow finished successfully |
| **Failed** | An unrecoverable error occurred |

### One-Shot Model

Each `VmwareInstaller` CR triggers provisioning exactly once. Once it reaches `Complete` or `Failed`, reconciliation stops. To re-provision a host, create a new `VmwareInstaller` CR. This prevents accidental re-provisioning and keeps the resource immutable after processing begins.

## Prerequisites

- Kubernetes cluster (v1.23+) with [Metal3](https://metal3.io) deployed
- Go 1.25+
- `kubectl` configured to access your cluster
- An OCI-compliant registry (Docker Hub, Quay, Harbor, private registry, etc.) with the VMware ISO pushed as an OCI artifact

## Building

Generate code and manifests:

```bash
make generate   # Regenerate DeepCopy methods
make manifests  # Regenerate CRD and RBAC manifests
make build      # Compile the controller binary
```

Run tests:

```bash
make test
```

## Deploying to a Real Cluster

### Step 1: Build and Push the Operator Image

```bash
export IMG=registry.example.com/vmware-operator:latest
make docker-build docker-push IMG=$IMG
```

### Step 2: Install CRDs and Deploy the Controller

```bash
make manifests
make deploy IMG=$IMG
```

This creates the `vmware-operator-system` namespace, installs the CRD, and deploys the controller manager with the correct RBAC.

Alternatively, apply the generated manifests individually:

```bash
kubectl apply -f config/crd/bases/
kubectl apply -f config/rbac/
kubectl apply -f config/manager/
```

### Step 3: Create Registry Credentials Secret

The operator needs credentials to pull the source ISO and push the modified ISO. Create a Secret in the same namespace as the `VmwareInstaller` CR.

**Option A — Docker config JSON (recommended):**

```bash
kubectl create secret generic registry-credentials \
  --from-file=.dockerconfigjson=$HOME/.docker/config.json \
  --type=kubernetes.io/dockerconfigjson \
  -n default
```

**Option B — Explicit username and password:**

```bash
kubectl create secret docker-registry registry-credentials \
  --docker-server=registry.example.com \
  --docker-username=myuser \
  --docker-password=mypassword \
  -n default
```

**Option C — Docker Hub token:**

```bash
kubectl create secret docker-registry docker-hub-creds \
  --docker-server=docker.io \
  --docker-username=myuser \
  --docker-password=<access-token> \
  -n default
```

The operator also supports legacy `.dockercfg` secrets and raw `username`/`password` keys in an `Opaque` Secret. See [Authentication Methods](#authentication-methods) for all supported formats.

### Step 4: Push Your VMware ISO to the Registry

The source ISO must be stored as an OCI artifact. Push it using `oras`:

```bash
oras push registry.example.com/vmware-iso:9.0 \
  --artifact-type application/vnd.vmware.iso \
  VMware-ESXi-9.0.iso:application/octet-stream
```

Or using Docker (wrapping the ISO as a container image layer) if your tooling already does that.

### Step 5: Create a VmwareInstaller CR

```bash
kubectl apply -f config/samples/metal3_v1_vmwareinstaller.yaml
```

Or create a custom CR (see [Example CRs](#example-crs) below).

### Step 6: Monitor Provisioning

```bash
# Watch phase transitions
kubectl get vmwareinstallers -w

# Inspect status and conditions
kubectl describe vmwareinstaller example-installer

# Stream controller logs
kubectl logs -n vmware-operator-system \
  deployment/vmware-operator-controller-manager -f
```

Expected log progression:

```
"Fetching ISO from registry"  image="registry.example.com/vmware-iso:9.0"
"Successfully fetched ISO"    digest="sha256:..." size=...
"Processing ISO with kickstart config"
"Successfully injected ks.cfg" modifiedSize=...
"Uploading modified ISO to registry"
"Successfully pushed modified ISO" digest="sha256:..." tag="..."
"Updating Bare Metal Host for provisioning" bmh="worker-node-01"
"VmwareInstaller workflow completed successfully"
```

## Example CRs

### Minimal Example

```yaml
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: esxi-provisioner
  namespace: default
spec:
  ksConfig: |
    vmaccepteula
    rootpw --iscrypted <hashed-password>
    bootloader --location=mbr --boot-drive=sda
    install --firstdisk --overwritevmfs
    network --bootproto=dhcp --device=vmnic0
    reboot

  isoRegistry:
    image: "registry.example.com/vmware-iso:9.0"
    authSecret:
      name: registry-credentials

  targetHost:
    name: worker-node-01
    namespace: openstack
```

### Specifying a Custom Output Image Tag

By default the modified ISO is pushed to `{source-registry}/{source-repo}:{targetHost.name}-{timestamp}` (e.g., `registry.example.com/vmware-iso:worker-node-01-20260420-153000`). To override this:

```yaml
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: esxi-provisioner-custom
  namespace: default
spec:
  ksConfig: |
    vmaccepteula
    rootpw --iscrypted <hashed-password>
    install --firstdisk --overwritevmfs
    reboot

  isoRegistry:
    image: "registry.example.com/vmware-iso:9.0"
    authSecret:
      name: registry-credentials

  targetHost:
    name: worker-node-01
    namespace: openstack

  outputImageTag: "registry.example.com/provisioned-isos/esxi:worker-node-01-v1"
```

### Using Docker Hub

```yaml
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: esxi-from-dockerhub
  namespace: default
spec:
  ksConfig: |
    vmaccepteula
    rootpw --iscrypted <hashed-password>
    install --firstdisk --overwritevmfs
    reboot

  isoRegistry:
    image: "docker.io/myorg/esxi-iso:8.0"
    authSecret:
      name: docker-hub-creds

  targetHost:
    name: rack1-node3
    namespace: metal3
```

## Authentication Methods

The operator reads credentials from a Kubernetes Secret referenced by `spec.isoRegistry.authSecret`. All standard Docker credential formats are supported:

### Docker Config JSON (kubernetes.io/dockerconfigjson)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded-config.json>
```

### Docker Config JSON (Opaque, legacy path)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
type: Opaque
data:
  .docker/config.json: <base64-encoded-config.json>
```

### Legacy .dockercfg

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
type: Opaque
data:
  .dockercfg: <base64-encoded-dockercfg>
```

### Raw Username / Password

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: registry-credentials
type: Opaque
data:
  username: <base64-encoded-username>
  password: <base64-encoded-password>
```

## Supported Registries

Any OCI-compliant registry is supported via ORAS v2:

| Registry | Image Reference Format |
|---|---|
| Docker Hub | `docker.io/username/repo:tag` |
| Quay.io | `quay.io/username/repo:tag` |
| GitHub Container Registry | `ghcr.io/org/repo:tag` |
| Harbor | `harbor.example.com/project/repo:tag` |
| Private / Self-hosted | `registry.example.com/namespace/repo:tag` |

## API Reference

### VmwareInstaller Spec

| Field | Type | Required | Description |
|---|---|---|---|
| `ksConfig` | string | Yes | Kickstart configuration content (plain text or base64-encoded) |
| `isoRegistry` | ISORegistryRef | Yes | OCI registry reference for the source ISO image |
| `targetHost` | ObjectReference | Yes | Reference to the Bare Metal Host (BMH) to provision |
| `outputImageTag` | string | No | Full OCI image reference for the modified ISO. Defaults to `{source-repo}:{targetHost.name}-{timestamp}` |

### ISORegistryRef

| Field | Type | Required | Description |
|---|---|---|---|
| `image` | string | Yes | OCI image reference (e.g., `registry.example.com/vmware-iso:9.0`) |
| `authSecret` | LocalObjectReference | No | Secret containing OCI registry credentials |

### VmwareInstaller Status

| Field | Type | Description |
|---|---|---|
| `phase` | Phase | Current provisioning phase |
| `message` | string | Human-readable status message |
| `isoDigest` | string | SHA256 digest of the pushed ISO manifest |
| `conditions` | []Condition | Standard Kubernetes conditions (`Ready`, `Failed`, `Progressing`) |

## Integration with Metal3

The operator integrates with Metal3 by updating the target Bare Metal Host (BMH) object:

1. Sets `spec.image.url` to a digest-pinned OCI reference (`registry/repo@sha256:...`) for Ironic
2. Sets `spec.image.diskFormat` to `"live-iso"` to trigger live-boot mode
3. Metal3/Ironic transitions the BMH: `Available → Provisioning → Provisioned`

The digest-pinned reference ensures Ironic fetches the exact modified ISO and not a newer tag.

## Testing

### Unit Tests

Unit tests run without any external dependencies using a built-in mock ORAS client:

```bash
make test
```

Or run directly:

```bash
go test ./... -v
```

### Integration Tests

Integration tests in `test/integration/` exercise the full workflow against a real Kubernetes API (via envtest). Run with:

```bash
make test
```

### Testing Against a Real Registry

To validate registry connectivity before deploying, you can create a minimal `VmwareInstaller` CR pointing at a known public image and watch the logs:

```bash
# Push a small test artifact
oras push docker.io/myorg/test-iso:latest \
  test.iso:application/octet-stream

# Create credentials secret
kubectl create secret docker-registry docker-hub-creds \
  --docker-server=docker.io \
  --docker-username=myorg \
  --docker-password=<access-token> \
  -n default

# Apply a test CR
kubectl apply -f - <<EOF
apiVersion: metal3.io/v1
kind: VmwareInstaller
metadata:
  name: registry-test
  namespace: default
spec:
  ksConfig: "vmaccepteula\nrootpw testpass\nreboot\n"
  isoRegistry:
    image: "docker.io/myorg/test-iso:latest"
    authSecret:
      name: docker-hub-creds
  targetHost:
    name: test-host
    namespace: default
EOF

# Watch logs
kubectl logs -n vmware-operator-system \
  deployment/vmware-operator-controller-manager -f
```

## Troubleshooting

### Invalid image reference

```
failed to resolve image reference: invalid reference
```

Verify the `spec.isoRegistry.image` value uses the full `registry/repo:tag` format, including the registry hostname for non-Docker-Hub registries.

### Authentication required

```
failed to resolve image reference: unauthorized: authentication required
```

- Verify the Secret referenced by `spec.isoRegistry.authSecret` exists in the same namespace as the `VmwareInstaller` CR
- Check that the credentials are correct and the token has not expired
- For Docker Hub, note that the server is `docker.io` but some clients require `registry-1.docker.io` — the operator handles this automatically

### Blob not found

```
failed to copy artifact from registry: not found
```

- Confirm the image exists in the registry and is accessible with the provided credentials
- Verify the tag or digest is correct

### BMH not updating

- Confirm the `targetHost.name` and `targetHost.namespace` match an existing `BareMetalHost` object
- Verify the controller's RBAC allows patching `baremetalhosts` in the target namespace
- Check `kubectl describe vmwareinstaller <name>` for error messages in `.status.message`

## Development

### Project Structure

```
.
├── api/v1/                          # CRD type definitions
│   ├── vmwareinstaller_types.go
│   └── groupversion_info.go
├── internal/controller/             # Controller reconciliation logic
│   ├── vmwareinstaller_controller.go
│   └── vmwareinstaller_controller_test.go
├── pkg/
│   ├── oras/                        # OCI registry operations (ORAS v2)
│   ├── iso/                         # ISO manipulation (diskfs)
│   └── bmh/                         # BMH provisioning (Metal3 API)
├── config/
│   ├── crd/bases/                   # Generated CRD manifests
│   ├── manager/                     # Controller Deployment and kustomization
│   ├── rbac/                        # RBAC roles and bindings
│   └── samples/                     # Example VmwareInstaller CRs
├── test/
│   ├── e2e/                         # End-to-end tests (requires Kind cluster)
│   └── integration/                 # Integration tests (envtest)
├── Makefile
└── go.mod
```

### Adding Features

1. Update CRD types in `api/v1/vmwareinstaller_types.go`
2. Regenerate code: `make generate && make manifests`
3. Implement logic in the controller and/or utility packages
4. Add tests in the corresponding `*_test.go` files
5. Verify: `make test && make lint-fix`

### Build Targets

```bash
make help         # Display all available targets
make manifests    # Generate CRD and RBAC manifests
make generate     # Generate DeepCopy methods
make build        # Build the controller binary
make run          # Run controller locally (uses current kubeconfig)
make docker-build # Build Docker image
make docker-push  # Push Docker image
make test         # Run all tests with coverage
make lint         # Run golangci-lint
make lint-fix     # Auto-fix linting issues
make deploy       # Deploy controller to cluster
make undeploy     # Remove controller from cluster
make install      # Install CRDs only
make uninstall    # Remove CRDs only
make clean        # Clean build artifacts
```

## Known Limitations

- No webhook validation (field errors surface at reconciliation time, not at `kubectl apply`)
- No retry/backoff logic for transient registry or network errors — failures go directly to `Failed` phase
- Large ISOs (hundreds of MB) are held in memory during processing

## Future Improvements

- [ ] Validation webhook for early error detection at admission time
- [ ] Exponential backoff for transient registry and network errors
- [ ] Streaming ISO processing to reduce peak memory usage for large ISOs
- [ ] OCI artifact signing support
- [ ] Registry rate limit handling and metrics (push/fetch latency, sizes)
- [ ] Batch provisioning support across multiple BMHs
- [ ] Tie lifecycle of VmwareInstaller objects to BMHs

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Run `make test && make lint-fix`
5. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See LICENSE file for details.

## Support

- GitHub Issues: Report bugs and request features
- Examples: See `config/samples/` directory
