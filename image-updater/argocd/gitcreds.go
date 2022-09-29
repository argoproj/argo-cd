package argocd

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"

	"github.com/argoproj/argo-cd/v2/image-updater/kube"
	"github.com/argoproj/argo-cd/v2/util/git"
)

// getGitCredsSource returns git credentials source that loads credentials from the secret or from Argo CD settings
func getGitCredsSource(creds string, kubeClient *kube.KubernetesClient, gitCredStore git.CredsStore) (GitCredsSource, error) {
	switch {
	case creds == "repocreds":
		return func(app *v1alpha1.Application) (git.Creds, error) {
			return getCredsFromArgoCD(app, kubeClient, gitCredStore)
		}, nil
	case strings.HasPrefix(creds, "secret:"):
		return func(app *v1alpha1.Application) (git.Creds, error) {
			return getCredsFromSecret(app, creds[len("secret:"):], kubeClient, gitCredStore)
		}, nil
	}
	return nil, fmt.Errorf("unexpected credentials format. Expected 'repocreds' or 'secret:<namespace>/<secret>' but got '%s'", creds)
}

// getCredsFromArgoCD loads repository credentials from Argo CD settings
func getCredsFromArgoCD(app *v1alpha1.Application, kubeClient *kube.KubernetesClient, gitCredStore git.CredsStore) (git.Creds, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	settingsMgr := settings.NewSettingsManager(ctx, kubeClient.Clientset, kubeClient.Namespace)
	argocdDB := db.NewDB(kubeClient.Namespace, settingsMgr, kubeClient.Clientset)
	repo, err := argocdDB.GetRepository(ctx, app.Spec.Source.RepoURL)
	if err != nil {
		return nil, err
	}
	if !repo.HasCredentials() {
		return nil, fmt.Errorf("credentials for '%s' are not configured in Argo CD settings", app.Spec.Source.RepoURL)
	}
	return repo.GetGitCreds(gitCredStore), nil
}

// getCredsFromSecret loads repository credentials from secret
func getCredsFromSecret(app *v1alpha1.Application, credentialsSecret string, kubeClient *kube.KubernetesClient, gitCredStore git.CredsStore) (git.Creds, error) {
	var credentials map[string][]byte
	var err error
	s := strings.SplitN(credentialsSecret, "/", 2)
	if len(s) == 2 {
		credentials, err = kubeClient.GetSecretData(s[0], s[1])
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("secret ref must be in format 'namespace/name', but is '%s'", credentialsSecret)
	}

	if ok, _ := git.IsSSHURL(app.Spec.Source.RepoURL); ok {
		var sshPrivateKey []byte
		if sshPrivateKey, ok = credentials["sshPrivateKey"]; !ok {
			return nil, fmt.Errorf("invalid secret %s: does not contain field sshPrivateKey", credentialsSecret)
		}
		return git.NewSSHCreds(string(sshPrivateKey), "", true, gitCredStore), nil
	} else if git.IsHTTPSURL(app.Spec.Source.RepoURL) {
		var username, password []byte
		if username, ok = credentials["username"]; !ok {
			return nil, fmt.Errorf("invalid secret %s: does not contain field username", credentialsSecret)
		}
		if password, ok = credentials["password"]; !ok {
			return nil, fmt.Errorf("invalid secret %s: does not contain field password", credentialsSecret)
		}

		gitCredStore.Add(string(username[:]), string(password[:]))
		return git.NewHTTPSCreds(string(username), string(password), "", "", true, "", gitCredStore), nil
	}
	return nil, fmt.Errorf("unknown repository type")
}
