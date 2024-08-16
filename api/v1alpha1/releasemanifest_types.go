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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReleaseManifestSpec defines the desired state of ReleaseManifest
type ReleaseManifestSpec struct {
	ReleaseVersion string     `json:"releaseVersion"`
	Components     Components `json:"components,omitempty"`
}

// ReleaseManifestStatus defines the observed state of ReleaseManifest
type ReleaseManifestStatus struct {
}

type Components struct {
	Kubernetes      Kubernetes      `json:"kubernetes"`
	OperatingSystem OperatingSystem `json:"operatingSystem"`
	Workloads       Workloads       `json:"workloads"`
}

type Workloads struct {
	Helm []HelmChart `json:"helm"`
}

type HelmChart struct {
	ReleaseName string `json:"releaseName"`
	Name        string `json:"chart"`
	Repository  string `json:"repository,omitempty"`
	Version     string `json:"version"`
	PrettyName  string `json:"prettyName"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	DependencyCharts []HelmChart `json:"dependencyCharts,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AddonCharts []HelmChart `json:"addonCharts,omitempty"`
}

type Kubernetes struct {
	K3S  KubernetesDistribution `json:"k3s"`
	RKE2 KubernetesDistribution `json:"rke2"`
}

type KubernetesDistribution struct {
	Version string `json:"version"`
}

type OperatingSystem struct {
	Version        string   `json:"version"`
	ZypperID       string   `json:"zypperID"`
	CPEScheme      string   `json:"cpeScheme"`
	RepoGPGPath    string   `json:"repoGPGPath"`
	SupportedArchs []string `json:"supportedArchs"`
	PrettyName     string   `json:"prettyName"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ReleaseManifest is the Schema for the releasemanifests API
type ReleaseManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseManifestSpec   `json:"spec,omitempty"`
	Status ReleaseManifestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReleaseManifestList contains a list of ReleaseManifest
type ReleaseManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReleaseManifest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReleaseManifest{}, &ReleaseManifestList{})
}