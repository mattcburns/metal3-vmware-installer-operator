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

package oras

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	corev1 "k8s.io/api/core/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// Client handles OCI registry operations for ISO images
// Phase 3: Supports both mock and real registry operations
type Client struct {
	// authSecret contains registry credentials if needed
	authSecret *corev1.Secret
	// mockMode enables test mode (returns mock data instead of real registry calls)
	mockMode bool
}

// Credential represents registry authentication
type Credential struct {
	Username string
	Password string
}

// NewClient creates a new ORAS client
// mockMode=true for testing (returns mock data), mockMode=false for real registry operations
func NewClient(authSecret *corev1.Secret) *Client {
	return newClientWithMode(authSecret, false)
}

// NewClientWithMode creates a new ORAS client with explicit mode selection
func newClientWithMode(authSecret *corev1.Secret, mockMode bool) *Client {
	return &Client{
		authSecret: authSecret,
		mockMode:   mockMode,
	}
}

// NewMockClient creates a new ORAS client in mock mode for testing
// Use this in unit tests to avoid needing a real registry
func NewMockClient() *Client {
	return newClientWithMode(nil, true)
}

// FetchISO fetches an ISO image from an OCI registry and returns the blob data and digest
// Supports both mock mode (for testing) and real ORAS registry operations
// Real implementation uses oras.land/oras-go/v2 library for OCI registry access
func (c *Client) FetchISO(ctx context.Context, imageRef string) ([]byte, string, error) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return nil, "", fmt.Errorf("imageRef is empty")
	}

	fmt.Printf("Fetching VMware ISO: %s\n", imageRef)

	// In mock mode, return test data (used for unit testing)
	if c.mockMode {
		mockISO := []byte("mock-esxi-iso-content-for-testing")
		mockDigest := sha256.Sum256(mockISO)
		digestStr := fmt.Sprintf("sha256:%x", mockDigest)
		return mockISO, digestStr, nil
	}

	// Real mode: Use ORAS v2 library to fetch from actual OCI registry
	// Create a remote registry repository
	remoteRepo, err := remote.NewRepository(imageRef)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create remote repository: %w", err)
	}

	// Extract and apply credentials if available
	cred, err := c.extractCredentials()
	if err != nil {
		fmt.Printf("Note: No credentials found in secret, attempting unauthenticated access\n")
	} else if cred != nil {
		fmt.Printf("Using registry credentials (username: %s)\n", cred.Username)
		// Create an auth client with credentials — StaticCredential takes the registry hostname only
		remoteRepo.Client = &auth.Client{
			Credential: auth.StaticCredential(remoteRepo.Reference.Registry, auth.Credential{
				Username: cred.Username,
				Password: cred.Password,
			}),
		}
	}

	// Resolve the image reference using the tag/reference parsed from the image ref
	desc, err := remoteRepo.Resolve(ctx, remoteRepo.Reference.Reference)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve image reference: %w", err)
	}

	fmt.Printf("Resolved image reference to digest: %s\n", desc.Digest)

	// Create a local memory store to fetch the content into
	memStore := memory.New()

	// Copy from remote repo to local store
	copyDesc, err := oras.Copy(ctx, remoteRepo, desc.Digest.String(), memStore, desc.Digest.String(), oras.CopyOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to copy artifact from registry: %w", err)
	}

	// Fetch the ISO blob (first layer in the artifact)
	isoData, manifestDigest, err := c.extractISOFromManifest(ctx, memStore, copyDesc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to extract ISO from manifest: %w", err)
	}

	if len(isoData) == 0 {
		return nil, "", fmt.Errorf("extracted ISO blob is empty")
	}

	// Calculate digest of the actual ISO data
	isoDigest := sha256.Sum256(isoData)
	digestStr := fmt.Sprintf("sha256:%x", isoDigest)

	fmt.Printf("Successfully fetched ISO (%d bytes, digest: %s)\n", len(isoData), digestStr)
	_ = manifestDigest // Use manifest digest for logging if needed
	return isoData, digestStr, nil
}

// PushISO pushes a modified ISO image to an OCI registry and returns the digest
// Supports both mock mode (for testing) and real ORAS registry operations
// Real implementation uses oras.land/oras-go/v2 library for OCI registry access
func (c *Client) PushISO(ctx context.Context, isoData []byte, imageRef string) (string, error) {
	imageRef = strings.TrimSpace(imageRef)
	if len(isoData) == 0 {
		return "", fmt.Errorf("isoData is empty")
	}
	if imageRef == "" {
		return "", fmt.Errorf("imageRef is empty")
	}

	fmt.Printf("Pushing modified VMware ISO: %s (size: %d bytes)\n", imageRef, len(isoData))

	// Calculate digest of the ISO data
	isoDigest := sha256.Sum256(isoData)
	digestStr := fmt.Sprintf("sha256:%x", isoDigest)

	// In mock mode, return digest immediately (used for unit testing)
	if c.mockMode {
		return digestStr, nil
	}

	// Real mode: Use ORAS v2 library to push to actual OCI registry
	// Create a local memory store with the ISO blob
	memStore := memory.New()

	// Add the ISO blob to the memory store
	isoDesc := ocispec.Descriptor{
		MediaType: "application/vnd.vmware.iso.image.v1+octet-stream",
		Size:      int64(len(isoData)),
		Digest:    digest.NewDigestFromEncoded(digest.SHA256, fmt.Sprintf("%x", isoDigest)),
	}

	// Push the blob to the memory store
	if err := memStore.Push(ctx, isoDesc, bytes.NewReader(isoData)); err != nil {
		return "", fmt.Errorf("failed to add ISO to memory store: %w", err)
	}

	// Create a remote registry repository
	remoteRepo, err := remote.NewRepository(imageRef)
	if err != nil {
		return "", fmt.Errorf("failed to create remote repository: %w", err)
	}

	// Extract and apply credentials if available
	cred, err := c.extractCredentials()
	if err != nil {
		fmt.Printf("Note: No credentials found in secret, attempting unauthenticated push\n")
	} else if cred != nil {
		fmt.Printf("Using registry credentials (username: %s)\n", cred.Username)
		remoteRepo.Client = &auth.Client{
			Credential: auth.StaticCredential(remoteRepo.Reference.Registry, auth.Credential{
				Username: cred.Username,
				Password: cred.Password,
			}),
		}
	}

	// Tag the manifest in the memory store so Copy can resolve it by tag
	tag := remoteRepo.Reference.Reference
	manifestDesc, err := oras.PackManifest(ctx, memStore, oras.PackManifestVersion1_1, "application/vnd.vmware.iso.image.v1", oras.PackManifestOptions{
		Layers: []ocispec.Descriptor{isoDesc},
	})
	if err != nil {
		return "", fmt.Errorf("failed to pack ISO into manifest: %w", err)
	}
	if err := memStore.Tag(ctx, manifestDesc, tag); err != nil {
		return "", fmt.Errorf("failed to tag manifest in memory store: %w", err)
	}

	// Copy manifest (and its blobs) from memory store to remote
	copiedDesc, err := oras.Copy(ctx, memStore, tag, remoteRepo, tag, oras.CopyOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to push ISO to registry: %w", err)
	}

	// Return the manifest digest so callers can construct a digest-pinned reference
	manifestDigest := copiedDesc.Digest.String()
	fmt.Printf("Successfully pushed ISO (%d bytes, manifest digest: %s)\n", len(isoData), manifestDigest)
	return manifestDigest, nil
}

// extractCredentials extracts Docker credentials from a Kubernetes Secret
func (c *Client) extractCredentials() (*Credential, error) {
	if c.authSecret == nil {
		return nil, fmt.Errorf("no auth secret provided")
	}

	// Try to extract from .dockercfg (old format)
	if dockerCfg, ok := c.authSecret.Data[".dockercfg"]; ok {
		return c.parseDockerCfg(dockerCfg)
	}

	// Try to extract from .docker/config.json (new format)
	if dockerConfig, ok := c.authSecret.Data[".docker/config.json"]; ok {
		return c.parseDockerConfigJson(dockerConfig)
	}

	// Support for kubernetes.io/dockerconfigjson (the default key)
	if dockerConfigJson, ok := c.authSecret.Data[".dockerconfigjson"]; ok {
		return c.parseDockerConfigJson(dockerConfigJson)
	}

	// Try direct username/password fields
	if username, ok := c.authSecret.Data["username"]; ok {
		if password, ok := c.authSecret.Data["password"]; ok {
			return &Credential{
				Username: string(username),
				Password: string(password),
			}, nil
		}
	}

	return nil, fmt.Errorf("no recognized credential format in secret")
}

// parseDockerCfg parses the old .dockercfg format
func (c *Client) parseDockerCfg(data []byte) (*Credential, error) {
	var dockerCfg map[string]interface{}
	if err := json.Unmarshal(data, &dockerCfg); err != nil {
		return nil, fmt.Errorf("failed to parse .dockercfg: %w", err)
	}

	// Get the first entry
	for _, entry := range dockerCfg {
		if authEntry, ok := entry.(map[string]interface{}); ok {
			if auth, ok := authEntry["auth"].(string); ok {
				// Decode base64
				decoded, err := base64.StdEncoding.DecodeString(auth)
				if err != nil {
					continue
				}

				// Split username:password
				parts := bytes.Split(decoded, []byte(":"))
				if len(parts) == 2 {
					return &Credential{
						Username: string(parts[0]),
						Password: string(parts[1]),
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no auth entry found in .dockercfg")
}

// parseDockerConfigJson parses the new .docker/config.json format
func (c *Client) parseDockerConfigJson(data []byte) (*Credential, error) {
	var config struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse docker config.json: %w", err)
	}

	// Get the first entry
	for _, entry := range config.Auths {
		if entry.Auth == "" {
			continue
		}

		// Decode base64
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			continue
		}

		// Split username:password
		parts := bytes.Split(decoded, []byte(":"))
		if len(parts) == 2 {
			return &Credential{
				Username: string(parts[0]),
				Password: string(parts[1]),
			}, nil
		}
	}

	return nil, fmt.Errorf("no auth entries found in docker config.json")
}

// extractISOFromManifest extracts the ISO blob from an OCI manifest
func (c *Client) extractISOFromManifest(ctx context.Context, store *memory.Store, manifestDesc ocispec.Descriptor) ([]byte, string, error) {
	// Read the manifest descriptor
	rc, err := store.Fetch(ctx, manifestDesc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer rc.Close()

	// Parse the manifest to get layers
	manifestData, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read manifest: %w", err)
	}

	// Try to parse as OCI Image Manifest
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		// Fallback: manifest might be a blob itself (single layer), return it
		fmt.Printf("Note: Could not parse as manifest, treating as raw blob\n")
		return manifestData, manifestDesc.Digest.String(), nil
	}

	// If there are layers, fetch the first one (usually the ISO)
	if len(manifest.Layers) > 0 {
		layerDesc := manifest.Layers[0]
		rc, err := store.Fetch(ctx, layerDesc)
		if err != nil {
			return nil, "", fmt.Errorf("failed to fetch layer: %w", err)
		}
		defer rc.Close()

		layerData, err := io.ReadAll(rc)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read layer: %w", err)
		}

		return layerData, layerDesc.Digest.String(), nil
	}

	// Fallback: return the manifest data itself as the ISO
	return manifestData, manifestDesc.Digest.String(), nil
}
