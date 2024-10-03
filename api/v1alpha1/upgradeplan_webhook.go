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
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-lifecycle-suse-com-v1alpha1-upgradeplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=lifecycle.suse.com,resources=upgradeplans,verbs=create;update,versions=v1alpha1,name=vupgradeplan.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &UpgradePlanValidator{}

type UpgradePlanValidator struct{}

func (*UpgradePlanValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	upgradePlan, ok := obj.(*UpgradePlan)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", obj)
	}

	_, err := validateReleaseVersion(upgradePlan.Spec.ReleaseVersion)
	return nil, err
}

func (*UpgradePlanValidator) ValidateUpdate(ctx context.Context, old, new runtime.Object) (admission.Warnings, error) {
	oldPlan, ok := old.(*UpgradePlan)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", old)
	}

	newPlan, ok := new.(*UpgradePlan)
	if !ok {
		return nil, fmt.Errorf("unexpected object type: %T", new)
	}

	// deletion is scheduled, but not yet finished and controller has updated the plan
	// with the removed finalizers
	if !newPlan.ObjectMeta.DeletionTimestamp.IsZero() && len(newPlan.Finalizers) < len(oldPlan.Finalizers) {
		return nil, nil
	}

	disallowingUpdateStates := []string{UpgradeInProgress, UpgradePending, UpgradeError}

	for _, condition := range newPlan.Status.Conditions {
		if slices.Contains(disallowingUpdateStates, condition.Reason) {
			return nil, fmt.Errorf("upgrade plan cannot be edited while condition '%s' is in '%s' state", condition.Type, condition.Reason)
		}
	}

	newReleaseVersion, err := validateReleaseVersion(newPlan.Spec.ReleaseVersion)
	if err != nil {
		return nil, err
	}

	if oldPlan.Status.LastSuccessfulReleaseVersion != "" {
		indicator, err := newReleaseVersion.Compare(oldPlan.Status.LastSuccessfulReleaseVersion)
		if err != nil {
			return nil, fmt.Errorf("comparing versions: %w", err)
		}

		switch indicator {
		case 0:
			return nil, fmt.Errorf("any edits over '%s' must come with an increment of the releaseVersion", newPlan.Name)
		case -1:
			return nil, fmt.Errorf("new releaseVersion must be greater than the currently applied one ('%s')", oldPlan.Status.LastSuccessfulReleaseVersion)
		}
	}

	return nil, nil
}

func (*UpgradePlanValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateReleaseVersion(releaseVersion string) (*version.Version, error) {
	if releaseVersion == "" {
		return nil, fmt.Errorf("release version is required")
	}

	v, err := version.ParseSemantic(releaseVersion)
	if err != nil {
		return nil, fmt.Errorf("'%s' is not a semantic version", releaseVersion)
	}

	return v, nil
}
