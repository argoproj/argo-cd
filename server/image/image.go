package image

import (
	"context"
	imagepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/image"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type server struct {
	keychain authn.Keychain
}

func (s *server) GetImage(ctx context.Context, req *imagepkg.GetImageRequest) (*imagepkg.GetImageResponse, error) {
	log.WithField("image", req.Image).Info("GetImage")
	ref, err := name.ParseReference(req.Image)
	if err != nil {
		return nil, err
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(s.keychain))
	if err != nil {
		return nil, err
	}
	f, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	return &imagepkg.GetImageResponse{
		Image: s.image(f),
	}, nil
}

func NewServer(ctx context.Context, kubernetesClient kubernetes.Interface, namespace string, serviceAccountName string, imagePullSecrets []string) (imagepkg.ImageServiceServer, error) {
	kc, err := k8schain.New(ctx, kubernetesClient, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccountName,
		ImagePullSecrets:   imagePullSecrets,
	})
	if err != nil {
		return nil, err
	}
	return &server{kc}, nil
}

func (s *server) image(f *v1.ConfigFile) *imagepkg.Image {
	return &imagepkg.Image{
		Created: &metav1.Time{
			Time: f.Created.Time,
		},
		Author: f.Author,
		Config: config(f.Config),
	}
}

func config(c v1.Config) *imagepkg.Config {
	return &imagepkg.Config{
		Command:      c.Cmd,
		Entrypoint:   c.Entrypoint,
		Env:          c.Env,
		WorkingDir:   c.WorkingDir,
		ExposedPorts: exposedPorts(c.ExposedPorts),
		Labels:       c.Labels,
	}
}

func exposedPorts(ports map[string]struct{}) map[string]*imagepkg.Port {
	out := make(map[string]*imagepkg.Port)
	for key := range ports {
		out[key] = &imagepkg.Port{}
	}
	return out
}
