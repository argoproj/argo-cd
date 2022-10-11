package image

import (
	"context"
	"fmt"
	imagepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/image"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
)

type server struct {
	keychain authn.Keychain
}

func (s *server) GetImage(ctx context.Context, req *imagepkg.GetImageRequest) (*imagepkg.GetImageResponse, error) {
	if os.Getenv("DISABLE_GET_IMAGE") == "true" {
		return nil, fmt.Errorf("GetImage is disabled")
	}
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
	links, err := links(req.Image)
	if err != nil {
		return nil, err
	}
	return &imagepkg.GetImageResponse{Image: image(f), XLinks: links}, nil
}

func NewServer(ctx context.Context, kubernetesClient kubernetes.Interface, namespace string) (imagepkg.ImageServiceServer, error) {
	var imagePullSecrets []string = nil
	v, ok := os.LookupEnv("IMAGE_PULL_SECRETS")
	if ok {
		imagePullSecrets = strings.Split(v, ",")
	}
	kc, err := k8schain.New(ctx, kubernetesClient, k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: os.Getenv("IMAGE_SERVICE_ACCOUNT_NAME"),
		ImagePullSecrets:   imagePullSecrets,
	})
	if err != nil {
		return nil, err
	}
	return &server{kc}, nil
}
