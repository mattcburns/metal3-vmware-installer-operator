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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewProvisioningClient(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)
	if pc == nil {
		t.Errorf("NewProvisioningClient returned nil")
	}
}

func TestUpdateBMHProvisioningEmptyNamespace(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "", "host", "http://iso-url")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty namespace")
	}
}

func TestUpdateBMHProvisioningEmptyName(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "default", "", "http://iso-url")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty name")
	}
}

func TestUpdateBMHProvisioningEmptyURL(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "default", "host1", "")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty URL")
	}
}

func TestUpdateBMHProvisioningValid(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "default", "host1", "http://iso-url")
	if err != nil {
		t.Errorf("UpdateBMHProvisioning returned error: %v", err)
	}
}

func TestGetBMHStatusEmptyNamespace(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	_, err := pc.GetBMHStatus(context.Background(), "", "host")
	if err == nil {
		t.Errorf("GetBMHStatus should return error for empty namespace")
	}
}

func TestGetBMHStatusEmptyName(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	_, err := pc.GetBMHStatus(context.Background(), "default", "")
	if err == nil {
		t.Errorf("GetBMHStatus should return error for empty name")
	}
}

func TestGetBMHStatusValid(t *testing.T) {
	c := fake.NewFakeClient()
	pc := NewProvisioningClient(c)

	status, err := pc.GetBMHStatus(context.Background(), "default", "host1")
	if err != nil {
		t.Errorf("GetBMHStatus returned error: %v", err)
	}
	if status == "" {
		t.Errorf("GetBMHStatus returned empty status")
	}
}
