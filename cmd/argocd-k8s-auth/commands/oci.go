package commands

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/spf13/cobra"

	argoerrors "github.com/argoproj/argo-cd/v3/util/errors"
)

const ociTokenExpiration = 4 * time.Minute

// newOCICommand returns a new instance of an oci command that generates a k8s auth token
// for Oracle Kubernetes Engine (OKE) using OKE Workload Identity.
func newOCICommand() *cobra.Command {
	var (
		clusterID string
		region    string
	)
	command := &cobra.Command{
		Use: "oci",
		Run: func(c *cobra.Command, _ []string) {
			ctx := c.Context()

			presignedURLString, err := getSignedOCIRequestWithRetry(ctx, time.Minute, 5*time.Second, clusterID, region, getSignedOCIRequest)
			argoerrors.CheckError(err)
			token := base64.URLEncoding.EncodeToString([]byte(presignedURLString))
			tokenExpiration := time.Now().UTC().Add(ociTokenExpiration)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(token, tokenExpiration))
		},
	}
	command.Flags().StringVar(&clusterID, "cluster-id", "", "OCI OKE Cluster OCID")
	command.Flags().StringVar(&region, "region", "", "OCI Region")
	return command
}

type getSignedOCIRequestFunc func(ctx context.Context, clusterID, region string) (string, error)

// getSignedOCIRequestWithRetry retries generating a signed OCI cluster_request URL until it succeeds or the timeout expires.
func getSignedOCIRequestWithRetry(ctx context.Context, timeout, interval time.Duration, clusterID, region string, fn getSignedOCIRequestFunc) (string, error) {
	if clusterID == "" {
		return "", errors.New("cluster ID must not be empty")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		signed, err := fn(ctx, clusterID, region)
		if err == nil {
			return signed, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout while trying to get signed oci request: last error: %w", err)
		case <-time.After(interval):
		}
	}
}

// getSignedOCIRequest initializes an OKE Workload Identity provider and signs the cluster_request URL.
func getSignedOCIRequest(ctx context.Context, clusterID, region string) (string, error) {
	if _, ok := os.LookupEnv(auth.ResourcePrincipalVersionEnvVar); !ok {
		_ = os.Setenv(auth.ResourcePrincipalVersionEnvVar, auth.ResourcePrincipalVersion2_2)
	}
	if region != "" {
		if _, ok := os.LookupEnv(auth.ResourcePrincipalRegionEnvVar); !ok {
			_ = os.Setenv(auth.ResourcePrincipalRegionEnvVar, region)
		}
	}
	provider, err := auth.OkeWorkloadIdentityConfigurationProvider()
	if err != nil {
		return "", fmt.Errorf("error loading OCI OKE workload identity provider: %w", err)
	}
	if region == "" {
		region, err = provider.Region()
		if err != nil {
			return "", fmt.Errorf("error resolving OCI region: %w", err)
		}
	}
	return getSignedOCIRequestWithProvider(ctx, clusterID, region, provider)
}

// getSignedOCIRequestWithProvider signs a GET request to the regional OKE cluster_request endpoint using the provided OCI KeyProvider.
func getSignedOCIRequestWithProvider(ctx context.Context, clusterID, region string, provider common.KeyProvider) (string, error) {
	endpoint := fmt.Sprintf("https://containerengine.%s.oraclecloud.com/cluster_request/%s", region, clusterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("error creating OCI cluster request: %w", err)
	}
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))

	signer := common.DefaultRequestSigner(provider)
	if err := signer.Sign(req); err != nil {
		return "", fmt.Errorf("error signing OCI cluster request: %w", err)
	}

	query := url.Values{}
	query.Set("authorization", req.Header.Get("Authorization"))
	query.Set("date", req.Header.Get("Date"))
	req.URL.RawQuery = query.Encode()

	return req.URL.String(), nil
}
