package argocd

import (
	"testing"
	"text/template"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/common"
	"github.com/argoproj/argo-cd/v2/image-updater/image"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"

	"sigs.k8s.io/kustomize/api/types"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_TemplateCommitMessage(t *testing.T) {
	t.Run("Template default commit message", func(t *testing.T) {
		exp := `build: automatic update of foobar

updates image foo/bar tag '1.0' to '1.1'
updates image bar/baz tag '2.0' to '2.1'
`
		tpl := template.Must(template.New("sometemplate").Parse(common.DefaultGitCommitMessage))
		cl := []ChangeEntry{
			{
				Image:  image.NewFromIdentifier("foo/bar"),
				OldTag: tag.NewImageTag("1.0", time.Now(), ""),
				NewTag: tag.NewImageTag("1.1", time.Now(), ""),
			},
			{
				Image:  image.NewFromIdentifier("bar/baz"),
				OldTag: tag.NewImageTag("2.0", time.Now(), ""),
				NewTag: tag.NewImageTag("2.1", time.Now(), ""),
			},
		}
		r := TemplateCommitMessage(tpl, "foobar", cl)
		assert.NotEmpty(t, r)
		assert.Equal(t, exp, r)
	})
}

func Test_TemplateBranchName(t *testing.T) {
	t.Run("Template branch name with image name", func(t *testing.T) {
		exp := `image-updater-foo/bar-1.1-bar/baz-2.1`
		tpl := "image-updater{{range .Images}}-{{.Name}}-{{.NewTag}}{{end}}"
		cl := []ChangeEntry{
			{
				Image:  image.NewFromIdentifier("foo/bar"),
				OldTag: tag.NewImageTag("1.0", time.Now(), ""),
				NewTag: tag.NewImageTag("1.1", time.Now(), ""),
			},
			{
				Image:  image.NewFromIdentifier("bar/baz"),
				OldTag: tag.NewImageTag("2.0", time.Now(), ""),
				NewTag: tag.NewImageTag("2.1", time.Now(), ""),
			},
		}
		r := TemplateBranchName(tpl, cl)
		assert.NotEmpty(t, r)
		assert.Equal(t, exp, r)
	})
	t.Run("Template branch name with alias", func(t *testing.T) {
		exp := `image-updater-bar-1.1`
		tpl := "image-updater{{range .Images}}-{{.Alias}}-{{.NewTag}}{{end}}"
		cl := []ChangeEntry{
			{
				Image:  image.NewFromIdentifier("bar=0001.dkr.ecr.us-east-1.amazonaws.com/bar"),
				OldTag: tag.NewImageTag("1.0", time.Now(), ""),
				NewTag: tag.NewImageTag("1.1", time.Now(), ""),
			},
		}
		r := TemplateBranchName(tpl, cl)
		assert.NotEmpty(t, r)
		assert.Equal(t, exp, r)
	})
	t.Run("Template branch name with hash", func(t *testing.T) {
		// Expected value generated from https://emn178.github.io/online-tools/sha256.html
		exp := `image-updater-0fcc2782543e4bb067c174c21bf44eb947f3e55c0d62c403e359c1c209cbd041`
		tpl := "image-updater-{{.SHA256}}"
		cl := []ChangeEntry{
			{
				Image:  image.NewFromIdentifier("foo/bar"),
				OldTag: tag.NewImageTag("1.0", time.Now(), ""),
				NewTag: tag.NewImageTag("1.1", time.Now(), ""),
			},
		}
		r := TemplateBranchName(tpl, cl)
		assert.NotEmpty(t, r)
		assert.Equal(t, exp, r)
	})
	t.Run("Template branch over 255 chars", func(t *testing.T) {
		tpl := "image-updater-lorem-ipsum-dolor-sit-amet-consectetur-" +
			"adipiscing-elit-phasellus-imperdiet-vitae-elit-quis-pulvinar-" +
			"suspendisse-pulvinar-lacus-vel-semper-congue-enim-purus-posuere-" +
			"orci-ut-vulputate-mi-ipsum-quis-ipsum-quisque-elit-arcu-lobortis-" +
			"in-blandit-vel-pharetra-vel-urna-aliquam-euismod-elit-vel-mi"
		exp := tpl[:255]
		cl := []ChangeEntry{}
		r := TemplateBranchName(tpl, cl)
		assert.NotEmpty(t, r)
		assert.Equal(t, exp, r)
		assert.Len(t, r, 255)
	})
}

func Test_parseImageOverride(t *testing.T) {
	cases := []struct {
		name     string
		override v1alpha1.KustomizeImage
		expected types.Image
	}{
		{"tag update", "ghcr.io:1234/foo/foo:123", types.Image{
			Name:   "ghcr.io:1234/foo/foo",
			NewTag: "123",
		}},
		{"image update", "ghcr.io:1234/foo/foo=ghcr.io:1234/bar", types.Image{
			Name:    "ghcr.io:1234/foo/foo",
			NewName: "ghcr.io:1234/bar",
		}},
		{"update everything", "ghcr.io:1234/foo/foo=1234.foo.com:9876/bar:123", types.Image{
			Name:    "ghcr.io:1234/foo/foo",
			NewName: "1234.foo.com:9876/bar",
			NewTag:  "123",
		}},
		{"change registry and tag", "ghcr.io:1234/foo/foo=1234.dkr.ecr.us-east-1.amazonaws.com/bar:123", types.Image{
			Name:    "ghcr.io:1234/foo/foo",
			NewName: "1234.dkr.ecr.us-east-1.amazonaws.com/bar",
			NewTag:  "123",
		}},
		{"change only registry", "0001.dkr.ecr.us-east-1.amazonaws.com/bar=1234.dkr.ecr.us-east-1.amazonaws.com/bar", types.Image{
			Name:    "0001.dkr.ecr.us-east-1.amazonaws.com/bar",
			NewName: "1234.dkr.ecr.us-east-1.amazonaws.com/bar",
		}},
		{"change image and set digest", "foo=acme/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", types.Image{
			Name:    "foo",
			NewName: "acme/app",
			Digest:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}},
		{"set digest", "acme/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", types.Image{
			Name:   "acme/app",
			Digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseImageOverride(tt.override))
		})
	}

}

func Test_imagesFilter(t *testing.T) {
	for _, tt := range []struct {
		name     string
		images   v1alpha1.KustomizeImages
		expected string
	}{
		{name: "simple", images: v1alpha1.KustomizeImages{"foo"}, expected: `
images:
- name: foo
`},
		{name: "tagged", images: v1alpha1.KustomizeImages{"foo:bar"}, expected: `
images:
- name: foo
  newTag: bar
`},
		{name: "rename", images: v1alpha1.KustomizeImages{"baz=foo:bar"}, expected: `
images:
- name: baz
  newName: foo
  newTag: bar
`},
		{name: "digest", images: v1alpha1.KustomizeImages{"baz=foo@sha12345"}, expected: `
images:
- name: baz
  newName: foo
  digest: sha12345
`},
		{name: "digest simple", images: v1alpha1.KustomizeImages{"foo@sha12345"}, expected: `
images:
- name: foo
  digest: sha12345
`},
		{name: "all", images: v1alpha1.KustomizeImages{
			"foo",
			"foo=bar", // merges with above
			"baz@sha12345",
			"bar:123",
			"foo=bar:123", // merges and overwrites the first two
		}, expected: `
images:
- name: foo
  newName: bar
  newTag: "123"
- name: baz
  digest: sha12345
- name: bar
  newTag: "123"
`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := imagesFilter(tt.images)
			assert.NoError(t, err)

			node := kyaml.NewRNode(&kyaml.Node{Kind: kyaml.DocumentNode, Content: []*kyaml.Node{
				kyaml.NewMapRNode(nil).YNode(),
			}})
			node, err = filter.Filter(node)
			assert.NoError(t, err)
			assert.YAMLEq(t, tt.expected, node.MustString())
		})
	}
}
