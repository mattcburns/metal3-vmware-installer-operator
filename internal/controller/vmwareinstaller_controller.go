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
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	vmwarev1 "github.com/vmware-operator/api/v1"
	"github.com/vmware-operator/pkg/bmh"
	"github.com/vmware-operator/pkg/iso"
	"github.com/vmware-operator/pkg/oras"
)

// VmwareInstallerReconciler reconciles a VmwareInstaller object
type VmwareInstallerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=metal3.io,resources=vmwareinstallers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=vmwareinstallers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal3.io,resources=vmwareinstallers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;patch;update

// Reconcile implements the main reconciliation loop for VmwareInstaller
func (r *VmwareInstallerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the VmwareInstaller object
	installer := &vmwarev1.VmwareInstaller{}
	if err := r.Get(ctx, req.NamespacedName, installer); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("VmwareInstaller not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get VmwareInstaller")
		return ctrl.Result{}, err
	}

	// If already complete or failed, don't reconcile again (one-shot model)
	if installer.Status.Phase == vmwarev1.PhaseComplete || installer.Status.Phase == vmwarev1.PhaseFailed {
		log.Info("Installer already in terminal state", "phase", installer.Status.Phase)
		return ctrl.Result{}, nil
	}

	// Validate inputs
	if installer.Spec.KsConfig == "" {
		log.Error(fmt.Errorf("ksConfig is empty"), "Invalid VmwareInstaller")
		r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed, "ksConfig must not be empty", "")
		return ctrl.Result{}, nil
	}

	log.Info("Fetching ISO from registry", "image", installer.Spec.IsoRegistry.Image)

	// Get auth secret if specified
	var authSecret *corev1.Secret
	if installer.Spec.IsoRegistry.AuthSecret != nil {
		authSecret = &corev1.Secret{}
		secretKey := client.ObjectKey{
			Namespace: installer.Namespace,
			Name:      installer.Spec.IsoRegistry.AuthSecret.Name,
		}
		if err := r.Get(ctx, secretKey, authSecret); err != nil {
			log.Error(err, "Failed to get auth secret")
			r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed,
				fmt.Sprintf("Failed to get auth secret: %v", err), "")
			return ctrl.Result{}, nil
		}
	}

	// Fetch the ISO from registry
	orasClient := oras.NewClient(authSecret)
	isoBlob, inputDigest, err := orasClient.FetchISO(ctx, installer.Spec.IsoRegistry.Image)
	if err != nil {
		log.Error(err, "Failed to fetch ISO")
		r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed,
			fmt.Sprintf("Failed to fetch ISO: %v", err), "")
		return ctrl.Result{}, nil
	}

	log.Info("Successfully fetched ISO", "digest", inputDigest, "size", len(isoBlob))

	log.Info("Processing ISO with kickstart config")

	// Inject ks.cfg into ISO
	modifiedISO, err := iso.InjectKsConfig(isoBlob, installer.Spec.KsConfig)
	if err != nil {
		log.Error(err, "Failed to inject ks.cfg into ISO")
		r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed,
			fmt.Sprintf("Failed to inject ks.cfg: %v", err), "")
		return ctrl.Result{}, nil
	}

	log.Info("Successfully injected ks.cfg", "modifiedSize", len(modifiedISO))

	log.Info("Uploading modified ISO to registry")

	// Determine output image tag
	outputTag := installer.Spec.OutputImageTag
	if outputTag == nil || *outputTag == "" {
		// Strip existing tag from source image to get bare registry/repository
		baseImage := installer.Spec.IsoRegistry.Image
		if i := strings.LastIndex(baseImage, ":"); i >= 0 {
			baseImage = baseImage[:i]
		}
		hostName := installer.Spec.TargetHost.Name
		timestamp := time.Now().UTC().Format("20060102-150405")
		derived := baseImage + ":" + hostName + "-" + timestamp
		outputTag = &derived
	}

	// Push the modified ISO
	outputDigest, err := orasClient.PushISO(ctx, modifiedISO, *outputTag)
	if err != nil {
		log.Error(err, "Failed to push modified ISO")
		r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed,
			fmt.Sprintf("Failed to push ISO: %v", err), "")
		return ctrl.Result{}, nil
	}

	log.Info("Successfully pushed modified ISO", "digest", outputDigest, "tag", *outputTag)

	// Build a digest-pinned OCI reference for Ironic (tag alone is not specific enough)
	// Strip tag from outputTag to get the bare repo, then append the manifest digest
	baseRepo := *outputTag
	if i := strings.LastIndex(baseRepo, ":"); i >= 0 {
		baseRepo = baseRepo[:i]
	}
	digestRef := baseRepo + "@" + outputDigest

	log.Info("Updating Bare Metal Host for provisioning", "bmh", installer.Spec.TargetHost.Name)

	// Update the BMH to trigger provisioning
	bmhClient := bmh.NewProvisioningClient(r.Client)
	targetNamespace := installer.Spec.TargetHost.Namespace
	if targetNamespace == "" {
		targetNamespace = installer.Namespace
	}

	err = bmhClient.UpdateBMHProvisioning(ctx, targetNamespace,
		installer.Spec.TargetHost.Name, digestRef)
	if err != nil {
		log.Error(err, "Failed to update BMH")
		r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseFailed,
			fmt.Sprintf("Failed to update BMH: %v", err), outputDigest)
		return ctrl.Result{}, nil
	}

	// Transition to Complete phase
	log.Info("VmwareInstaller workflow completed successfully")
	r.updateInstallerStatus(ctx, installer, vmwarev1.PhaseComplete,
		"Provisioning workflow completed successfully", outputDigest)

	return ctrl.Result{}, nil
}

// updateInstallerStatus updates the installer status and conditions
func (r *VmwareInstallerReconciler) updateInstallerStatus(ctx context.Context, installer *vmwarev1.VmwareInstaller,
	phase vmwarev1.Phase, message, digest string) {
	log := logf.FromContext(ctx)

	conditionType := "Progressing"
	conditionStatus := metav1.ConditionTrue
	reason := "Progressing"

	switch phase {
	case vmwarev1.PhaseComplete:
		conditionType = "Ready"
		reason = "Provisioned"
	case vmwarev1.PhaseFailed:
		conditionType = "Failed"
		reason = "Failed"
		conditionStatus = metav1.ConditionTrue
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &vmwarev1.VmwareInstaller{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(installer), latest); err != nil {
			return err
		}
		latest.Status.Phase = phase
		latest.Status.Message = message
		if digest != "" {
			latest.Status.IsoDigest = digest
		}
		condition := metav1.Condition{
			Type:               conditionType,
			Status:             conditionStatus,
			ObservedGeneration: latest.Generation,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
		}
		found := false
		for i, c := range latest.Status.Conditions {
			if c.Type == conditionType {
				latest.Status.Conditions[i] = condition
				found = true
				break
			}
		}
		if !found {
			latest.Status.Conditions = append(latest.Status.Conditions, condition)
		}
		return r.Status().Update(ctx, latest)
	}); err != nil {
		log.Error(err, "Failed to update installer status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VmwareInstallerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vmwarev1.VmwareInstaller{}).
		Named("vmwareinstaller").
		Complete(r)
}
