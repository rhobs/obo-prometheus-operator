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

package v1beta1

import (
	v1beta1 "github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring/v1beta1"
)

// TimeRangeApplyConfiguration represents an declarative configuration of the TimeRange type for use
// with apply.
type TimeRangeApplyConfiguration struct {
	StartTime *v1beta1.Time `json:"startTime,omitempty"`
	EndTime   *v1beta1.Time `json:"endTime,omitempty"`
}

// TimeRangeApplyConfiguration constructs an declarative configuration of the TimeRange type for use with
// apply.
func TimeRange() *TimeRangeApplyConfiguration {
	return &TimeRangeApplyConfiguration{}
}

// WithStartTime sets the StartTime field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the StartTime field is set to the value of the last call.
func (b *TimeRangeApplyConfiguration) WithStartTime(value v1beta1.Time) *TimeRangeApplyConfiguration {
	b.StartTime = &value
	return b
}

// WithEndTime sets the EndTime field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the EndTime field is set to the value of the last call.
func (b *TimeRangeApplyConfiguration) WithEndTime(value v1beta1.Time) *TimeRangeApplyConfiguration {
	b.EndTime = &value
	return b
}
