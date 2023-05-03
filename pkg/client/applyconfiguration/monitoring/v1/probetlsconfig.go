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
	corev1 "k8s.io/api/core/v1"
)

// ProbeTLSConfigApplyConfiguration represents an declarative configuration of the ProbeTLSConfig type for use
// with apply.
type ProbeTLSConfigApplyConfiguration struct {
	SafeTLSConfigApplyConfiguration `json:",inline"`
}

// ProbeTLSConfigApplyConfiguration constructs an declarative configuration of the ProbeTLSConfig type for use with
// apply.
func ProbeTLSConfig() *ProbeTLSConfigApplyConfiguration {
	return &ProbeTLSConfigApplyConfiguration{}
}

// WithCA sets the CA field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CA field is set to the value of the last call.
func (b *ProbeTLSConfigApplyConfiguration) WithCA(value *SecretOrConfigMapApplyConfiguration) *ProbeTLSConfigApplyConfiguration {
	b.CA = value
	return b
}

// WithCert sets the Cert field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Cert field is set to the value of the last call.
func (b *ProbeTLSConfigApplyConfiguration) WithCert(value *SecretOrConfigMapApplyConfiguration) *ProbeTLSConfigApplyConfiguration {
	b.Cert = value
	return b
}

// WithKeySecret sets the KeySecret field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the KeySecret field is set to the value of the last call.
func (b *ProbeTLSConfigApplyConfiguration) WithKeySecret(value corev1.SecretKeySelector) *ProbeTLSConfigApplyConfiguration {
	b.KeySecret = &value
	return b
}

// WithServerName sets the ServerName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ServerName field is set to the value of the last call.
func (b *ProbeTLSConfigApplyConfiguration) WithServerName(value string) *ProbeTLSConfigApplyConfiguration {
	b.ServerName = &value
	return b
}

// WithInsecureSkipVerify sets the InsecureSkipVerify field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the InsecureSkipVerify field is set to the value of the last call.
func (b *ProbeTLSConfigApplyConfiguration) WithInsecureSkipVerify(value bool) *ProbeTLSConfigApplyConfiguration {
	b.InsecureSkipVerify = &value
	return b
}