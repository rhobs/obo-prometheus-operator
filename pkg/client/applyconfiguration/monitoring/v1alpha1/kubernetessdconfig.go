// Copyright The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1alpha1"
)

// KubernetesSDConfigApplyConfiguration represents an declarative configuration of the KubernetesSDConfig type for use
// with apply.
type KubernetesSDConfigApplyConfiguration struct {
	Role      *v1alpha1.K8SRole                     `json:"role,omitempty"`
	Selectors []K8SSelectorConfigApplyConfiguration `json:"selectors,omitempty"`
}

// KubernetesSDConfigApplyConfiguration constructs an declarative configuration of the KubernetesSDConfig type for use with
// apply.
func KubernetesSDConfig() *KubernetesSDConfigApplyConfiguration {
	return &KubernetesSDConfigApplyConfiguration{}
}

// WithRole sets the Role field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Role field is set to the value of the last call.
func (b *KubernetesSDConfigApplyConfiguration) WithRole(value v1alpha1.K8SRole) *KubernetesSDConfigApplyConfiguration {
	b.Role = &value
	return b
}

// WithSelectors adds the given value to the Selectors field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Selectors field.
func (b *KubernetesSDConfigApplyConfiguration) WithSelectors(values ...*K8SSelectorConfigApplyConfiguration) *KubernetesSDConfigApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithSelectors")
		}
		b.Selectors = append(b.Selectors, *values[i])
	}
	return b
}
