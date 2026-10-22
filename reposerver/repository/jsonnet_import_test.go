package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/git"
)

const (
	// outsideSentinel is written only to files that a confined importer must not read.
	outsideSentinel = "JSONNET-OUTSIDE-SENTINEL"
	// insideSentinel is written only to files a relative in-repo import is allowed to read.
	insideSentinel = "JSONNET-INSIDE-SENTINEL"
)

// jsonnetFixture is a temp checkout. Symlinks that point outside the repo live
// next to the app directory, not inside it, so the manifest walk does not
// reject them before Jsonnet evaluates the import.
type jsonnetFixture struct {
	parent      string
	repoRoot    string
	appDir      string
	outsideDir  string
	outsideFile string
}

// jsonnetTempDir creates a temporary directory in the working tree and returns
// its absolute path. We use this instead of t.TempDir() because OSX does weird
// things with temp directories: /var is a symlink to /private/var, so a prefix
// check against the temp path disagrees with filepath.EvalSymlinks.
// Same approach as tempRoot in commitserver/commit/hydratorhelper_test.go.
func jsonnetTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	})
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

func newJsonnetFixture(t *testing.T) *jsonnetFixture {
	t.Helper()
	parent := jsonnetTempDir(t)
	f := &jsonnetFixture{
		parent:      parent,
		repoRoot:    filepath.Join(parent, "repo"),
		appDir:      filepath.Join(parent, "repo", "app"),
		outsideDir:  filepath.Join(parent, "outside"),
		outsideFile: filepath.Join(parent, "outside", "secret.txt"),
	}
	require.NoError(t, os.MkdirAll(f.appDir, 0o755))
	require.NoError(t, os.MkdirAll(f.outsideDir, 0o755))
	require.NoError(t, os.WriteFile(f.outsideFile, []byte(outsideSentinel), 0o644))
	return f
}

func (f *jsonnetFixture) write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func (f *jsonnetFixture) relFromApp(t *testing.T, target string) string {
	t.Helper()
	rel, err := filepath.Rel(f.appDir, target)
	require.NoError(t, err)
	return filepath.ToSlash(rel)
}

// deepRelFromApp walks to the filesystem root with extra ".." segments, then
// names the absolute target. That is the long traversal reporters used.
func (f *jsonnetFixture) deepRelFromApp(target string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(target), "/")
	return strings.Repeat("../", 32) + trimmed
}

func jsonnetConfigMap(dataExpr string) string {
	return fmt.Sprintf(`{
  apiVersion: 'v1',
  kind: 'ConfigMap',
  metadata: { name: 'leak' },
  data: { leak: %s },
}
`, dataExpr)
}

func quoted(path string) string {
	return strconv.Quote(filepath.ToSlash(path))
}

func generateJsonnet(t *testing.T, appPath, repoRoot string, src v1alpha1.ApplicationSource) (*apiclient.ManifestResponse, error) {
	t.Helper()
	if src.Directory == nil {
		src.Directory = &v1alpha1.ApplicationSourceDirectory{}
	}
	if src.Path == "" {
		rel, err := filepath.Rel(repoRoot, appPath)
		require.NoError(t, err)
		src.Path = filepath.ToSlash(rel)
	}
	q := apiclient.ManifestRequest{
		Repo:               &v1alpha1.Repository{},
		ApplicationSource:  &src,
		ProjectName:        "jsonnet-import",
		ProjectSourceRepos: []string{"*"},
	}
	return GenerateManifests(t.Context(), appPath, repoRoot, "", &q, false, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
}

// assertImportDenied is the desired state for a read that must not succeed.
// On an unfixed importer the manifest contains the sentinel, and NotContains
// fails. A successful empty read (for example /dev/null) fails require.Error.
func assertImportDenied(t *testing.T, res *apiclient.ManifestResponse, err error, sentinel string) {
	t.Helper()
	if sentinel != "" {
		if res != nil {
			for _, manifest := range res.Manifests {
				assert.NotContains(t, manifest, sentinel)
			}
		}
		if err != nil {
			assert.NotContains(t, err.Error(), sentinel)
		}
	}
	require.Error(t, err)
}

// jsonnetImportBin reads a file with importbin and turns the byte array back
// into a string so a successful read shows the sentinel in the manifest.
func jsonnetImportBin(path string) string {
	return fmt.Sprintf(`std.join("", std.map(function(b) std.char(b), importbin %s))`, quoted(path))
}

func assertImportAllowed(t *testing.T, res *apiclient.ManifestResponse, err error, sentinel string) {
	t.Helper()
	require.NoError(t, err)
	require.NotNil(t, res)
	found := false
	for _, manifest := range res.Manifests {
		if strings.Contains(manifest, sentinel) {
			found = true
		}
	}
	assert.True(t, found, "expected in-repo import contents in the manifest")
}

func TestJsonnetImportDenial(t *testing.T) {
	t.Parallel()

	// GHSA-5cvw-w2m2-2389, GHSA-j3v9-m755-64vx, GHSA-f5x7-76gj-w9cg,
	// GHSA-9cjg-34v2-grxg, GHSA-9fjf-7jcx-7x4w, GHSA-g8q9-8cx4-vx88,
	// GHSA-chx4-wqx3-5p2c, GHSA-qfjc-g6x2-8955, GHSA-h723-gvm9-7276,
	// GHSA-m3vr-7329-44ww, GHSA-fvrq-p99v-rv6j, GHSA-m623-x735-qxhr.
	t.Run("importstr absolute path outside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.outsideFile)))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-j3v9-m755-64vx, GHSA-f5x7-76gj-w9cg, GHSA-9fjf-7jcx-7x4w, GHSA-m623-x735-qxhr.
	t.Run("importstr relative path leaves repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.relFromApp(t, f.outsideFile))))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-f5x7-76gj-w9cg: a long "../" chain that walks to the filesystem root.
	t.Run("importstr deep relative path leaves repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.deepRelFromApp(f.outsideFile))))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-m623-x735-qxhr: "../" into another checkout on the same disk.
	t.Run("importstr sibling checkout", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		sibling := filepath.Join(f.parent, "other-checkout", "secret.txt")
		f.write(t, sibling, outsideSentinel)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.relFromApp(t, sibling))))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-f5x7-76gj-w9cg, GHSA-9fjf-7jcx-7x4w, GHSA-h723-gvm9-7276 (import, not only importstr).
	t.Run("import outside jsonnet file", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		outsideJSON := filepath.Join(f.outsideDir, "leak.jsonnet")
		f.write(t, outsideJSON, "{ leak: '"+outsideSentinel+"' }\n")
		body := fmt.Sprintf(`local leak = import %s;
%s`, quoted(outsideJSON), jsonnetConfigMap("leak.leak"))
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), body)
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-5cvw-w2m2-2389, GHSA-g8q9-8cx4-vx88, GHSA-h723-gvm9-7276.
	t.Run("importbin outside file", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap(jsonnetImportBin(f.outsideFile)))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-j3v9-m755-64vx reported importstr of /dev/zero. This uses /dev/null so an
	// unfixed run returns an empty read instead of exhausting memory.
	t.Run("importstr non regular file", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap(`importstr '/dev/null'`))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		// /dev/null has no sentinel. An unfixed importer returns an empty string
		// and this fails because the import was accepted.
		assertImportDenied(t, res, err, "")
	})

	// No report used this PoC. GHSA-f5x7-76gj-w9cg notes that checkout symlink checks
	// do not apply to import resolution, so an in-repo symlink can still point outside.
	t.Run("symlink target outside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		link := filepath.Join(f.repoRoot, "leak-link")
		require.NoError(t, os.Symlink(f.outsideFile, link))
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.relFromApp(t, link))))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// No report. Same import-resolution gap as GHSA-f5x7-76gj-w9cg, when a path
	// component is a symlink rather than the imported name itself.
	t.Run("relative path through symlink outside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		linkDir := filepath.Join(f.repoRoot, "linked-dir")
		require.NoError(t, os.Symlink(f.outsideDir, linkDir))
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.relFromApp(t, linkDir)+"/secret.txt")))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// No report. An in-repo file imports another in-repo file that then reads outside.
	t.Run("nested import reads outside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "hop.libsonnet"), "{ leak: importstr "+quoted(f.relFromApp(t, f.outsideFile))+" }\n")
		body := `local hop = import 'hop.libsonnet';
` + jsonnetConfigMap("hop.leak")
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), body)
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-qfjc-g6x2-8955: importstr is supplied as a code:true top-level argument,
	// not written in the .jsonnet file.
	t.Run("tla code importstr", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), `function(leak)
`+jsonnetConfigMap("leak"))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{
			Directory: &v1alpha1.ApplicationSourceDirectory{
				Jsonnet: v1alpha1.ApplicationSourceJsonnet{
					TLAs: []v1alpha1.JsonnetVar{{
						Name:  "leak",
						Value: "importstr " + quoted(f.outsideFile),
						Code:  true,
					}},
				},
			},
		})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-qfjc-g6x2-8955: the same code:true TLA injected by .argocd-source.yaml,
	// which needs git write and no Argo CD account.
	t.Run("argocd source yaml tla code importstr", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), `function(leak)
`+jsonnetConfigMap("leak"))
		f.write(t, filepath.Join(f.appDir, ".argocd-source.yaml"), fmt.Sprintf(`directory:
  jsonnet:
    tlas:
      - name: leak
        value: importstr %s
        code: true
`, quoted(f.outsideFile)))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// No report used std.extVar. Same code-evaluation path as the TLA in GHSA-qfjc-g6x2-8955.
	// GHSA-26ch-q32p-mx3v's title says ExtVars; its writeup is importstr in the manifest source.
	t.Run("ext var code importstr", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("std.extVar('leak')"))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{
			Directory: &v1alpha1.ApplicationSourceDirectory{
				Jsonnet: v1alpha1.ApplicationSourceJsonnet{
					ExtVars: []v1alpha1.JsonnetVar{{
						Name:  "leak",
						Value: "importstr " + quoted(f.outsideFile),
						Code:  true,
					}},
				},
			},
		})
		assertImportDenied(t, res, err, outsideSentinel)
	})

	// GHSA-m3vr-7329-44ww, GHSA-fvrq-p99v-rv6j, GHSA-m623-x735-qxhr.
	// GenerateManifestWithFiles extracts the upload and calls GenerateManifests
	// with that directory as repoRoot. The sentinel lives in a sibling directory.
	t.Run("streamed archive sibling checkout", func(t *testing.T) {
		t.Parallel()
		parent := jsonnetTempDir(t)
		archive := filepath.Join(parent, "archive")
		sibling := filepath.Join(parent, "sibling", "secret.txt")
		require.NoError(t, os.MkdirAll(archive, 0o755))
		require.NoError(t, os.MkdirAll(filepath.Dir(sibling), 0o755))
		require.NoError(t, os.WriteFile(sibling, []byte(outsideSentinel), 0o644))
		rel, err := filepath.Rel(archive, sibling)
		require.NoError(t, err)
		body := jsonnetConfigMap("importstr " + quoted(filepath.ToSlash(rel)))
		require.NoError(t, os.WriteFile(filepath.Join(archive, "app.jsonnet"), []byte(body), 0o644))
		res, err := generateJsonnet(t, archive, archive, v1alpha1.ApplicationSource{Path: "."})
		assertImportDenied(t, res, err, outsideSentinel)
	})
}

// GHSA-26ch-q32p-mx3v reaches repo-server GenerateManifest directly. Directory-app
// reports (GHSA-j3v9-m755-64vx, GHSA-f5x7-76gj-w9cg, GHSA-9fjf-7jcx-7x4w, GHSA-chx4-wqx3-5p2c)
// use the same call after checkout.
func TestJsonnetImportDenialViaGenerateManifest(t *testing.T) {
	t.Parallel()
	f := newJsonnetFixture(t)
	f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.outsideFile)))

	service := newService(t, f.repoRoot)
	q := apiclient.ManifestRequest{
		Repo: &v1alpha1.Repository{},
		ApplicationSource: &v1alpha1.ApplicationSource{
			Path:      "app",
			Directory: &v1alpha1.ApplicationSourceDirectory{},
		},
		ProjectName:        "jsonnet-import",
		ProjectSourceRepos: []string{"*"},
	}
	res, err := service.GenerateManifest(t.Context(), &q)
	assertImportDenied(t, res, err, outsideSentinel)
}

func TestJsonnetImportAllowed(t *testing.T) {
	t.Parallel()

	// No report. A same-directory importstr must keep working.
	t.Run("importstr same directory", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "note.txt"), insideSentinel)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap(`importstr 'note.txt'`))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportAllowed(t, res, err, insideSentinel)
	})

	// No report. An import of a file in a subdirectory must keep working.
	t.Run("import subdirectory", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "sub", "lib.jsonnet"), "{ leak: '"+insideSentinel+"' }\n")
		body := `local lib = import 'sub/lib.jsonnet';
` + jsonnetConfigMap("lib.leak")
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), body)
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportAllowed(t, res, err, insideSentinel)
	})

	// No report of an in-repo library import. GHSA-f5x7-76gj-w9cg notes that configured
	// library paths are already confined, so an in-repo lib must still resolve.
	t.Run("import library path", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.repoRoot, "vendor", "params.libsonnet"), "{ leak: '"+insideSentinel+"' }\n")
		body := `local params = import 'params.libsonnet';
` + jsonnetConfigMap("params.leak")
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), body)
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{
			Directory: &v1alpha1.ApplicationSourceDirectory{
				Jsonnet: v1alpha1.ApplicationSourceJsonnet{
					Libs: []string{"vendor"},
				},
			},
		})
		assertImportAllowed(t, res, err, insideSentinel)
	})

	// No report of a permitted parent import. GHSA-f5x7-76gj-w9cg evaluates Jsonnet
	// from an app subdirectory, so "../" that stays inside the repo must still work.
	t.Run("relative parent stays inside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		note := filepath.Join(f.repoRoot, "lib", "note.txt")
		f.write(t, note, insideSentinel)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap("importstr "+quoted(f.relFromApp(t, note))))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportAllowed(t, res, err, insideSentinel)
	})

	// No report. A symlink whose target stays inside the repo must still be readable.
	t.Run("symlink target stays inside repo", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		note := filepath.Join(f.repoRoot, "lib", "note.txt")
		f.write(t, note, insideSentinel)
		link := filepath.Join(f.appDir, "note-link")
		require.NoError(t, os.Symlink(note, link))
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap(`importstr 'note-link'`))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		assertImportAllowed(t, res, err, insideSentinel)
	})

	// No report. A missing relative import is a not-found error, not a read of the outside file.
	t.Run("missing import is not found", func(t *testing.T) {
		t.Parallel()
		f := newJsonnetFixture(t)
		f.write(t, filepath.Join(f.appDir, "app.jsonnet"), jsonnetConfigMap(`importstr 'missing.txt'`))
		res, err := generateJsonnet(t, f.appDir, f.repoRoot, v1alpha1.ApplicationSource{})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), outsideSentinel)
		require.ErrorContains(t, err, "couldn't open import")
		if res != nil {
			for _, manifest := range res.Manifests {
				assert.NotContains(t, manifest, outsideSentinel)
			}
		}
	})
}
