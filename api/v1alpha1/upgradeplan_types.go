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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	UpgradePlanFinalizer = "upgradeplan.lifecycle.suse.com/finalizer"

	ValidationFailedCondition     = "ValidationFailed"
	UnsupportedArchitectureReason = "UnsupportedArchitecture"

	OperatingSystemUpgradedCondition = "OSUpgraded"
	KubernetesUpgradedCondition      = "KubernetesUpgraded"

	// UpgradeError indicates that the upgrade process has encountered a transient error.
	UpgradeError = "Error"

	// UpgradePending indicates that the upgrade process has not begun.
	UpgradePending = "Pending"

	// UpgradeInProgress indicates that the upgrade process has started.
	UpgradeInProgress = "InProgress"

	// UpgradeSkipped indicates that the upgrade has been skipped.
	UpgradeSkipped = "Skipped"

	// UpgradeSucceeded indicates that the upgrade process has been successful.
	UpgradeSucceeded = "Succeeded"

	// UpgradeFailed indicates that the upgrade process has failed.
	UpgradeFailed = "Failed"
)

// UpgradePlanSpec defines the desired state of UpgradePlan
type UpgradePlanSpec struct {
	// ReleaseVersion specifies the target version for platform upgrade.
	// The version format is X.Y.Z, for example "3.0.2".
	ReleaseVersion string `json:"releaseVersion"`
	// DisableDrain specifies whether control-plane and worker nodes drain should be disabled.
	// +optional
	DisableDrain *DisableDrain `json:"disableDrain"`
	// Helm specifies additional values for components installed via Helm.
	// It is only advised to use this field for values that are critical for upgrades.
	// Standard chart value updates should be performed after
	// the respective charts have been upgraded to the next version.
	// +optional
	Helm []HelmValues `json:"helm"`
}

type DisableDrain struct {
	// +optional
	ControlPlane bool `json:"controlPlane"`
	// +optional
	Worker bool `json:"worker"`
}

type HelmValues struct {
	Chart  string                `json:"chart"`
	Values *apiextensionsv1.JSON `json:"values"`
}

// UpgradePlanStatus defines the observed state of UpgradePlan
type UpgradePlanStatus struct {
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// ObservedGeneration is the currently tracked generation of the UpgradePlan. Meant for internal use only.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// SUCNameSuffix is the suffix added to all resources created for SUC. Meant for internal use only.
	// Changes for each new ObservedGeneration.
	SUCNameSuffix string `json:"sucNameSuffix,omitempty"`

	// LastSuccessfulReleaseVersion is the last release version that this UpgradePlan has successfully upgraded to.
	LastSuccessfulReleaseVersion string `json:"lastSuccessfulReleaseVersion,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// UpgradePlan is the Schema for the upgradeplans API
type UpgradePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpgradePlanSpec   `json:"spec,omitempty"`
	Status UpgradePlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UpgradePlanList contains a list of UpgradePlan
type UpgradePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpgradePlan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UpgradePlan{}, &UpgradePlanList{})
}

func GetChartConditionType(prettyName string) string {
	return fmt.Sprintf("%sUpgraded", prettyName)
}
