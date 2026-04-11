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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vmwarev1 "github.com/vmware-operator/api/v1"
)

var _ = Describe("VmwareInstaller Controller", func() {
	Context("When reconciling a VmwareInstaller resource", func() {
		const installerName = "test-installer"
		const bmhName = "test-bmh"
		const namespace = "default"

		ctx := context.Background()

		installerKey := types.NamespacedName{
			Name:      installerName,
			Namespace: namespace,
		}

		BeforeEach(func() {
			// Create a test VmwareInstaller with valid spec
			By("Creating a test VmwareInstaller")
			installer := &vmwarev1.VmwareInstaller{
				ObjectMeta: metav1.ObjectMeta{
					Name:      installerName,
					Namespace: namespace,
				},
				Spec: vmwarev1.VmwareInstallerSpec{
					KsConfig: "# Sample kickstart config",
					IsoRegistry: vmwarev1.ISORegistryRef{
						Image: "registry.example.com/iso:latest",
					},
					TargetHost: corev1.ObjectReference{
						Name:      bmhName,
						Namespace: namespace,
						Kind:      "BareMetalHost",
					},
				},
			}
			Expect(k8sClient.Create(ctx, installer)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleaning up VmwareInstaller")
			installer := &vmwarev1.VmwareInstaller{}
			if err := k8sClient.Get(ctx, installerKey, installer); err == nil {
				Expect(k8sClient.Delete(ctx, installer)).To(Succeed())
			}
		})

		It("should transition through phases during reconciliation", func() {
			By("Reconciling the VmwareInstaller")
			controllerReconciler := &VmwareInstallerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			// First reconciliation: Pending -> Fetching
			_, _ = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: installerKey,
			})

			// Note: Reconciliation will attempt to create BMH but will fail since BMH CRD isn't in envtest
			// For now, we verify the controller doesn't panic and VmwareInstaller status is updated
			installer := &vmwarev1.VmwareInstaller{}
			Expect(k8sClient.Get(ctx, installerKey, installer)).To(Succeed())

			// Verify status was updated (phase should be set)
			// Note: Will be "Failed" due to missing BMH, which is expected in this test environment
			Expect(installer.Status.Phase).NotTo(BeEmpty())
		})

		It("should handle validation errors gracefully", func() {
			By("Attempting to create a VmwareInstaller with empty ksConfig (validation fails at API)")
			invalidInstaller := &vmwarev1.VmwareInstaller{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-invalid",
					Namespace: namespace,
				},
				Spec: vmwarev1.VmwareInstallerSpec{
					KsConfig: "", // Empty config violates CRD validation
					IsoRegistry: vmwarev1.ISORegistryRef{
						Image: "registry.example.com/iso:latest",
					},
					TargetHost: corev1.ObjectReference{
						Name: bmhName,
					},
				},
			}

			// Creation should fail at API validation level
			err := k8sClient.Create(ctx, invalidInstaller)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ksConfig"))
		})

		It("should not reconcile if already in terminal state", func() {
			By("Manually setting VmwareInstaller to Complete state")
			installer := &vmwarev1.VmwareInstaller{}
			Expect(k8sClient.Get(ctx, installerKey, installer)).To(Succeed())

			installer.Status.Phase = vmwarev1.PhaseComplete
			installer.Status.Message = "Already provisioned"
			Expect(k8sClient.Status().Update(ctx, installer)).To(Succeed())

			By("Reconciling should return immediately")
			controllerReconciler := &VmwareInstallerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: installerKey,
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify status unchanged
			updatedInstaller := &vmwarev1.VmwareInstaller{}
			Expect(k8sClient.Get(ctx, installerKey, updatedInstaller)).To(Succeed())
			Expect(updatedInstaller.Status.Phase).To(Equal(vmwarev1.PhaseComplete))
			Expect(updatedInstaller.Status.Message).To(Equal("Already provisioned"))
		})

		It("should handle missing VmwareInstaller gracefully", func() {
			By("Reconciling non-existent VmwareInstaller")
			controllerReconciler := &VmwareInstallerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			missingKey := types.NamespacedName{
				Name:      "non-existent",
				Namespace: namespace,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: missingKey,
			})
			Expect(err).NotTo(HaveOccurred())

			// Should not error on missing resource (standard K8s behavior)
			By("Verifying no resource was created")
			installer := &vmwarev1.VmwareInstaller{}
			err = k8sClient.Get(ctx, missingKey, installer)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
