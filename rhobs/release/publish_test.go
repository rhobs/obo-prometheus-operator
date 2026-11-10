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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func (f *fixture) imagePublication(a *app, commit string, existing map[string]string, registryError bool) (commandResult, error, [][]string, map[string]string) {
	f.t.Helper()
	tools := filepath.Join(f.Dir, "fake-tools")
	if err := os.Mkdir(tools, 0o755); err != nil {
		f.t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		f.t.Fatal(err)
	}
	for _, name := range []string{"docker", "make", "rhobs-release"} {
		if err := os.Symlink(executable, filepath.Join(tools, name)); err != nil {
			f.t.Fatal(err)
		}
	}
	stateFile, logFile := filepath.Join(f.Dir, "registry.json"), filepath.Join(f.Dir, "commands.jsonl")
	content, err := json.Marshal(existing)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := writeFile(stateFile, content); err != nil {
		f.t.Fatal(err)
	}
	env := map[string]string{
		"PATH":              tools + string(os.PathListSeparator) + os.Getenv("PATH"),
		"RHOBS_RELEASE_BIN": filepath.Join(tools, "rhobs-release"), "RHOBS_TEST_HELPER": "1",
		"IMAGE_ORG": "rhobs", "CPU_ARCHS": "amd64", "FAKE_REGISTRY": stateFile,
		"FAKE_LOG": logFile, "FAKE_COMMIT": commit, "FAKE_REGISTRY_ERROR": strconv.FormatBool(registryError),
		"GORACE": "atexit_sleep_ms=0",
	}
	result, commandErr := a.Runner(command{Dir: a.Root, Env: env, Args: []string{"bash", "rhobs/publish-images.sh"}})
	var commands [][]string
	file, err := os.Open(logFile)
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		for {
			var entry []string
			err := decoder.Decode(&entry)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				f.t.Fatal(err)
			}
			commands = append(commands, entry)
		}
	} else if !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	var state map[string]string
	if err := json.Unmarshal([]byte(f.read(f.Dir, "registry.json")), &state); err != nil {
		f.t.Fatal(err)
	}
	return result, commandErr, commands, state
}

func TestImagePublication(t *testing.T) {
	for _, scenario := range []string{"reuse", "partial", "conflict", "registry error"} {
		t.Run(scenario, func(t *testing.T) {
			f := newFixture(t)
			data, a, commit := f.published()
			existing := make(map[string]string)
			operatorImage := "quay.io/rhobs/obo-prometheus-operator:v" + data.Version
			switch scenario {
			case "reuse":
				for _, image := range []string{"obo-prometheus-operator", "obo-prometheus-config-reloader", "obo-admission-webhook", "obo-prometheus-operator-bundle"} {
					existing["quay.io/rhobs/"+image+":v"+data.Version] = commit
				}
			case "partial":
				existing[operatorImage] = commit
			case "conflict":
				existing[operatorImage] = "different-commit"
			}
			result, err, commands, state := f.imagePublication(a, commit, existing, scenario == "registry error")
			if scenario == "reuse" || scenario == "partial" {
				if err != nil {
					t.Fatalf("publication failed: %v\n%s", err, result.Stderr)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe publication unexpectedly succeeded")
				}
				message := "Refusing to overwrite"
				if scenario == "registry error" {
					message = "authentication required"
				}
				if !strings.Contains(result.Stderr, message) {
					t.Fatalf("missing expected diagnostic %q: %s", message, result.Stderr)
				}
			}
			var promoted []string
			for _, command := range commands {
				created := len(command) >= 4 && reflect.DeepEqual(command[:4], []string{"docker", "buildx", "imagetools", "create"})
				if scenario != "partial" && (command[0] == "make" || created) {
					t.Fatalf("unexpected build/promotion: %v", command)
				}
				if created {
					for i, arg := range command {
						if arg == "--tag" && i+1 < len(command) && strings.HasSuffix(command[i+1], ":v"+data.Version) {
							promoted = append(promoted, command[i+1])
						}
					}
				}
			}
			if scenario == "partial" {
				requireEqual(t, len(promoted), 3)
				for _, image := range promoted {
					if image == operatorImage {
						t.Fatal("already published operator image was overwritten")
					}
				}
				requireEqual(t, state[operatorImage], commit)
			} else if !reflect.DeepEqual(state, existing) {
				t.Fatalf("existing registry artifacts changed: %v", state)
			}
		})
	}
}
