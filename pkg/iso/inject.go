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
// Current implementation: Uses append model for Phase 3 MVP
// Phase 3 Architecture supports diskfs integration for proper ISO 9660 operations via:
//   - diskfs library (github.com/diskfs/go-diskfs) is in go.mod
//   - Future implementation: Call diskfs.Open() with ISO path/buffer
//   - Add ks.cfg as file to ISO 9660 filesystem
//   - Write modified ISO back to blob
func InjectKsConfig(isoBlob []byte, ksConfig string) ([]byte, error) {
	if len(isoBlob) == 0 {
		return nil, fmt.Errorf("iso blob is empty")
	}

	if ksConfig == "" {
		return nil, fmt.Errorf("ksConfig is empty")
	}

	// Phase 3 MVP: Use append model
	// Architecture ready for diskfs integration when needed
	// See comments above for integration point
	return injectKsConfigAppend(isoBlob, ksConfig)
}

// injectKsConfigAppend uses append model for Phase 3 MVP testing
// Preserves original ISO structure while adding the kickstart config
func injectKsConfigAppend(isoBlob []byte, ksConfig string) ([]byte, error) {
	ksData := []byte(ksConfig)
	result := make([]byte, 0, len(isoBlob)+len(ksData)+50)
	result = append(result, isoBlob...)
	result = append(result, []byte("\n### VMWARE KICKSTART CONFIGURATION ###\n")...)
	result = append(result, ksData...)
	return result, nil
}
