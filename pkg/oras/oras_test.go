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
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Errorf("NewClient returned nil")
	}
}

func TestFetchISOEmpty(t *testing.T) {
	client := NewClient(nil)
	_, _, err := client.FetchISO(context.Background(), "")
	if err == nil {
		t.Errorf("FetchISO should return error for empty imageRef")
	}
}

func TestFetchISOValid(t *testing.T) {
	client := NewClient(nil)
	data, digest, err := client.FetchISO(context.Background(), "registry.example.com/iso:latest")
	if err != nil {
		t.Errorf("FetchISO returned error: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("FetchISO returned empty data")
	}
	if digest == "" {
		t.Errorf("FetchISO returned empty digest")
	}
}

func TestPushISO(t *testing.T) {
	client := NewClient(nil)
	testData := []byte("test-iso-data")
	digest, err := client.PushISO(context.Background(), testData, "registry.example.com/iso:latest")
	if err != nil {
		t.Errorf("PushISO returned error: %v", err)
	}
	if digest == "" {
		t.Errorf("PushISO returned empty digest")
	}
}

func TestPushISOEmpty(t *testing.T) {
	client := NewClient(nil)
	_, err := client.PushISO(context.Background(), []byte{}, "")
	if err == nil {
		t.Errorf("PushISO should return error for empty imageRef")
	}
}
