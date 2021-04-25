package db

import (
	"strconv"

	"github.com/argoproj/argo-cd/v2/common"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (db *db) listSecretsByType(types ...string) ([]*apiv1.Secret, error) {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, types)
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)

	secretsLister, err := db.settingsMgr.GetSecretsLister()
	if err != nil {
		return nil, err
	}
	secrets, err := secretsLister.Secrets(db.ns).List(labelSelector)
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

func boolOrDefault(s *apiv1.Secret, key string, def bool) (bool, error) {
	val, present := s.Data[key]
	if !present {
		return def, nil
	}

	return strconv.ParseBool(string(val))
}

func intOrDefault(s *apiv1.Secret, key string, def int64) (int64, error) {
	val, present := s.Data[key]
	if !present {
		return def, nil
	}

	return strconv.ParseInt(string(val), 10, 64)
}
