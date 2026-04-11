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

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func createSchemeWithBMH() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := bmov1alpha1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	return scheme
}

func TestNewProvisioningClient(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)
	if pc == nil {
		t.Errorf("NewProvisioningClient returned nil")
	}
}

func TestUpdateBMHProvisioningEmptyNamespace(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "", "host", "http://iso-url")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty namespace")
	}
}

func TestUpdateBMHProvisioningEmptyName(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "default", "", "http://iso-url")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty name")
	}
}

func TestUpdateBMHProvisioningEmptyURL(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)

	err := pc.UpdateBMHProvisioning(context.Background(), "default", "host1", "")
	if err == nil {
		t.Errorf("UpdateBMHProvisioning should return error for empty URL")
	}
}

func TestUpdateBMHProvisioningValid(t *testing.T) {
	// Create a test BareMetalHost object
	bmh := &bmov1alpha1.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-host",
			Namespace: "default",
		},
		Spec: bmov1alpha1.BareMetalHostSpec{
			BMC: bmov1alpha1.BMCDetails{
				Address: "redfish://bmc.example.com",
			},
		},
	}

	// Create a fake client with the BMH object and the correct scheme
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(bmh).Build()
	pc := NewProvisioningClient(c)

	// Test update
	err := pc.UpdateBMHProvisioning(context.Background(), "default", "test-host", "http://iso-url")
	if err != nil {
		t.Errorf("UpdateBMHProvisioning returned error: %v", err)
	}

	// Verify the update
	updated := &bmov1alpha1.BareMetalHost{}
	err = c.Get(context.Background(), client.ObjectKey{Name: "test-host", Namespace: "default"}, updated)
	if err != nil {
		t.Errorf("Failed to get updated BMH: %v", err)
	}

	if updated.Spec.Image == nil || updated.Spec.Image.URL != "http://iso-url" {
		t.Errorf("BMH image URL not updated correctly: got %v", updated.Spec.Image)
	}
}

func TestGetBMHStatusEmptyNamespace(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)

	_, err := pc.GetBMHStatus(context.Background(), "", "host")
	if err == nil {
		t.Errorf("GetBMHStatus should return error for empty namespace")
	}
}

func TestGetBMHStatusEmptyName(t *testing.T) {
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	pc := NewProvisioningClient(c)

	_, err := pc.GetBMHStatus(context.Background(), "default", "")
	if err == nil {
		t.Errorf("GetBMHStatus should return error for empty name")
	}
}

func TestGetBMHStatusValid(t *testing.T) {
	// Create a test BareMetalHost object with status
	bmh := &bmov1alpha1.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-host",
			Namespace: "default",
		},
		Spec: bmov1alpha1.BareMetalHostSpec{
			BMC: bmov1alpha1.BMCDetails{
				Address: "redfish://bmc.example.com",
			},
		},
		Status: bmov1alpha1.BareMetalHostStatus{
			Provisioning: bmov1alpha1.ProvisionStatus{
				State: "provisioning",
			},
		},
	}

	// Create a fake client with the BMH object and the correct scheme
	scheme := createSchemeWithBMH()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(bmh).Build()
	pc := NewProvisioningClient(c)

	status, err := pc.GetBMHStatus(context.Background(), "default", "test-host")
	if err != nil {
		t.Errorf("GetBMHStatus returned error: %v", err)
	}

	if status != "provisioning" {
		t.Errorf("GetBMHStatus returned wrong status: got %s, expected provisioning", status)
	}
}
