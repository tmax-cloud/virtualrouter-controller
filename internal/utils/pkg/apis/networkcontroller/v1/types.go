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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualRouter is a specification for a VirtualRouter resource
type VirtualRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualRouterSpec   `json:"spec"`
	Status VirtualRouterStatus `json:"status"`
}

// VirtualRouterSpec is the spec for a VirtualRouter resource
type VirtualRouterSpec struct {
	DeploymentName  string         `json:"deploymentName"`
	Replicas        *int32         `json:"replicas"`
	VlanNumber      int32          `json:"vlanNumber" `
	InternalIP      string         `json:"internalIP"`
	InternalNetmask string         `json:"internalNetmask"`
	ExternalIP      string         `json:"externalIP"`
	ExternalNetmask string         `json:"externalNetmask"`
	GatewayIP       string         `json:"gatewayIP"`
	Image           string         `json:"image"`
	NodeSelector    []NodeSelector `json:"nodeSelector"`
}

// VirtualRouterStatus is the status for a VirtualRouter resource
type VirtualRouterStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualRouterList is a list of VirtualRouter resources
type VirtualRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VirtualRouter `json:"items"`
}

type NodeSelector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
