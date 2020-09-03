/*
Copyright 2017 The Kubernetes Authors.

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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Icecream is a specification for a Icecream resource
type Icecream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IcecreamSpec   `json:"spec"`
	Status IcecreamStatus `json:"status"`
}

// IcecreamSpec is the spec for an Icecream resource
type IcecreamSpec struct {
	Flavor         string `json:"flavor"`
}

// IcecreamStatus is the status for a Icecream resource
type IcecreamStatus struct {
	// PodIPs contains the IP of all pods with the corresponding flavor
	// as specified in IcecreamSpec
	PodIPs []string
	// PodNames contains the name of all pods with the corresponding flavor
	// as specified in IcecreamSpec
	PodNames []string
	// TotalAllocatedCPU contains the total allocated CPU for all pods with the corresponding
	// flavor as specified in IcecreamSpec
	TotalAllocatedCPU string
	// TotalAllocatedMemory contains the total allocated memory for all pods with the
	// corresponding flavor as specified in IcecreamSpec
	TotalAllocatedMemory string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IcecreamList is a list of Icecream resources
type IcecreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Icecream `json:"items"`
}
