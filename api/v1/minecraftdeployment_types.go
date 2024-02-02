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

type MinecraftDeploymentType string

const (
	Proxy  MinecraftDeploymentType = "PROXY"
	Lobby  MinecraftDeploymentType = "LOBBY"
	Server MinecraftDeploymentType = "SERVER"
)

type MinecraftDeploymentStaticSpec struct {
	Enabled         bool   `json:"enabled"`
	StorageCapacity string `json:"storageCapacity"`
}

// MinecraftDeploymentSpec defines the desired state of MinecraftDeployment
type MinecraftDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MinecraftDeployment. Edit minecraftdeployment_types.go to remove/update
	Replicas      int                           `json:"replicas"`
	Type          MinecraftDeploymentType       `json:"type"`
	Static        MinecraftDeploymentStaticSpec `json:"static"`
	LobbyPriority int                           `json:"lobbyPriority"`
	Template      MinecraftServerTemplateSpec   `json:"template"`
}

// MinecraftDeploymentStatus defines the observed state of MinecraftDeployment
type MinecraftDeploymentStatus struct {
	Replicas int `json:"replicas"`
	Ready    int `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MinecraftDeployment is the Schema for the minecraftdeployments API
type MinecraftDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftDeploymentSpec   `json:"spec,omitempty"`
	Status MinecraftDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MinecraftDeploymentList contains a list of MinecraftDeployment
type MinecraftDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftDeployment{}, &MinecraftDeploymentList{})
}
