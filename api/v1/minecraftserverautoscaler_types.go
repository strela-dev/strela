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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MinecraftServerAutoscalerType string

const (
	ServerAutoscale MinecraftServerAutoscalerType = "SERVER"
	SlotAutoscale   MinecraftServerAutoscalerType = "SLOT"
)

// MinecraftServerAutoscalerSpec defines the desired state of MinecraftServerAutoscaler
type MinecraftServerAutoscalerSpec struct {
	TargetDeployment string                        `json:"targetDeployment"`
	Type             MinecraftServerAutoscalerType `json:"type"`
	Function         string                        `json:"function"`
	MinScalePause    int                           `json:"minScalePause"`
}

// MinecraftServerAutoscalerStatus defines the observed state of MinecraftServerAutoscaler
type MinecraftServerAutoscalerStatus struct {
	LastScaleTime int32 `json:"lastScaleTime"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MinecraftServerAutoscaler is the Schema for the minecraftserverautoscalers API
type MinecraftServerAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftServerAutoscalerSpec   `json:"spec,omitempty"`
	Status MinecraftServerAutoscalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MinecraftServerAutoscalerList contains a list of MinecraftServerAutoscaler
type MinecraftServerAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftServerAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftServerAutoscaler{}, &MinecraftServerAutoscalerList{})
}
