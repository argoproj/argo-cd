package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func TestAuthReconcileWithMissingNamespace(t *testing.T) {
	namespace := "test-ns"
	fakeBearer := "fake-bearer"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := &metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("namespace \"%s\" not found", namespace),
			Reason:  metav1.StatusReasonNotFound,
			Code:    http.StatusNotFound,
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	kubeConfigFlags.Namespace = &namespace
	kubeConfigFlags.APIServer = &server.URL
	kubeConfigFlags.BearerToken = &fakeBearer
	matchFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	fact := cmdutil.NewFactory(matchFlags)

	config := &rest.Config{Host: server.URL}
	k := &kubectlResourceOperations{
		config: config,
		fact:   fact,
	}

	role := testingutils.NewRole()
	role.SetNamespace(namespace)

	_, err := k.authReconcile(context.Background(), role, "/dev/null", cmdutil.DryRunNone)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "returned error wasn't not found")

	roleBinding := testingutils.NewRoleBinding()
	roleBinding.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), roleBinding, "/dev/null", cmdutil.DryRunNone)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "returned error wasn't not found")

	clusterRole := testingutils.NewClusterRole()
	clusterRole.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), clusterRole, "/dev/null", cmdutil.DryRunNone)
	assert.NoError(t, err)

	clusterRoleBinding := testingutils.NewClusterRoleBinding()
	clusterRoleBinding.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), clusterRoleBinding, "/dev/null", cmdutil.DryRunNone)
	assert.NoError(t, err)
}
