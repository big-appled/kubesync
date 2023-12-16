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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GroupVersionResource struct {
	Group    string `json:"group,omitempty"`
	Version  string `json:"version,omitempty"`
	Resource string `json:"resource,omitempty"`
}
type SyncResource struct {
	Names      []string             `json:"names,omitempty"`
	Namespaces []string             `json:"namespaces,omitempty"`
	GVR        GroupVersionResource `json:"gvr"`
}

// KubeSyncSpec defines the desired state of KubeSync
type KubeSyncSpec struct {
	SrcClusterSecret string            `json:"srcClusterSecret"`
	DstClusterSecret string            `json:"dstClusterSecret"`
	SyncResources    []SyncResource    `json:"syncResources"`
	NsMap            map[string]string `json:"nsMap,omitempty"`
	Pause            bool              `json:"pause,omitempty"`
}

type KsState string

const (
	Ready    KsState = "Ready"
	NotReady KsState = "NotReady"
	Paused   KsState = "Paused"
	Deleting KsState = "Deleting"
)

// KubeSyncStatus defines the observed state of KubeSync
type KubeSyncStatus struct {
	State KsState `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="SrcClusterSecret",type=string,JSONPath=".spec.srcClusterSecret"
//+kubebuilder:printcolumn:name="DstClusterSecret",type=string,JSONPath=".spec.dstClusterSecret"
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=".status.state"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// KubeSync is the Schema for the kubesyncs API
type KubeSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeSyncSpec   `json:"spec,omitempty"`
	Status KubeSyncStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeSyncList contains a list of KubeSync
type KubeSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeSync{}, &KubeSyncList{})
}

func (s *KubeSync) SetState(state KsState) {
	s.Status.State = state
}

func (s *KubeSync) SetReady() {
	s.SetState(Ready)
}

func (s *KubeSync) SetNotReady() {
	s.SetState(NotReady)
}

func (s *KubeSync) SetDeleting() {
	s.SetState(Deleting)
}

func (s *KubeSync) SetPaused() {
	s.SetState(Paused)
}

func (s *KubeSync) IsReady() bool {
	return s.Status.State == Ready
}

func (s *KubeSync) IsNotReady() bool {
	return s.Status.State == NotReady
}

func (s *KubeSync) IsDeleting() bool {
	return s.Status.State == Deleting
}

func (s *KubeSync) IsPaused() bool {
	return s.Status.State == Paused
}
