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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase represents the current phase of the VmwareInstaller workflow
// +kubebuilder:validation:Enum=Pending;Fetching;Processing;Uploading;Provisioning;Complete;Failed
type Phase string

const (
	PhasePending      Phase = "Pending"
	PhaseFetching     Phase = "Fetching"
	PhaseProcessing   Phase = "Processing"
	PhaseUploading    Phase = "Uploading"
	PhaseProvisioning Phase = "Provisioning"
	PhaseComplete     Phase = "Complete"
	PhaseFailed       Phase = "Failed"
)

// ISORegistryRef describes how to access the ISO image in an OCI registry
type ISORegistryRef struct {
	// image is the OCI image reference (e.g., "registry.example.com/vmware-iso:latest")
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// authSecret is a reference to a Secret containing OCI registry credentials
	// The Secret should contain .dockercfg or .docker/config.json
	// +optional
	AuthSecret *corev1.LocalObjectReference `json:"authSecret,omitempty"`
}

// VmwareInstallerSpec defines the desired state of VmwareInstaller
type VmwareInstallerSpec struct {
	// ksConfig is the kickstart configuration content to be injected into the ISO.
	// Can be plain text or base64-encoded.
	// +kubebuilder:validation:MinLength=1
	KsConfig string `json:"ksConfig"`

	// isoRegistry specifies how to access the VMware ISO image in an OCI registry
	// +kubebuilder:validation:Required
	IsoRegistry ISORegistryRef `json:"isoRegistry"`

	// targetHost is a reference to the Bare Metal Host (BMH) object that should be provisioned
	// +kubebuilder:validation:Required
	TargetHost corev1.ObjectReference `json:"targetHost"`

	// outputImageTag is the OCI image reference where the modified ISO will be pushed.
	// If not specified, defaults to appending "-provisioned" to the input image tag.
	// +optional
	OutputImageTag *string `json:"outputImageTag,omitempty"`
}

// VmwareInstallerStatus defines the observed state of VmwareInstaller.
type VmwareInstallerStatus struct {
	// phase represents the current phase of the provisioning workflow
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// message is a human-readable string describing the current state
	// +optional
	Message string `json:"message,omitempty"`

	// isoDigest is the digest of the prepared ISO image (e.g., sha256:abc123...)
	// for auditability and verification
	// +optional
	IsoDigest string `json:"isoDigest,omitempty"`

	// conditions represent the current state of the VmwareInstaller resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Condition types include:
	// - "IsoPrepared": the ISO has been successfully fetched and modified
	// - "BMHUpdated": the Bare Metal Host has been updated with the provisioning ISO
	// - "Ready": the provisioning workflow is complete
	// - "Failed": an error occurred that prevents further progress
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VmwareInstaller is the Schema for the vmwareinstallers API
type VmwareInstaller struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of VmwareInstaller
	// +required
	Spec VmwareInstallerSpec `json:"spec"`

	// status defines the observed state of VmwareInstaller
	// +optional
	Status VmwareInstallerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VmwareInstallerList contains a list of VmwareInstaller
type VmwareInstallerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []VmwareInstaller `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VmwareInstaller{}, &VmwareInstallerList{})
}
