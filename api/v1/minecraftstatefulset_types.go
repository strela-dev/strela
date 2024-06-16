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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinecraftStatefulSetSpec defines the desired state of MinecraftStatefulSet
type MinecraftStatefulSetSpec struct {
	Replicas        int                         `json:"replicas,omitempty"`
	MinReadySeconds int32                       `json:"minReadySeconds,omitempty"`
	Template        MinecraftServerTemplateSpec `json:"template,omitempty"`
	// +optional
	VolumeClaimTemplates []SelfVolumeClaimSpec `json:"volumeClaimTemplates,omitempty"`
}

type SelfVolumeClaimSpec struct {
	Metadata SelfMetadata                     `json:"metadata,omitempty"`
	Spec     corev1.PersistentVolumeClaimSpec `json:"spec,omitempty"`
}

type SelfMetadata struct {
	Name string `json:"name,omitempty"`
}

// MinecraftStatefulSetStatus defines the observed state of MinecraftStatefulSet
type MinecraftStatefulSetStatus struct {
	Replicas int `json:"replicas,omitempty"`
	Ready    int `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MinecraftStatefulSet is the Schema for the minecraftstatefulsets API
type MinecraftStatefulSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftStatefulSetSpec   `json:"spec,omitempty"`
	Status MinecraftStatefulSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MinecraftStatefulSetList contains a list of MinecraftStatefulSet
type MinecraftStatefulSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftStatefulSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftStatefulSet{}, &MinecraftStatefulSetList{})
}
