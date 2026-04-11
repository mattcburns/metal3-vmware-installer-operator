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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// Client handles OCI registry operations for ISO images
type Client struct {
	// authSecret contains registry credentials if needed
	authSecret *corev1.Secret
}

// Credential represents registry authentication
type Credential struct {
	Username string
	Password string
}

// NewClient creates a new ORAS client
func NewClient(authSecret *corev1.Secret) *Client {
	return &Client{
		authSecret: authSecret,
	}
}

// FetchISO fetches an ISO image from an OCI registry and returns the blob data and digest
// This is a placeholder implementation. In production, this would use ORAS library
// to properly handle OCI image push/pull with correct manifests.
func (c *Client) FetchISO(ctx context.Context, imageRef string) ([]byte, string, error) {
	if imageRef == "" {
		return nil, "", fmt.Errorf("imageRef is empty")
	}

	// Extract credentials if available
	cred, err := c.extractCredentials()
	if err != nil {
		// Log warning but don't fail - may be using cluster pull secrets
		fmt.Printf("Warning: could not extract credentials: %v\n", err)
	}
	_ = cred // Use credential in production implementation

	// Placeholder implementation: return a mock ISO blob and digest
	// In production, this would:
	// 1. Authenticate to the registry using the credentials
	// 2. Fetch the image manifest
	// 3. Download the blob layers
	// 4. Combine into the ISO data
	// 5. Calculate and return the digest

	mockISO := []byte("mock-iso-placeholder-content")
	mockDigest := "sha256:abc123def456" // Placeholder digest

	fmt.Printf("Placeholder: Fetching ISO %s (credentials: %v)\n", imageRef, cred != nil)
	return mockISO, mockDigest, nil
}

// PushISO pushes a modified ISO image to an OCI registry and returns the digest
// This is a placeholder implementation. In production, this would use ORAS library.
func (c *Client) PushISO(ctx context.Context, isoData []byte, imageRef string) (string, error) {
	if len(isoData) == 0 {
		return "", fmt.Errorf("isoData is empty")
	}
	if imageRef == "" {
		return "", fmt.Errorf("imageRef is empty")
	}

	// Extract credentials if available
	cred, err := c.extractCredentials()
	if err != nil {
		fmt.Printf("Warning: could not extract credentials: %v\n", err)
	}
	_ = cred

	// Placeholder implementation: return a mock digest
	// In production, this would:
	// 1. Create an OCI image config and manifest
	// 2. Upload layers to the registry
	// 3. Upload the manifest
	// 4. Return the digest of the uploaded image

	// Simple digest calculation for demonstration
	digestHex := "0000000000000000000000000000000000000000"       // 40 hex chars for sha256 prefix
	mockDigest := "sha256:" + digestHex[:min(16, len(digestHex))] // Truncated placeholder

	fmt.Printf("Placeholder: Pushing ISO to %s (size: %d bytes, credentials: %v)\n",
		imageRef, len(isoData), cred != nil)
	return mockDigest, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
					return nil, fmt.Errorf("failed to decode auth: %w", err)
				}

				// Parse username:password
				parts := string(decoded)
				for i, c := range parts {
					if c == ':' {
						return &Credential{
							Username: parts[:i],
							Password: parts[i+1:],
						}, nil
					}
				}
			}
		}
		break
	}

	return nil, fmt.Errorf("no auth entry found in .dockercfg")
}

// parseDockerConfigJson parses the new .docker/config.json format
func (c *Client) parseDockerConfigJson(data []byte) (*Credential, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse docker config: %w", err)
	}

	auths, ok := config["auths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no auths section in docker config")
	}

	// Get the first entry
	for _, entry := range auths {
		if authEntry, ok := entry.(map[string]interface{}); ok {
			if auth, ok := authEntry["auth"].(string); ok {
				// Decode base64
				decoded, err := base64.StdEncoding.DecodeString(auth)
				if err != nil {
					return nil, fmt.Errorf("failed to decode auth: %w", err)
				}

				// Parse username:password
				parts := string(decoded)
				for i, c := range parts {
					if c == ':' {
						return &Credential{
							Username: parts[:i],
							Password: parts[i+1:],
						}, nil
					}
				}
			}
		}
		break
	}

	return nil, fmt.Errorf("no auth entry found in docker config")
}
