package helm

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Match the pinned Helm directory loader's chart read budget. Optional
// prewarming falls back to Helm when metadata cannot be read within it.
const maxChartMetadataBytes int64 = 100 * 1024 * 1024

func readChartMetadataFile(dir, name string) ([]byte, error) {
	file := filepath.Join(dir, name)
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxChartMetadataBytes {
		return nil, fmt.Errorf("unsupported or oversized Helm metadata file %s", name)
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxChartMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxChartMetadataBytes {
		return nil, fmt.Errorf("oversized Helm metadata file %s", name)
	}
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), nil
}

// ReadChartMetadata reads a root chart metadata file with Helm's size bound
// and root-file .helmignore semantics. An ignored file behaves as absent.
func ReadChartMetadata(dir, name string) ([]byte, error) {
	rules, err := readChartMetadataFile(dir, ".helmignore")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	ignored := false
	scanner := bufio.NewScanner(bytes.NewReader(rules))
	for scanner.Scan() {
		rule := strings.TrimSpace(scanner.Text())
		if rule == "" || strings.HasPrefix(rule, "#") {
			continue
		}
		if strings.Contains(rule, "**") {
			return nil, errors.New("double-star syntax is not supported in .helmignore")
		}
		if _, err := filepath.Match(rule, "abc"); err != nil {
			return nil, err
		}
		negative := strings.HasPrefix(rule, "!")
		rule = strings.TrimPrefix(rule, "!")
		directory := strings.HasSuffix(rule, "/")
		rule = strings.TrimPrefix(strings.TrimSuffix(rule, "/"), "/")
		match, _ := filepath.Match(rule, name)
		// Helm's negative rules ignore nonmatches; they are not Git-style
		// reinclusions. Root metadata files never satisfy directory rules.
		if negative && (directory || !match) || !negative && !directory && match {
			ignored = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if ignored {
		return nil, &os.PathError{Op: "read", Path: filepath.Join(dir, name), Err: os.ErrNotExist}
	}
	return readChartMetadataFile(dir, name)
}
