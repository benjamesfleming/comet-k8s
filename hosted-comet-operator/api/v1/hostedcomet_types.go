/*
Copyright 2022.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

var (
	letterMap = []rune("abcdefghijklmnopqrstuvwxyz")
)

// HostedCometSpec defines the desired state of HostedComet
type HostedCometSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Region     string `json:"region,omitempty"`
	HostedZone string `json:"hostedZone,omitempty"`
	Replicas   int    `json:"replicas,omitempty"`
}

// HostedCometStatus defines the observed state of HostedComet
type HostedCometStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HostedComet is the Schema for the hostedcomets API
type HostedComet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HostedCometSpec   `json:"spec,omitempty"`
	Status HostedCometStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HostedCometList contains a list of HostedComet
type HostedCometList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostedComet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostedComet{}, &HostedCometList{})
}

// --

func (h *HostedComet) GetRegionFQDN() string {
	return fmt.Sprintf("%s.%s", h.Spec.Region, h.Spec.HostedZone)
}

func (h *HostedComet) GetPodFQDN(i int) string {
	return fmt.Sprintf("%s%c.%s", h.Spec.Region, letterMap[i], h.Spec.HostedZone)
}
