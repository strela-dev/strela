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
	"fmt"
	"hash/fnv"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

type ConfigurationMode string

const (
	Bungeecord       ConfigurationMode = "BUNGEECORD"
	Velocity         ConfigurationMode = "VELOCITY"
	PaperVelocity    ConfigurationMode = "PAPER_VELOCITY"
	SpgiotBungeecord ConfigurationMode = "SPIGOT_BUNGEECORD"
)

type MinecraftServerType string

const (
	Proxy  MinecraftServerType = "PROXY"
	Server MinecraftServerType = "SERVER"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinecraftServerSpec defines the desired state of MinecraftServer
type MinecraftServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MinecraftServer. Edit minecraftserver_types.go to remove/update
	Container         string                 `json:"container,omitempty"`
	Type              MinecraftServerType    `json:"serverType,omitempty"`
	ConfigurationMode ConfigurationMode      `json:"configurationMode,omitempty"`
	ConfigDir         string                 `json:"configDir,omitempty"`
	MaxPlayers        int                    `json:"maxPlayers,omitempty"`
	Template          corev1.PodTemplateSpec `json:"template"`
}

// MinecraftServerStatus defines the observed state of MinecraftServer
type MinecraftServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Ingame      bool        `json:"ingame,omitempty"`
	Ready       bool        `json:"ready,omitempty"`
	PlayerCount int         `json:"playerCount,omitempty"`
	ReadyTime   metav1.Time `json:"readyTime,omitempty"`
	IngameTime  metav1.Time `json:"ingameTime,omitempty"`
	PodIP       string      `json:"podIP,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MinecraftServer is the Schema for the minecraftservers API
type MinecraftServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftServerSpec   `json:"spec,omitempty"`
	Status MinecraftServerStatus `json:"status,omitempty"`
}

type MinecraftServerTemplateSpec struct {
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec MinecraftServerSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// MinecraftServerList contains a list of MinecraftServer
type MinecraftServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftServer `json:"items"`
}

func (template *MinecraftServerTemplateSpec) GenerateTemplateSpecHash() (string, error) {
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(fmt.Sprintf("%+v", *template))); err != nil {
		return "", err
	}
	return strconv.FormatUint(uint64(hasher.Sum32()), 16), nil
}

func init() {
	SchemeBuilder.Register(&MinecraftServer{}, &MinecraftServerList{})
}
