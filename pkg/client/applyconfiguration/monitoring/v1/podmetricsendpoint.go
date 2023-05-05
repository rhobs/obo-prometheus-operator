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
	v1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

// PodMetricsEndpointApplyConfiguration represents an declarative configuration of the PodMetricsEndpoint type for use
// with apply.
type PodMetricsEndpointApplyConfiguration struct {
	Port                 *string                                        `json:"port,omitempty"`
	TargetPort           *intstr.IntOrString                            `json:"targetPort,omitempty"`
	Path                 *string                                        `json:"path,omitempty"`
	Scheme               *string                                        `json:"scheme,omitempty"`
	Params               map[string][]string                            `json:"params,omitempty"`
	Interval             *v1.Duration                                   `json:"interval,omitempty"`
	ScrapeTimeout        *v1.Duration                                   `json:"scrapeTimeout,omitempty"`
	TLSConfig            *PodMetricsEndpointTLSConfigApplyConfiguration `json:"tlsConfig,omitempty"`
	BearerTokenSecret    *corev1.SecretKeySelector                      `json:"bearerTokenSecret,omitempty"`
	HonorLabels          *bool                                          `json:"honorLabels,omitempty"`
	HonorTimestamps      *bool                                          `json:"honorTimestamps,omitempty"`
	BasicAuth            *BasicAuthApplyConfiguration                   `json:"basicAuth,omitempty"`
	OAuth2               *OAuth2ApplyConfiguration                      `json:"oauth2,omitempty"`
	Authorization        *SafeAuthorizationApplyConfiguration           `json:"authorization,omitempty"`
	MetricRelabelConfigs []*v1.RelabelConfig                            `json:"metricRelabelings,omitempty"`
	RelabelConfigs       []*v1.RelabelConfig                            `json:"relabelings,omitempty"`
	ProxyURL             *string                                        `json:"proxyUrl,omitempty"`
	FollowRedirects      *bool                                          `json:"followRedirects,omitempty"`
	EnableHttp2          *bool                                          `json:"enableHttp2,omitempty"`
	FilterRunning        *bool                                          `json:"filterRunning,omitempty"`
}

// PodMetricsEndpointApplyConfiguration constructs an declarative configuration of the PodMetricsEndpoint type for use with
// apply.
func PodMetricsEndpoint() *PodMetricsEndpointApplyConfiguration {
	return &PodMetricsEndpointApplyConfiguration{}
}

// WithPort sets the Port field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Port field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithPort(value string) *PodMetricsEndpointApplyConfiguration {
	b.Port = &value
	return b
}

// WithTargetPort sets the TargetPort field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the TargetPort field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithTargetPort(value intstr.IntOrString) *PodMetricsEndpointApplyConfiguration {
	b.TargetPort = &value
	return b
}

// WithPath sets the Path field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Path field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithPath(value string) *PodMetricsEndpointApplyConfiguration {
	b.Path = &value
	return b
}

// WithScheme sets the Scheme field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Scheme field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithScheme(value string) *PodMetricsEndpointApplyConfiguration {
	b.Scheme = &value
	return b
}

// WithParams puts the entries into the Params field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Params field,
// overwriting an existing map entries in Params field with the same key.
func (b *PodMetricsEndpointApplyConfiguration) WithParams(entries map[string][]string) *PodMetricsEndpointApplyConfiguration {
	if b.Params == nil && len(entries) > 0 {
		b.Params = make(map[string][]string, len(entries))
	}
	for k, v := range entries {
		b.Params[k] = v
	}
	return b
}

// WithInterval sets the Interval field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Interval field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithInterval(value v1.Duration) *PodMetricsEndpointApplyConfiguration {
	b.Interval = &value
	return b
}

// WithScrapeTimeout sets the ScrapeTimeout field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ScrapeTimeout field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithScrapeTimeout(value v1.Duration) *PodMetricsEndpointApplyConfiguration {
	b.ScrapeTimeout = &value
	return b
}

// WithTLSConfig sets the TLSConfig field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the TLSConfig field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithTLSConfig(value *PodMetricsEndpointTLSConfigApplyConfiguration) *PodMetricsEndpointApplyConfiguration {
	b.TLSConfig = value
	return b
}

// WithBearerTokenSecret sets the BearerTokenSecret field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BearerTokenSecret field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithBearerTokenSecret(value corev1.SecretKeySelector) *PodMetricsEndpointApplyConfiguration {
	b.BearerTokenSecret = &value
	return b
}

// WithHonorLabels sets the HonorLabels field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the HonorLabels field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithHonorLabels(value bool) *PodMetricsEndpointApplyConfiguration {
	b.HonorLabels = &value
	return b
}

// WithHonorTimestamps sets the HonorTimestamps field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the HonorTimestamps field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithHonorTimestamps(value bool) *PodMetricsEndpointApplyConfiguration {
	b.HonorTimestamps = &value
	return b
}

// WithBasicAuth sets the BasicAuth field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BasicAuth field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithBasicAuth(value *BasicAuthApplyConfiguration) *PodMetricsEndpointApplyConfiguration {
	b.BasicAuth = value
	return b
}

// WithOAuth2 sets the OAuth2 field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the OAuth2 field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithOAuth2(value *OAuth2ApplyConfiguration) *PodMetricsEndpointApplyConfiguration {
	b.OAuth2 = value
	return b
}

// WithAuthorization sets the Authorization field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Authorization field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithAuthorization(value *SafeAuthorizationApplyConfiguration) *PodMetricsEndpointApplyConfiguration {
	b.Authorization = value
	return b
}

// WithMetricRelabelConfigs adds the given value to the MetricRelabelConfigs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MetricRelabelConfigs field.
func (b *PodMetricsEndpointApplyConfiguration) WithMetricRelabelConfigs(values ...**v1.RelabelConfig) *PodMetricsEndpointApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithMetricRelabelConfigs")
		}
		b.MetricRelabelConfigs = append(b.MetricRelabelConfigs, *values[i])
	}
	return b
}

// WithRelabelConfigs adds the given value to the RelabelConfigs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the RelabelConfigs field.
func (b *PodMetricsEndpointApplyConfiguration) WithRelabelConfigs(values ...**v1.RelabelConfig) *PodMetricsEndpointApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithRelabelConfigs")
		}
		b.RelabelConfigs = append(b.RelabelConfigs, *values[i])
	}
	return b
}

// WithProxyURL sets the ProxyURL field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ProxyURL field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithProxyURL(value string) *PodMetricsEndpointApplyConfiguration {
	b.ProxyURL = &value
	return b
}

// WithFollowRedirects sets the FollowRedirects field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the FollowRedirects field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithFollowRedirects(value bool) *PodMetricsEndpointApplyConfiguration {
	b.FollowRedirects = &value
	return b
}

// WithEnableHttp2 sets the EnableHttp2 field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the EnableHttp2 field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithEnableHttp2(value bool) *PodMetricsEndpointApplyConfiguration {
	b.EnableHttp2 = &value
	return b
}

// WithFilterRunning sets the FilterRunning field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the FilterRunning field is set to the value of the last call.
func (b *PodMetricsEndpointApplyConfiguration) WithFilterRunning(value bool) *PodMetricsEndpointApplyConfiguration {
	b.FilterRunning = &value
	return b
}
