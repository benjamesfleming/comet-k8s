/*
Copyright 2023.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CometServerLicense struct {
	Issuer   string               `json:"issuer,omitempty"`
	Features CometLicenseFeatures `json:"features,omitempty"`
}

type CometServerIngress struct {
	Host string `json:"host,omitempty"`
}

// CometServerSpec defines the desired state of CometServer
type CometServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Version string             `json:"version,omitempty"`
	License CometServerLicense `json:"license,omitempty"`
	Ingress CometServerIngress `json:"ingress,omitempty"`
}

// CometServerStatus defines the observed state of CometServer
type CometServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CometServer is the Schema for the cometservers API
type CometServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CometServerSpec   `json:"spec,omitempty"`
	Status CometServerStatus `json:"status,omitempty"`
}

// SerialNumber of the Comet Server resource. Empty string if not configured.
func (cs *CometServer) SerialNumber() string {
	if sn, ok := cs.Annotations["cometd.cometbackup.com/serial-number"]; ok {
		return sn
	}
	return ""
}

func (cs *CometServer) FQDN() string {
	return fmt.Sprintf("%s.%s", cs.Name, cs.Spec.Ingress.Host)
}

//+kubebuilder:object:root=true

// CometServerList contains a list of CometServer
type CometServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CometServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CometServer{}, &CometServerList{})
}
