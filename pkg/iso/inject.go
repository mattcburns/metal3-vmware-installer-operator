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

// InjectKsConfig injects a kickstart configuration file into an ISO image
// It returns the modified ISO image data and any error encountered
//
// Note: This is a placeholder implementation. In production, you would use
// diskfs or iso9660 library to properly insert the file into the ISO filesystem.
// For now, we append the ks.cfg content to the ISO to demonstrate the workflow.
func InjectKsConfig(isoBlob []byte, ksConfig string) ([]byte, error) {
	if len(isoBlob) == 0 {
		return nil, fmt.Errorf("iso blob is empty")
	}

	if ksConfig == "" {
		return nil, fmt.Errorf("ksConfig is empty")
	}

	// Convert kickstart config to bytes
	ksData := []byte(ksConfig)

	// In a real implementation, we would:
	// 1. Open the ISO using diskfs
	// 2. Create a new file "ks.cfg" in the ISO filesystem
	// 3. Write the ks.cfg content
	// 4. Repackage the ISO
	//
	// For now, we simply append the ks.cfg content to the blob
	// This preserves the ISO structure while adding the configuration
	result := make([]byte, 0, len(isoBlob)+len(ksData)+10)
	result = append(result, isoBlob...)
	result = append(result, []byte("\n# ks.cfg content follows:\n")...)
	result = append(result, ksData...)

	return result, nil
}
