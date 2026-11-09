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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceGoVersionAndRelativePlanPath(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	version, err := f.App.sourceGoVersion(data)
	if err != nil {
		t.Fatal(err)
	}
	requireEqual(t, version, "1.20")
	file, err := os.Create(filepath.Join(f.Controller, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(file, data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCLI([]string{"--root", f.Controller, "go-version", "--plan", "plan.json"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	requireEqual(t, output.String(), "1.20\n")
}

func TestPlanRequiresBooleanChanged(t *testing.T) {
	f := newFixture(t)
	data := f.plan()
	data.Changed = nil
	file, err := os.Create(filepath.Join(f.Dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(file, data); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	options := prepareOptions{planOptions: defaultPlanOptions(), PlanFile: file.Name()}
	_, err = f.App.preparePlan(options)
	requireError(t, err, "boolean changed field")
}

func TestWrapperIgnoresParentWorkspaceAndVendorFlags(t *testing.T) {
	f := newFixture(t)
	plan := filepath.Join(f.Dir, "plan.json")
	data := f.plan()
	var contents bytes.Buffer
	if err := writeJSON(&contents, data); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(plan, contents.Bytes()); err != nil {
		t.Fatal(err)
	}
	result, err := f.App.Runner(command{
		Dir:  filepath.Join(f.Controller, "rhobs"),
		Args: []string{"bash", "release.sh", "go-version", "--plan", plan},
		Env: map[string]string{
			"GOFLAGS": "-mod=vendor", "GOWORK": filepath.Join(f.Dir, "nonexistent.work"),
			"RHOBS_RELEASE_BIN": "",
		},
	})
	if err != nil {
		t.Fatalf("wrapper failed to isolate the tool module: %v\n%s", err, result.Stderr)
	}
	requireEqual(t, strings.TrimSpace(result.Stdout), "1.20")
}
