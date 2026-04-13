# Real ORAS Registry Testing Guide

## Overview

The vmware-operator now includes a complete implementation of real ORAS v2 registry operations. This guide shows how to test fetching and pushing ISOs to actual OCI registries like Docker Hub, quay.io, or private registries.

## Prerequisites

1. **Registry account** (Docker Hub, quay.io, or private registry)
2. **Kubernetes cluster** with the vmware-operator deployed
3. **Docker image** with ISO content pushed to the registry

## Example: Testing with Docker Hub

### Step 1: Create Docker Secret with Credentials

```bash
kubectl create secret docker-registry docker-creds \
  --docker-server=docker.io \
  --docker-username=<your-username> \
  --docker-password=<your-token> \
  --docker-email=<your-email> \
  -n vmware-operator-system
```

Or convert existing docker config:

```bash
kubectl create secret generic docker-creds \
  --from-file=.docker/config.json=$HOME/.docker/config.json \
  -n vmware-operator-system
```

### Step 2: Create a VMwareInstaller CR

The operator will now use real registry operations when a valid secret is provided:

```yaml
apiVersion: metal3.io/v1
kind: VMwareInstaller
metadata:
  name: esxi-provisioner
  namespace: default
spec:
  # ISO image reference (will be fetched from real registry)
  isoImageRef: docker.io/myorg/esxi:8.0
  
  # Kickstart configuration
  kickstartConfig: |
    vmaccepteula
    rootpw MySecurePassword123
    bootloader --location=mbr --boot-drive=sda
    
  # BareMetalHost to provision
  baremetalHostRef:
    name: bmc-host-1
    namespace: default
```

### Step 3: Operator Flow

When the VMMwareInstaller is created:

1. **Fetch ISO**: `FetchISO()`
   - Resolves image reference in registry
   - Downloads blob to local memory
   - Calculates SHA256 digest

2. **Inject Config**: `InjectKsConfig()`
   - Appends ks.cfg to ISO blob
   - Returns modified ISO

3. **Push Modified ISO**: `PushISO()`
   - Creates manifest with modified ISO
   - Pushes to registry (e.g., `docker.io/myorg/esxi:8.0-provisioned`)
   - Returns digest

4. **Update BareMetalHost**: `UpdateBMHProvisioning()`
   - Sets BMH provisioning spec with ISO URL
   - Triggers Ironic provisioning

## Testing Modes

### Mock Mode (Default for Unit Tests)

Unit tests run in mock mode to avoid external dependencies:

```go
// Unit tests automatically use mock mode
client := oras.NewMockClient()
iso, digest, _ := client.FetchISO(ctx, "any-ref")
// Returns deterministic test data
```

### Real Mode (Production)

Production uses real registry operations:

```go
// Production code
client := oras.NewClient(kubeSecret)
iso, digest, err := client.FetchISO(ctx, "docker.io/myorg/esxi:8.0")
// Connects to real registry
```

## Supported Registries

- **Docker Hub**: `docker.io/username/repo:tag`
- **quay.io**: `quay.io/username/repo:tag`
- **Private Registries**: `registry.example.com/namespace/repo:tag`
- **Any OCI-compliant Registry**: Supported via ORAS v2

## Authentication Methods

The operator supports multiple credential formats in Kubernetes Secrets:

### 1. Docker Config JSON (Recommended)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-creds
type: Opaque
data:
  .docker/config.json: <base64-encoded-config>
```

### 2. Old Docker Config Format

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-creds
type: Opaque
data:
  .dockercfg: <base64-encoded-config>
```

### 3. Direct Username/Password

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: docker-creds
type: Opaque
data:
  username: <base64-encoded-username>
  password: <base64-encoded-password>
```

## Troubleshooting

### Connection Errors

```
failed to resolve image reference: invalid reference
```

**Solution**: Verify image reference format is correct (includes registry, repo, tag)

### Authentication Errors

```
failed to resolve image reference: unauthorized: authentication required
```

**Solutions**:
1. Verify credentials in Secret are correct
2. Ensure Secret exists in the correct namespace
3. Check if registry requires specific endpoint (e.g., `registry.docker.io` vs `docker.io`)

### Blob Not Found

```
failed to copy artifact from registry: not found
```

**Solutions**:
1. Verify image exists in registry
2. Check if image is publicly accessible or if credentials are valid
3. Try with a known public image first (e.g., alpine, nginx)

## Testing Against Docker Hub

### 1. Push a Test Image

```bash
# Tag an image
docker tag my-iso:latest mattcburns/test-iso:latest

# Push to Docker Hub
docker push mattcburns/test-iso:latest
```

### 2. Create Secret

```bash
kubectl create secret docker-registry docker-hub \
  --docker-server=docker.io \
  --docker-username=mattcburns \
  --docker-password=<your-token>
```

### 3. Update VMwareInstaller CR

```yaml
spec:
  isoImageRef: docker.io/mattcburns/test-iso:latest
  # ... rest of config
```

### 4. Watch Reconciliation

```bash
kubectl logs -n vmware-operator-system deployment/vmware-operator-controller-manager -f

# Look for:
# "Fetching VMware ISO: docker.io/mattcburns/test-iso:latest"
# "Successfully fetched ISO (XXXX bytes, digest: sha256:...)"
# "Successfully pushed ISO (XXXX bytes, digest: sha256:...)"
```

## Performance Considerations

- **First fetch**: Full image is downloaded and cached locally
- **Subsequent fetches**: New requests are made each time (consider implementing cache)
- **Push**: Each modified ISO is pushed as a new blob
- **Network**: Large ISOs (hundreds of MB) may take time over slow connections

## Future Enhancements

- [ ] Cache frequently-used ISOs locally
- [ ] Implement exponential backoff for registry retries
- [ ] Support for OCI artifact signing
- [ ] Registry rate limit handling
- [ ] Metrics for registry operations (push/fetch times, sizes)

## References

- [ORAS Specification](https://github.com/oras-project/specs)
- [OCI Image Spec](https://github.com/opencontainers/image-spec)
- [Docker Registry HTTP API](https://docs.docker.com/registry/spec/api/)
