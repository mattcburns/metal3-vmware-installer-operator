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
// Phase 1 MVP: Returns mock data for testing. Production implementation would use ORAS v2 library.
func (c *Client) FetchISO(ctx context.Context, imageRef string) ([]byte, string, error) {
	if imageRef == "" {
		return nil, "", fmt.Errorf("imageRef is empty")
	}

	// Extract credentials if available
	cred, err := c.extractCredentials()
	if err != nil {
		fmt.Printf("Note: No credentials found in secret\n")
	}

	fmt.Printf("Fetching VMware ISO: %s\n", imageRef)
	if cred != nil {
		fmt.Printf("Using registry credentials (username: %s)\n", cred.Username)
	}

	// Phase 1 MVP: Return mock ISO blob and digest
	// This allows testing the workflow without real registry connectivity
	mockISO := []byte("mock-esxi-iso-content-for-testing")
	mockDigest := sha256.Sum256(mockISO)
	digestStr := fmt.Sprintf("sha256:%x", mockDigest)

	return mockISO, digestStr, nil
}

// PushISO pushes a modified ISO image to an OCI registry and returns the digest
// Phase 1 MVP: Returns mock data for testing. Production implementation would use ORAS v2 library.
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
		fmt.Printf("Note: No credentials found in secret\n")
	}

	fmt.Printf("Pushing modified VMware ISO: %s (size: %d bytes)\n", imageRef, len(isoData))
	if cred != nil {
		fmt.Printf("Using registry credentials (username: %s)\n", cred.Username)
	}

	// Phase 1 MVP: Return digest of the pushed blob
	// This allows testing the workflow without real registry connectivity
	digest := sha256.Sum256(isoData)
	digestStr := fmt.Sprintf("sha256:%x", digest)

	return digestStr, nil
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
