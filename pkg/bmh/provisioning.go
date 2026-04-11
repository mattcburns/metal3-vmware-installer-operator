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

package bmh

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProvisioningClient handles updates to Bare Metal Host objects for provisioning
type ProvisioningClient struct {
	c client.Client
}

// ImageRef represents the image reference in a BMH object
// This mirrors the Metal3 BMH API structure
type ImageRef struct {
	URL          string `json:"url"`
	Checksum     string `json:"checksum,omitempty"`
	ChecksumType string `json:"checksumType,omitempty"`
	DiskFormat   string `json:"format,omitempty"`
}

// NewProvisioningClient creates a new provisioning client
func NewProvisioningClient(c client.Client) *ProvisioningClient {
	return &ProvisioningClient{c: c}
}

// UpdateBMHProvisioning updates a Bare Metal Host to provision with the specified ISO
// It sets spec.image.url and spec.image.diskFormat to trigger live-ISO provisioning
//
// Note: This is a placeholder implementation. In production, this would:
// 1. Fetch the actual Metal3 BareMetalHost CRD object
// 2. Update spec.image.url with the ISO URL
// 3. Set spec.image.diskFormat to "live-iso"
// 4. Apply the update to the cluster
func (pc *ProvisioningClient) UpdateBMHProvisioning(ctx context.Context, bmhNamespace, bmhName string, isoURL string) error {
	if bmhNamespace == "" {
		return fmt.Errorf("bmh namespace cannot be empty")
	}
	if bmhName == "" {
		return fmt.Errorf("bmh name cannot be empty")
	}
	if isoURL == "" {
		return fmt.Errorf("iso URL cannot be empty")
	}

	// Placeholder implementation
	// In production, this would:
	// 1. Use dynamic client or Metal3 API types to fetch the BMH
	// 2. Update the spec.image fields
	// 3. Patch the object in the cluster

	fmt.Printf("Placeholder: Updating BMH %s/%s with ISO URL: %s\n", bmhNamespace, bmhName, isoURL)
	return nil
}

// GetBMHStatus retrieves the current status of a Bare Metal Host
func (pc *ProvisioningClient) GetBMHStatus(ctx context.Context, bmhNamespace, bmhName string) (string, error) {
	if bmhNamespace == "" {
		return "", fmt.Errorf("bmh namespace cannot be empty")
	}
	if bmhName == "" {
		return "", fmt.Errorf("bmh name cannot be empty")
	}

	// Placeholder implementation
	fmt.Printf("Placeholder: Getting BMH status for %s/%s\n", bmhNamespace, bmhName)
	return "unknown", nil
}
