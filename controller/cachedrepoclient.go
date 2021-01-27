package controller

import (
	"context"
	"time"

	"github.com/argoproj/argo-cd/util/git"

	"github.com/argoproj/argo-cd/reposerver/apiclient"

	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	reposervercache "github.com/argoproj/argo-cd/reposerver/cache"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/io"
)

const expiration = time.Hour

func NewInMemoryCachedRepoServerClientSet(clientSet apiclient.Clientset) *cachedClientSet {
	return &cachedClientSet{wrappedClientSet: clientSet, cache: reposervercache.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(expiration)), expiration)}
}

type cachedClientSet struct {
	wrappedClientSet apiclient.Clientset
	cache            *reposervercache.Cache
}

func (cs *cachedClientSet) NewRepoServerClient() (io.Closer, apiclient.RepoServerServiceClient, error) {
	closer, client, err := cs.wrappedClientSet.NewRepoServerClient()
	if err == nil {
		client = &cacheClient{cache: cs.cache, wrappedClient: client}
	}
	return closer, client, err
}

type cacheClient struct {
	wrappedClient apiclient.RepoServerServiceClient
	cache         *reposervercache.Cache
}

func (c *cacheClient) GenerateManifest(ctx context.Context, in *apiclient.ManifestRequest, opts ...grpc.CallOption) (*apiclient.ManifestResponse, error) {
	var cached reposervercache.CachedManifestResponse
	canUseCached := git.IsCommitSHA(in.Revision) && git.IsTruncatedCommitSHA(in.Revision) && !in.NoCache
	if canUseCached && c.cache.GetManifests(in.Revision, in.ApplicationSource, in.Namespace, in.AppLabelKey, in.AppName, &cached) == nil {
		return cached.ManifestResponse, nil
	}
	res, err := c.wrappedClient.GenerateManifest(ctx, in, opts...)
	if err == nil {
		_ = c.cache.SetManifests(in.Revision, in.ApplicationSource, in.Namespace, in.AppLabelKey, in.AppName, &reposervercache.CachedManifestResponse{ManifestResponse: res})
	}
	return res, err
}

func (c *cacheClient) ListRefs(ctx context.Context, in *apiclient.ListRefsRequest, opts ...grpc.CallOption) (*apiclient.Refs, error) {
	return c.wrappedClient.ListRefs(ctx, in, opts...)
}

func (c *cacheClient) ListApps(ctx context.Context, in *apiclient.ListAppsRequest, opts ...grpc.CallOption) (*apiclient.AppList, error) {
	return c.wrappedClient.ListApps(ctx, in, opts...)
}

func (c *cacheClient) GetAppDetails(ctx context.Context, in *apiclient.RepoServerAppDetailsQuery, opts ...grpc.CallOption) (*apiclient.RepoAppDetailsResponse, error) {
	return c.wrappedClient.GetAppDetails(ctx, in, opts...)
}

func (c *cacheClient) GetRevisionMetadata(ctx context.Context, in *apiclient.RepoServerRevisionMetadataRequest, opts ...grpc.CallOption) (*v1alpha1.RevisionMetadata, error) {
	return c.wrappedClient.GetRevisionMetadata(ctx, in, opts...)
}

func (c *cacheClient) GetHelmCharts(ctx context.Context, in *apiclient.HelmChartsRequest, opts ...grpc.CallOption) (*apiclient.HelmChartsResponse, error) {
	return c.wrappedClient.GetHelmCharts(ctx, in, opts...)
}
