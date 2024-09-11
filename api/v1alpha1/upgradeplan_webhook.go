/*
Copyright 2024.

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

package v1alpha1

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *UpgradePlan) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-lifecycle-suse-com-v1alpha1-upgradeplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=lifecycle.suse.com,resources=upgradeplans,verbs=create;update,versions=v1alpha1,name=vupgradeplan.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &UpgradePlan{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *UpgradePlan) ValidateCreate() (admission.Warnings, error) {
	if r.Spec.ReleaseVersion == "" {
		return nil, fmt.Errorf("release version is required")
	}

	_, err := version.ParseSemantic(r.Spec.ReleaseVersion)
	if err != nil {
		return nil, fmt.Errorf("'%s' is not a semantic version", r.Spec.ReleaseVersion)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *UpgradePlan) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldPlan, ok := old.(*UpgradePlan)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", old)
	}

	// deletion is scheduled, but not yet finished and controller has updated the plan
	// with the removed finalizers
	if !r.ObjectMeta.DeletionTimestamp.IsZero() && len(r.Finalizers) < len(oldPlan.Finalizers) {
		return nil, nil
	}

	if oldPlan.Status.LastSuccessfulReleaseVersion != "" {
		newReleaseVersion, err := version.ParseSemantic(r.Spec.ReleaseVersion)
		if err != nil {
			return nil, fmt.Errorf("'%s' is not a semantic version", r.Spec.ReleaseVersion)
		}

		indicator, err := newReleaseVersion.Compare(oldPlan.Status.LastSuccessfulReleaseVersion)
		if err != nil {
			return nil, fmt.Errorf("comparing versions: %w", err)
		}

		switch indicator {
		case 0:
			return nil, fmt.Errorf("any edits over '%s' must come with an increment of the releaseVersion", r.Name)
		case -1:
			return nil, fmt.Errorf("new releaseVersion '%s' must be greater than the currently applied '%s' releaseVersion", r.Spec.ReleaseVersion, oldPlan.Status.LastSuccessfulReleaseVersion)
		}
	}

	disallowingUpdateStates := []string{UpgradeInProgress, UpgradePending, UpgradeError}

	for _, condition := range r.Status.Conditions {
		if slices.Contains(disallowingUpdateStates, condition.Reason) {
			return nil, fmt.Errorf("upgrade plan cannot be edited while condition '%s' is in '%s' state", condition.Type, condition.Reason)
		}
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *UpgradePlan) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
