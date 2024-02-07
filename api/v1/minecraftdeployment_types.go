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
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type MinecraftDeploymentType string

const (
	Proxy  MinecraftDeploymentType = "PROXY"
	Server MinecraftDeploymentType = "SERVER"
)

// MinecraftDeploymentSpec defines the desired state of MinecraftDeployment
type MinecraftDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MinecraftDeployment. Edit minecraftdeployment_types.go to remove/update
	Replicas int                         `json:"replicas,omitempty"`
	Type     MinecraftDeploymentType     `json:"type,omitempty"`
	Template MinecraftServerTemplateSpec `json:"template,omitempty"`
}

// MinecraftDeploymentStatus defines the observed state of MinecraftDeployment
type MinecraftDeploymentStatus struct {
	Replicas int `json:"replicas,omitempty"`
	Ready    int `json:"ready,omitempty"`
	Ingame   int `json:"ingame,omitempty"`
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

const MinecraftServerSetOwnerKey string = ".metadata.controller"

func (m *MinecraftDeployment) GetMinecraftSets(cl client.Client, ctx context.Context) ([]MinecraftServerSet, error) {
	// List all MinecraftServerSets owned by this MinecraftDeployment
	var serverSets MinecraftServerSetList
	if err := cl.List(ctx, &serverSets, client.InNamespace("test"), client.MatchingFields{MinecraftServerSetOwnerKey: m.Name}); err != nil {
		return nil, err
	}
	return serverSets.Items, nil
}

func (deployment *MinecraftDeployment) GetActiveMinecraftServerSet(cl client.Client, ctx context.Context) (*MinecraftServerSet, error) {
	podTemplateHash, err := deployment.Spec.Template.GenerateTemplateSpecHash()
	if err != nil {
		return nil, err
	}
	hashedName := fmt.Sprintf("%s-%s", deployment.Name, podTemplateHash[:8])

	var minecraftServerSet MinecraftServerSet
	if err := cl.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: hashedName}, &minecraftServerSet); err != nil {
		return nil, err
	}

	return &minecraftServerSet, nil
}

func init() {
	SchemeBuilder.Register(&MinecraftDeployment{}, &MinecraftDeploymentList{})
}
