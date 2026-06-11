package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	resourceCustomizationsDir = "resource_customizations"
	deletionBlock             = `-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

`
	terminatingTestPath  = "testdata/terminating.yaml"
	terminatingInputPath = "testdata/terminating.yaml"
	terminatingFinalizer = "example.com/finalizer"
	deletionTimestamp    = "2024-01-01T00:00:00Z"
)

type healthTestFile struct {
	Tests []struct {
		InputPath string `yaml:"inputPath"`
	} `yaml:"tests"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var healthLuaFiles []string
	err := filepath.Walk(resourceCustomizationsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || info.Name() != "health.lua" {
			return nil
		}
		healthLuaFiles = append(healthLuaFiles, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk resource customizations: %w", err)
	}

	var updatedLua, updatedTests int
	for _, healthLuaPath := range healthLuaFiles {
		dir := filepath.Dir(healthLuaPath)
		luaUpdated, err := updateHealthLua(healthLuaPath)
		if err != nil {
			return fmt.Errorf("%s: %w", healthLuaPath, err)
		}
		if luaUpdated {
			updatedLua++
		}

		testUpdated, err := updateHealthTests(dir)
		if err != nil {
			return fmt.Errorf("%s: %w", dir, err)
		}
		if testUpdated {
			updatedTests++
		}
	}

	fmt.Printf("updated %d health.lua files and %d health test directories\n", updatedLua, updatedTests)
	return nil
}

func updateHealthLua(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(content), "deletionTimestamp") {
		return false, nil
	}
	updated := deletionBlock + string(content)
	return true, os.WriteFile(path, []byte(updated), 0o644)
}

func updateHealthTests(dir string) (bool, error) {
	healthTestPath := filepath.Join(dir, "health_test.yaml")
	terminatingPath := filepath.Join(dir, terminatingTestPath)

	testBytes, err := os.ReadFile(healthTestPath)
	if err != nil {
		return false, fmt.Errorf("read health_test.yaml: %w", err)
	}

	content := removeTerminatingTestCase(string(testBytes))
	if hasTerminatingTestCase(content) {
		return false, nil
	}

	var tests healthTestFile
	if err := yaml.Unmarshal([]byte(content), &tests); err != nil {
		return false, fmt.Errorf("parse health_test.yaml: %w", err)
	}
	if len(tests.Tests) == 0 {
		return false, errors.New("health_test.yaml has no tests")
	}

	firstInput := tests.Tests[0].InputPath
	samplePath := filepath.Join(dir, firstInput)
	apiVersion, kind, err := readAPIVersionKind(samplePath)
	if err != nil {
		return false, fmt.Errorf("read sample testdata %s: %w", firstInput, err)
	}

	terminatingYAML := fmt.Sprintf(`apiVersion: %s
kind: %s
metadata:
  name: terminating-resource
  deletionTimestamp: "%s"
  finalizers:
    - %s
`, apiVersion, kind, deletionTimestamp, terminatingFinalizer)

	if err := os.MkdirAll(filepath.Dir(terminatingPath), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(terminatingPath, []byte(terminatingYAML), 0o644); err != nil {
		return false, err
	}

	updatedTests := strings.TrimRight(content, "\n") + "\n" + terminatingTestCase(content)
	return true, os.WriteFile(healthTestPath, []byte(updatedTests), 0o644)
}

func removeTerminatingTestCase(content string) string {
	patterns := []string{
		"\n- healthStatus:\n    status: Progressing\n    message: \"Pending deletion; blocked by finalizers: example.com/finalizer\"\n  inputPath: testdata/terminating.yaml",
		"\n  - healthStatus:\n      status: Progressing\n      message: \"Pending deletion; blocked by finalizers: example.com/finalizer\"\n    inputPath: testdata/terminating.yaml",
	}
	for _, pattern := range patterns {
		content = strings.ReplaceAll(content, pattern, "")
	}
	return strings.TrimRight(content, "\n") + "\n"
}

func hasTerminatingTestCase(content string) bool {
	var tests healthTestFile
	if err := yaml.Unmarshal([]byte(content), &tests); err != nil {
		return false
	}
	for _, test := range tests.Tests {
		if test.InputPath == terminatingInputPath {
			return true
		}
	}
	return false
}

func terminatingTestCase(existingContent string) string {
	listIndent := ""
	statusIndent := "    "
	inputIndent := "  "
	for line := range strings.SplitSeq(existingContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, "- healthStatus:") || trimmed == "- healthStatus:" {
			if idx := strings.Index(line, "- healthStatus:"); idx > 0 {
				listIndent = line[:idx]
				statusIndent = listIndent + "    "
				inputIndent = listIndent + "  "
			}
			break
		}
	}
	return fmt.Sprintf(`%s- healthStatus:
%sstatus: Progressing
%smessage: "Pending deletion; blocked by finalizers: example.com/finalizer"
%sinputPath: testdata/terminating.yaml
`, listIndent, statusIndent, statusIndent, inputIndent)
}

func readAPIVersionKind(path string) (string, string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var obj map[string]any
	if err := yaml.Unmarshal(bytes, &obj); err != nil {
		return "", "", err
	}
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	if apiVersion == "" || kind == "" {
		return "", "", fmt.Errorf("missing apiVersion or kind in %s", path)
	}
	return apiVersion, kind, nil
}
