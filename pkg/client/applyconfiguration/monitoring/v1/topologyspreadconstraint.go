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

package v1

import (
	monitoringv1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologySpreadConstraintApplyConfiguration represents an declarative configuration of the TopologySpreadConstraint type for use
// with apply.
type TopologySpreadConstraintApplyConfiguration struct {
	CoreV1TopologySpreadConstraintApplyConfiguration `json:",inline"`
	AdditionalLabelSelectors                         *monitoringv1.AdditionalLabelSelectors `json:"additionalLabelSelectors,omitempty"`
}

// TopologySpreadConstraintApplyConfiguration constructs an declarative configuration of the TopologySpreadConstraint type for use with
// apply.
func TopologySpreadConstraint() *TopologySpreadConstraintApplyConfiguration {
	return &TopologySpreadConstraintApplyConfiguration{}
}

// WithMaxSkew sets the MaxSkew field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MaxSkew field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithMaxSkew(value int32) *TopologySpreadConstraintApplyConfiguration {
	b.MaxSkew = &value
	return b
}

// WithTopologyKey sets the TopologyKey field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the TopologyKey field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithTopologyKey(value string) *TopologySpreadConstraintApplyConfiguration {
	b.TopologyKey = &value
	return b
}

// WithWhenUnsatisfiable sets the WhenUnsatisfiable field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the WhenUnsatisfiable field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithWhenUnsatisfiable(value corev1.UnsatisfiableConstraintAction) *TopologySpreadConstraintApplyConfiguration {
	b.WhenUnsatisfiable = &value
	return b
}

// WithLabelSelector sets the LabelSelector field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the LabelSelector field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithLabelSelector(value metav1.LabelSelector) *TopologySpreadConstraintApplyConfiguration {
	b.LabelSelector = &value
	return b
}

// WithMinDomains sets the MinDomains field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MinDomains field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithMinDomains(value int32) *TopologySpreadConstraintApplyConfiguration {
	b.MinDomains = &value
	return b
}

// WithNodeAffinityPolicy sets the NodeAffinityPolicy field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the NodeAffinityPolicy field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithNodeAffinityPolicy(value corev1.NodeInclusionPolicy) *TopologySpreadConstraintApplyConfiguration {
	b.NodeAffinityPolicy = &value
	return b
}

// WithNodeTaintsPolicy sets the NodeTaintsPolicy field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the NodeTaintsPolicy field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithNodeTaintsPolicy(value corev1.NodeInclusionPolicy) *TopologySpreadConstraintApplyConfiguration {
	b.NodeTaintsPolicy = &value
	return b
}

// WithMatchLabelKeys adds the given value to the MatchLabelKeys field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MatchLabelKeys field.
func (b *TopologySpreadConstraintApplyConfiguration) WithMatchLabelKeys(values ...string) *TopologySpreadConstraintApplyConfiguration {
	for i := range values {
		b.MatchLabelKeys = append(b.MatchLabelKeys, values[i])
	}
	return b
}

// WithAdditionalLabelSelectors sets the AdditionalLabelSelectors field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the AdditionalLabelSelectors field is set to the value of the last call.
func (b *TopologySpreadConstraintApplyConfiguration) WithAdditionalLabelSelectors(value monitoringv1.AdditionalLabelSelectors) *TopologySpreadConstraintApplyConfiguration {
	b.AdditionalLabelSelectors = &value
	return b
}
