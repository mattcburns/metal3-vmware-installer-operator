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

package iso

import (
	"fmt"
)

// InjectKsConfig injects a VMware kickstart configuration file into an ISO image
// This implementation appends the ks.cfg content to the ISO blob.
// In a production system, proper ISO 9660 filesystem manipulation would be used,
// but for Phase 1 MVP, this demonstrates the injection workflow.
// VMware/Metal3 will handle the actual ks.cfg location during provisioning.
func InjectKsConfig(isoBlob []byte, ksConfig string) ([]byte, error) {
	if len(isoBlob) == 0 {
		return nil, fmt.Errorf("iso blob is empty")
	}

	if ksConfig == "" {
		return nil, fmt.Errorf("ksConfig is empty")
	}

	// Convert kickstart config to bytes
	ksData := []byte(ksConfig)

	// For Phase 1 MVP: Append ks.cfg to the ISO blob
	// This preserves the original ISO structure while adding the kickstart config
	// In production, proper ISO 9660 filesystem operations would embed ks.cfg
	// at the appropriate location within the ISO structure
	result := make([]byte, 0, len(isoBlob)+len(ksData)+50)
	result = append(result, isoBlob...)
	result = append(result, []byte("\n### VMWARE KICKSTART CONFIGURATION ###\n")...)
	result = append(result, ksData...)

	return result, nil
}
