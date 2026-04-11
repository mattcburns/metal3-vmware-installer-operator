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

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
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

	// Fetch the BareMetalHost object
	bmh := &bmov1alpha1.BareMetalHost{}
	err := pc.c.Get(ctx, types.NamespacedName{Name: bmhName, Namespace: bmhNamespace}, bmh)
	if err != nil {
		return fmt.Errorf("failed to fetch BareMetalHost %s/%s: %w", bmhNamespace, bmhName, err)
	}

	// Save a copy of the original for the merge patch
	original := bmh.DeepCopy()

	// Update the image reference for live-ISO provisioning
	if bmh.Spec.Image == nil {
		bmh.Spec.Image = &bmov1alpha1.Image{}
	}

	diskFormat := "live-iso"
	bmh.Spec.Image.URL = isoURL
	bmh.Spec.Image.DiskFormat = &diskFormat

	// Patch the BareMetalHost with the new image reference
	err = pc.c.Patch(ctx, bmh, client.MergeFrom(original))
	if err != nil {
		return fmt.Errorf("failed to update BareMetalHost %s/%s: %w", bmhNamespace, bmhName, err)
	}

	fmt.Printf("Successfully updated BMH %s/%s with ISO: %s (diskFormat: live-iso)\n",
		bmhNamespace, bmhName, isoURL)

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

	// Fetch the BareMetalHost object
	bmh := &bmov1alpha1.BareMetalHost{}
	err := pc.c.Get(ctx, types.NamespacedName{Name: bmhName, Namespace: bmhNamespace}, bmh)
	if err != nil {
		return "", fmt.Errorf("failed to fetch BareMetalHost %s/%s: %w", bmhNamespace, bmhName, err)
	}

	// Return the current provisioning state
	return string(bmh.Status.Provisioning.State), nil
}
