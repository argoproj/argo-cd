package helm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	lookupCallRegexp      = regexp.MustCompile(`(?s)\{\{[^}]*\blookup[\s(][^}]*\}\}`)
	templateCommentRegexp = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
)

// DetectLookupUsage returns the template files (relative to chartPath) that
// appear to invoke the Helm `lookup` function. The scan is best-effort and
// prefers false positives over false negatives.
func DetectLookupUsage(chartPath string) ([]string, error) {
	const maxTemplateScanBytes = 5 * 1024 * 1024

	var matches []string
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(chartPath, path)
		if relErr != nil {
			return nil
		}
		if !isHelmTemplateFile(rel) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if info.Size() > maxTemplateScanBytes {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		stripped := templateCommentRegexp.ReplaceAllString(string(data), "")
		if lookupCallRegexp.MatchString(stripped) {
			matches = append(matches, filepath.ToSlash(rel))
		}
		return nil
	}

	if err := filepath.WalkDir(chartPath, walkFn); err != nil {
		return nil, fmt.Errorf("failed to scan chart for lookup usage: %w", err)
	}
	return matches, nil
}

func isHelmTemplateFile(relPath string) bool {
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	return slices.Contains(parts[:len(parts)-1], "templates")
}
