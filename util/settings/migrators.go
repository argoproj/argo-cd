package settings

import (
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/argoproj/argo-cd/common"
)

func (mgr *SettingsManager) MigrateLegacySettings(settings *ArgoCDSettings) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	err = mgr.migrate001(settings)
	if err != nil {
		return err
	}

	mgr.migrate002(settings)
	mgr.migrate003(settings)

	return nil
}

// Migrates legacy (v0.10 and below) repo secrets into the v0.11 configmap
func (mgr *SettingsManager) migrate001(settings *ArgoCDSettings) error {

	if len(settings.Repositories) > 0 {
		return nil
	}

	log.Infof("Migrating repositories")

	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{"repository"})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*req)
	repoSecrets, err := mgr.secrets.Secrets(mgr.namespace).List(labelSelector)
	if err != nil {
		return err
	}
	settings.Repositories = make([]RepoCredentials, len(repoSecrets))
	for i, s := range repoSecrets {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(s)
		if err != nil {
			return err
		}
		cred := RepoCredentials{URL: string(s.Data["repository"])}
		if username, ok := s.Data["username"]; ok && string(username) != "" {
			cred.UsernameSecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "username",
			}
		}
		if password, ok := s.Data["password"]; ok && string(password) != "" {
			cred.PasswordSecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "password",
			}
		}
		if sshPrivateKey, ok := s.Data["sshPrivateKey"]; ok && string(sshPrivateKey) != "" {
			cred.SshPrivateKeySecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "sshPrivateKey",
			}
		}
		settings.Repositories[i] = cred
	}
	return nil
}

func (mgr *SettingsManager) migrate002(settings *ArgoCDSettings) {

	if settings.HelmRepositories == nil {
		return
	}

	for i, repo := range settings.HelmRepositories {
		log.Infof("Setting repository %s's type to %s", repo.URL, Helm)
		settings.HelmRepositories[i].Type = Helm
	}

	log.Infof("Migrating Helm repositories")
	settings.Repositories = append(settings.Repositories, settings.HelmRepositories...)
	settings.HelmRepositories = nil
}

func (mgr *SettingsManager) migrate003(settings *ArgoCDSettings) {

	for i, repo := range settings.Repositories {
		if repo.Type == "" {
			log.Infof("Setting repository %s's type to %s", repo.URL, Git)
			settings.Repositories[i].Type = Git
		}
	}
}
