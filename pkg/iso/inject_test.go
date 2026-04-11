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
	"testing"
)

func TestInjectKsConfig(t *testing.T) {
	mockISO := []byte("mock-iso-content")
	ksConfig := "test kickstart config"

	result, err := InjectKsConfig(mockISO, ksConfig)
	if err != nil {
		t.Errorf("InjectKsConfig returned error: %v", err)
	}

	if len(result) <= len(mockISO) {
		t.Errorf("InjectKsConfig did not increase size: original=%d, result=%d",
			len(mockISO), len(result))
	}

	// Verify the ks.cfg content is included
	if !contains(result, []byte(ksConfig)) {
		t.Errorf("InjectKsConfig did not include ks.cfg content")
	}
}

func TestInjectKsConfigEmptyISO(t *testing.T) {
	_, err := InjectKsConfig([]byte{}, "config")
	if err == nil {
		t.Errorf("InjectKsConfig should return error for empty ISO")
	}
}

func TestInjectKsConfigEmptyConfig(t *testing.T) {
	_, err := InjectKsConfig([]byte("iso-data"), "")
	if err == nil {
		t.Errorf("InjectKsConfig should return error for empty config")
	}
}

// helper function to check if slice contains subslice
func contains(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
