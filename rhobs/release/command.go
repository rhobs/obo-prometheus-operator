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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type command struct {
	Dir    string
	Args   []string
	Env    map[string]string
	Stream bool
}

type commandResult struct {
	Stdout string
	Stderr string
}

type commandRunner func(command) (commandResult, error)

type app struct {
	Root   string
	Out    io.Writer
	Err    io.Writer
	Runner commandRunner
}

func newApp(root string, out, stderr io.Writer) *app {
	a := &app{Root: root, Out: out, Err: stderr}
	a.Runner = func(c command) (commandResult, error) {
		cmd := exec.Command(c.Args[0], c.Args[1:]...)
		cmd.Dir = c.Dir
		cmd.Env = commandEnv(c.Env)
		var stdout, errors bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &errors
		if c.Stream {
			cmd.Stdout = io.MultiWriter(out, &stdout)
			cmd.Stderr = io.MultiWriter(stderr, &errors)
		}
		err := cmd.Run()
		return commandResult{Stdout: stdout.String(), Stderr: errors.String()}, err
	}
	return a
}

func commandEnv(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if _, overridden := overrides[name]; !overridden {
			env = append(env, value)
		}
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}

func (a *app) execute(c command) (string, error) {
	result, err := a.Runner(c)
	if err != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return "", fmt.Errorf("%s: %s: %w", strings.Join(c.Args, " "), detail, err)
	}
	return result.Stdout, nil
}

func (a *app) run(dir string, args ...string) (string, error) {
	return a.execute(command{Dir: dir, Args: args})
}

func (a *app) git(args ...string) (string, error) {
	return a.run(a.Root, append([]string{"git"}, args...)...)
}

func (a *app) gitOK(args ...string) bool {
	_, err := a.git(args...)
	return err == nil
}

func (a *app) gitShow(commit, path string) (string, error) {
	return a.git("show", commit+":"+path)
}

func (a *app) repoPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(a.Root, path)
}

func (a *app) fetchRef(repository, ref, privateRef string) (string, error) {
	if _, err := a.git("fetch", "--no-tags", repository, "+"+ref+":"+privateRef); err != nil {
		return "", err
	}
	commit, err := a.git("rev-parse", privateRef+"^{commit}")
	return strings.TrimSpace(commit), err
}

func (a *app) remoteRefs(repository, kind string) (map[string]string, error) {
	output, err := a.git("ls-remote", "--"+kind, repository)
	if err != nil {
		return nil, err
	}
	refs := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && !strings.HasSuffix(fields[1], "^{}") {
			refs[fields[1]] = fields[0]
		}
	}
	return refs, nil
}

func (a *app) assertClean() error {
	status, err := a.git("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("checkout is not clean:\n%s", status)
	}
	return nil
}

func temporaryParent() string {
	if path := os.Getenv("TMPDIR"); path != "" {
		return path
	}
	if info, err := os.Stat("/tmp/opencode"); err == nil && info.IsDir() {
		return "/tmp/opencode"
	}
	return ""
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}
