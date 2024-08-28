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

// K8SSelectorConfigApplyConfiguration represents an declarative configuration of the K8SSelectorConfig type for use
// with apply.
type K8SSelectorConfigApplyConfiguration struct {
	Role  *v1alpha1.KubernetesRole `json:"role,omitempty"`
	Label *string                  `json:"label,omitempty"`
	Field *string                  `json:"field,omitempty"`
}

// K8SSelectorConfigApplyConfiguration constructs an declarative configuration of the K8SSelectorConfig type for use with
// apply.
func K8SSelectorConfig() *K8SSelectorConfigApplyConfiguration {
	return &K8SSelectorConfigApplyConfiguration{}
}

// WithRole sets the Role field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Role field is set to the value of the last call.
func (b *K8SSelectorConfigApplyConfiguration) WithRole(value v1alpha1.KubernetesRole) *K8SSelectorConfigApplyConfiguration {
	b.Role = &value
	return b
}

// WithLabel sets the Label field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Label field is set to the value of the last call.
func (b *K8SSelectorConfigApplyConfiguration) WithLabel(value string) *K8SSelectorConfigApplyConfiguration {
	b.Label = &value
	return b
}

// WithField sets the Field field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Field field is set to the value of the last call.
func (b *K8SSelectorConfigApplyConfiguration) WithField(value string) *K8SSelectorConfigApplyConfiguration {
	b.Field = &value
	return b
}
