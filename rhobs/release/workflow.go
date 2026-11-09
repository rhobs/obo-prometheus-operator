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

package main

import (
	"fmt"
	"regexp"
)

func (a *app) sourceGoVersion(data Metadata) (string, error) {
	text, err := a.gitShow(data.SourceCommit, "go.mod")
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`(?m)^go[\t ]+([0-9]+\.[0-9]+(?:\.[0-9]+)?)[\t ]*(?://[^\n]*)?\r?$`).FindStringSubmatch(text)
	if match == nil {
		return "", fmt.Errorf("pinned source go.mod has no supported Go version directive")
	}
	return match[1], nil
}
