package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/opencontainers/go-digest"
	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/errdef"
)

// ErrTagNotFound is returned (wrapped) by Pusher.ResolveTag when the tag does not exist in the repository.
var ErrTagNotFound = errors.New("tag not found")

// PushOptions configures an artifact push.
type PushOptions struct {
	// ArtifactType is the OCI artifactType set on the manifest. Required.
	ArtifactType string
	// Annotations are set on the OCI manifest.
	Annotations map[string]string
}

// Pusher publishes artifacts to a single OCI repository.
type Pusher interface {
	// ResolveTag returns the manifest digest (e.g. "sha256:...") and the manifest annotations of the artifact the tag
	// currently points to. If the tag does not exist the returned error satisfies errors.Is(err, ErrTagNotFound).
	ResolveTag(ctx context.Context, tag string) (digest string, annotations map[string]string, err error)
	// PushDirectory packs the contents of dir into a single gzipped tar layer, pushes it with an OCI image manifest,
	// points tag at that manifest (creating the tag or moving it), and returns the manifest digest.
	//
	// dir may only contain regular files and directories; anything else (e.g. a symlink) is an error.
	//
	// The layer is reproducible: the same directory content always yields the same layer digest, regardless of file
	// timestamps, ownership or permissions. This holds for a given Argo CD build; the compressed bytes may differ
	// between Go versions. The manifest digest is only reproducible if the caller also sets the
	// "org.opencontainers.image.created" annotation (an RFC 3339 timestamp) in opts.Annotations. If it is not set,
	// the current time is used, so every push yields a new digest.
	//
	// The tag is moved last, so a failed push never leaves the tag pointing at an incomplete artifact.
	PushDirectory(ctx context.Context, dir string, tag string, opts PushOptions) (digest string, err error)
}

// NewPusher returns a Pusher for the given oci:// repository URL, using the same TLS, proxy and credential handling as NewClient.
func NewPusher(repoURL string, creds Creds, proxy, noProxy string) (Pusher, error) {
	repo, err := newRemoteRepository(repoURL, creds, proxy, noProxy)
	if err != nil {
		return nil, err
	}
	return NewPusherForTarget(repo), nil
}

// NewPusherForTarget returns a Pusher that writes to the given oras target. It exists so callers can test against an in-memory store.
func NewPusherForTarget(target oras.Target) Pusher {
	return &pusher{target: target}
}

type pusher struct {
	target oras.Target
}

func (p *pusher) ResolveTag(ctx context.Context, tag string) (string, map[string]string, error) {
	if tag == "" {
		return "", nil, errors.New("tag must not be empty")
	}

	desc, err := p.target.Resolve(ctx, tag)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return "", nil, fmt.Errorf("failed to resolve tag %q: %w", tag, ErrTagNotFound)
		}
		return "", nil, fmt.Errorf("failed to resolve tag %q: %w", tag, err)
	}

	manifest, err := getOCIManifest(ctx, desc.Digest.String(), resolvedTarget{ReadOnlyTarget: p.target, desc: desc})
	if err != nil {
		return "", nil, fmt.Errorf("failed to read manifest of tag %q: %w", tag, err)
	}

	return desc.Digest.String(), manifest.Annotations, nil
}

func (p *pusher) PushDirectory(ctx context.Context, dir string, tag string, opts PushOptions) (string, error) {
	if tag == "" {
		return "", errors.New("tag must not be empty")
	}
	if opts.ArtifactType == "" {
		return "", errors.New("artifact type must not be empty")
	}

	layer, uncompressedDigest, err := packDirectory(dir)
	if err != nil {
		return "", fmt.Errorf("failed to pack directory %q: %w", dir, err)
	}
	layerDesc := content.NewDescriptorFromBytes(imagev1.MediaTypeImageLayerGzip, layer)
	// The same annotations `oras push <ref> .` sets on a directory layer, so that `oras pull` unpacks the artifact
	// into the working directory.
	layerDesc.Annotations = map[string]string{
		imagev1.AnnotationTitle: ".",
		file.AnnotationUnpack:   "true",
		file.AnnotationDigest:   uncompressedDigest.String(),
	}

	// Unchanged content is the common case when re-hydrating, so avoid uploading a layer the repository already has.
	exists, err := p.target.Exists(ctx, layerDesc)
	if err != nil {
		return "", fmt.Errorf("failed to check for existing layer %s: %w", layerDesc.Digest, err)
	}
	if !exists {
		if err := p.target.Push(ctx, layerDesc, bytes.NewReader(layer)); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
			return "", fmt.Errorf("failed to push layer %s: %w", layerDesc.Digest, err)
		}
	}

	manifestDesc, err := oras.PackManifest(ctx, p.target, oras.PackManifestVersion1_1, opts.ArtifactType, oras.PackManifestOptions{
		Layers:              []imagev1.Descriptor{layerDesc},
		ManifestAnnotations: opts.Annotations,
	})
	if err != nil {
		return "", fmt.Errorf("failed to push manifest: %w", err)
	}

	if err := p.target.Tag(ctx, manifestDesc, tag); err != nil {
		return "", fmt.Errorf("failed to tag manifest %s as %q: %w", manifestDesc.Digest, tag, err)
	}

	return manifestDesc.Digest.String(), nil
}

// resolvedTarget resolves every reference to an already resolved descriptor. This lets ResolveTag reuse getOCIManifest
// without resolving the tag a second time, so the returned annotations always belong to the returned digest even if
// the tag is moved concurrently. It also avoids resolving by digest, which in-memory stores do not support.
type resolvedTarget struct {
	oras.ReadOnlyTarget
	desc imagev1.Descriptor
}

func (t resolvedTarget) Resolve(context.Context, string) (imagev1.Descriptor, error) {
	return t.desc, nil
}

// packDirectory returns the contents of dir as a gzipped tar archive with entry names relative to dir, and the digest
// of the uncompressed archive. The output depends only on the names and contents of the files: entries are written in
// lexical order with fixed modes and no timestamps or ownership, so that packing the same content twice yields the
// same bytes.
func packDirectory(dir string) ([]byte, digest.Digest, error) {
	var buf bytes.Buffer
	// A gzip.Writer with an untouched Header writes neither a file name nor a modification time.
	gzw := gzip.NewWriter(&buf)
	uncompressed := digest.Canonical.Digester()
	tw := tar.NewWriter(io.MultiWriter(gzw, uncompressed.Hash()))

	// filepath.WalkDir visits entries in lexical order and does not follow symlinks.
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			if !d.IsDir() {
				return errors.New("not a directory")
			}
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)

		switch {
		case d.IsDir():
			err := tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     name + "/",
				Mode:     0o755,
				ModTime:  time.Unix(0, 0),
			})
			if err != nil {
				return fmt.Errorf("failed to write tar header for %q: %w", name, err)
			}
			return nil
		case d.Type().IsRegular():
			return writeTarFile(tw, path, name)
		default:
			return fmt.Errorf("unsupported file type %s for %q: only regular files and directories are supported", d.Type(), name)
		}
	})
	if err != nil {
		return nil, "", err
	}

	if err := tw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finish tar archive: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finish gzip stream: %w", err)
	}

	return buf.Bytes(), uncompressed.Digest(), nil
}

// writeTarFile adds the regular file at path to tw under the given name.
func writeTarFile(tw *tar.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     0o644,
		Size:     info.Size(),
		ModTime:  time.Unix(0, 0),
	})
	if err != nil {
		return fmt.Errorf("failed to write tar header for %q: %w", name, err)
	}

	// The tar writer only accepts exactly info.Size() bytes for this entry, so a file that changes size while it is
	// being packed fails the push instead of producing a corrupt archive.
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("failed to write %q to tar archive: %w", name, err)
	}
	return nil
}
