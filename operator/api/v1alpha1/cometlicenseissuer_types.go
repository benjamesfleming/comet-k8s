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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CometLicenseFeatures map[string]int

// CometLicenseIssuerAuth defines the API authentication for account.cometbackup.com
type CometLicenseIssuerAuth struct {
	Email string `json:"email,omitempty"`
	Token string `json:"token,omitempty"`
}

// CometLicenseIssuerSpec defines the desired state of CometLicenseIssuer
type CometLicenseIssuerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Auth     CometLicenseIssuerAuth `json:"auth,omitempty"`
	Features CometLicenseFeatures   `json:"features,omitempty"`
}

// CometLicenseIssuerStatus defines the observed state of CometLicenseIssuer
type CometLicenseIssuerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CometLicenseIssuer is the Schema for the cometlicenseissuers API
type CometLicenseIssuer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CometLicenseIssuerSpec   `json:"spec,omitempty"`
	Status CometLicenseIssuerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CometLicenseIssuerList contains a list of CometLicenseIssuer
type CometLicenseIssuerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CometLicenseIssuer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CometLicenseIssuer{}, &CometLicenseIssuerList{})
}
