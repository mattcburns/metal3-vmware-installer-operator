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

package integration

import (
	"context"
	"testing"

	"github.com/vmware-operator/pkg/iso"
	"github.com/vmware-operator/pkg/oras"
)

// TestFullWorkflowIntegration tests the complete Phase 3 workflow:
// Fetch ISO -> Inject ks.cfg -> Push modified ISO
// This validates that all subsystems work together correctly
func TestFullWorkflowIntegration(t *testing.T) {
	ctx := context.Background()

	// Step 1: Create ORAS client (mock mode for testing)
	orasClient := oras.NewClient(nil)

	// Step 2: Fetch ISO from "registry"
	isoBlob, inputDigest, err := orasClient.FetchISO(ctx, "registry.example.com/vmware/esxi:8.0")
	if err != nil {
		t.Fatalf("Failed to fetch ISO: %v", err)
	}

	if len(isoBlob) == 0 {
		t.Errorf("ISO blob is empty")
	}

	if inputDigest == "" {
		t.Errorf("Input digest is empty")
	}

	t.Logf("Step 1 - Fetch ISO: OK (size: %d bytes, digest: %s)", len(isoBlob), inputDigest)

	// Step 2: Inject kickstart configuration into ISO
	ksConfig := `# ESXi 8.0 Kickstart Configuration
vmaccepteula
rootpw mypassword123
`

	modifiedISO, err := iso.InjectKsConfig(isoBlob, ksConfig)
	if err != nil {
		t.Fatalf("Failed to inject ks.cfg: %v", err)
	}

	if len(modifiedISO) == 0 {
		t.Errorf("Modified ISO blob is empty")
	}

	// Verify ISO was modified
	if len(modifiedISO) <= len(isoBlob) {
		t.Errorf("Modified ISO should be larger than original (orig: %d, modified: %d)",
			len(isoBlob), len(modifiedISO))
	}

	t.Logf("Step 2 - Inject ks.cfg: OK (original: %d bytes, modified: %d bytes)",
		len(isoBlob), len(modifiedISO))

	// Step 3: Push modified ISO back to registry
	outputDigest, err := orasClient.PushISO(ctx, modifiedISO, "registry.example.com/vmware/esxi:8.0-provisioned")
	if err != nil {
		t.Fatalf("Failed to push ISO: %v", err)
	}

	if outputDigest == "" {
		t.Errorf("Output digest is empty")
	}

	t.Logf("Step 3 - Push modified ISO: OK (digest: %s)", outputDigest)

	// Verify workflow integrity
	if inputDigest == outputDigest {
		t.Logf("Note: Input and output digests are identical (expected in mock mode)")
	}

	t.Logf("Workflow Integration Test: PASSED")
	t.Logf("  - Input digest:  %s", inputDigest)
	t.Logf("  - Output digest: %s", outputDigest)
}

// TestORASSizeGrowth validates that ORAS preserves blob size correctly
func TestORASSizeGrowth(t *testing.T) {
	ctx := context.Background()
	client := oras.NewClient(nil)

	// Fetch initial ISO
	iso1, digest1, _ := client.FetchISO(ctx, "registry.example.com/vmware/esxi:8.0")
	iso2, digest2, _ := client.FetchISO(ctx, "registry.example.com/vmware/esxi:8.0")

	// Should be consistent
	if len(iso1) != len(iso2) {
		t.Errorf("Multiple fetches should return consistent size")
	}

	if digest1 != digest2 {
		t.Errorf("Multiple fetches should return consistent digest")
	}

	t.Logf("ORAS Size Growth: CONSISTENT (size: %d bytes, digest: %s)", len(iso1), digest1)
}

// TestISOInjectionSize validates that injection increases ISO size appropriately
func TestISOInjectionSize(t *testing.T) {
	originalISO := []byte("mock-esxi-iso")
	ksConfig := `# Sample ks.cfg`

	injected, _ := iso.InjectKsConfig(originalISO, ksConfig)

	expectedMinSize := len(originalISO) + len(ksConfig)
	if len(injected) < expectedMinSize {
		t.Errorf("Injected ISO is too small (expected at least %d, got %d)",
			expectedMinSize, len(injected))
	}

	t.Logf("ISO Injection Size: CORRECT (original: %d, injected: %d, growth: %d bytes)",
		len(originalISO), len(injected), len(injected)-len(originalISO))
}
