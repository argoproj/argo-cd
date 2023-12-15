package db

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"sort"
	"strconv"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// CreateApplicationRevisionHistory creates a revisionHistory for application
func (db *db) CreateApplicationRevisionHistory(ctx context.Context, app *v1alpha1.Application,
	history *v1alpha1.ApplicationRevisionHistory) error {
	secName := ApplicationRevisionHistoryToSecretName("application-history",
		history.ApplicationName, history.ApplicationUID, history.HistoryID)

	historySecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType:                        common.LabelValueApplicationRevisionHistory,
				common.LabelKeyApplicationRevisionHistoryAppName: app.Name,
				common.LabelKeyApplicationRevisionHistoryAppUID:  string(app.UID),
				common.LabelKeyApplicationRevisionHistoryID:      fmt.Sprintf("%d", history.HistoryID),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "Application",
					Name:       app.Name,
					UID:        app.UID,
				},
			},
		},
	}
	if err := applicationRevisionHistoryToSecret(history, historySecret); err != nil {
		return err
	}
	if _, err := db.createSecret(ctx, historySecret); err != nil {
		if apierr.IsAlreadyExists(err) {
			return status.Errorf(codes.AlreadyExists, "application revision history '%s/%s/%d' already exists",
				history.ApplicationName, history.ApplicationUID, history.HistoryID)
		}
	}
	return db.settingsMgr.ResyncInformers()
}

// DeleteApplicationRevisionHistory delete the revisionHistory by application and historyID
func (db *db) DeleteApplicationRevisionHistory(ctx context.Context, appName, appUID string, historyID int64) error {
	secret, err := db.getAppRevisionHistorySecret(appName, appUID, historyID)
	if err != nil {
		return err
	}
	if err = db.deleteSecret(ctx, secret); err != nil {
		return err
	}
	return db.settingsMgr.ResyncInformers()
}

// GetApplicationRevisionHistory get the revisionHistory by application and historyID
func (db *db) GetApplicationRevisionHistory(_ context.Context, appName, appUID string,
	historyID int64) (*v1alpha1.ApplicationRevisionHistory, error) {
	secret, err := db.getAppRevisionHistorySecret(appName, appUID, historyID)
	if err != nil {
		return nil, err
	}
	history, err := secretToApplicationRevisionHistory(secret)
	if err != nil {
		return nil, err
	}
	return history, nil
}

type revisionHistorySort []*v1alpha1.ApplicationRevisionHistory

func (s revisionHistorySort) Len() int {
	return len(s)
}

func (s revisionHistorySort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s revisionHistorySort) Less(i, j int) bool {
	return s[i].HistoryID > s[j].HistoryID
}

// ListApplicationRevisionHistories list revisionHistories by application
func (db *db) ListApplicationRevisionHistories(_ context.Context, appName,
	appUID string) ([]*v1alpha1.ApplicationRevisionHistory, error) {
	secrets, err := db.listAppRevisionHistories(appName, appUID)
	if err != nil {
		return nil, err
	}
	result := make([]*v1alpha1.ApplicationRevisionHistory, 0, len(secrets))
	for _, secret := range secrets {
		history, err := secretToApplicationRevisionHistory(secret)
		if err != nil {
			return nil, err
		}
		result = append(result, history)
	}
	bs, _ := json.Marshal(result)
	fmt.Printf("secrets >>>> %s\n", string(bs))
	sort.Sort(revisionHistorySort(result))
	return result, nil
}

func (db *db) listAppRevisionHistories(appName, appUID string) ([]*apiv1.Secret, error) {
	labelSelector, err := buildApplicationRevisionHistoryLabelSelector(map[string][]string{
		common.LabelKeySecretType:                        {common.LabelValueApplicationRevisionHistory},
		common.LabelKeyApplicationRevisionHistoryAppName: {appName},
		common.LabelKeyApplicationRevisionHistoryAppUID:  {appUID},
	})
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

func (db *db) getAppRevisionHistorySecret(appName, appUID string, historyID int64) (*apiv1.Secret, error) {
	labelSelector, err := buildApplicationRevisionHistoryLabelSelector(map[string][]string{
		common.LabelKeySecretType:                        {common.LabelValueApplicationRevisionHistory},
		common.LabelKeyApplicationRevisionHistoryAppName: {appName},
		common.LabelKeyApplicationRevisionHistoryAppUID:  {appUID},
		common.LabelKeyApplicationRevisionHistoryID:      {fmt.Sprintf("%d", historyID)},
	})
	secretsLister, err := db.settingsMgr.GetSecretsLister()
	if err != nil {
		return nil, err
	}
	secrets, err := secretsLister.Secrets(db.ns).List(labelSelector)
	if err != nil {
		return nil, err
	}
	if len(secrets) == 0 {
		return nil, status.Errorf(codes.NotFound, "application revision history '%s/%s/%d' not found",
			appName, appUID, historyID)
	}
	return secrets[0], nil
}

func buildApplicationRevisionHistoryLabelSelector(selectors map[string][]string) (labels.Selector, error) {
	labelSelector := labels.NewSelector()
	for key, values := range selectors {
		req, err := labels.NewRequirement(key, selection.Equals, values)
		if err != nil {
			return nil, err
		}
		labelSelector = labelSelector.Add(*req)
	}
	return labelSelector, nil
}

func applicationRevisionHistoryToSecret(history *v1alpha1.ApplicationRevisionHistory, secret *apiv1.Secret) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	updateSecretString(secret, "applicationName", history.ApplicationName)
	updateSecretString(secret, "applicationUID", history.ApplicationUID)
	updateSecretString(secret, "project", history.Project)
	updateSecretString(secret, "historyID", fmt.Sprintf("%d", history.HistoryID))
	manifests, err := json.Marshal(history.ManagedResources)
	if err != nil {
		return fmt.Errorf("marshal managed resources failed: %s", err.Error())
	}
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err = gzipWriter.Write(manifests); err != nil {
		return fmt.Errorf("gzip writer manfiests failed: %s", err.Error())
	}
	gzipWriter.Close()
	secret.Data["managedResources"] = compressed.Bytes()
	return nil
}

func secretToApplicationRevisionHistory(secret *apiv1.Secret) (*v1alpha1.ApplicationRevisionHistory, error) {
	historyIDStr := string(secret.Data["historyID"])
	historyID, err := strconv.ParseInt(historyIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse history id '%s' failed: %s", historyIDStr, err.Error())
	}
	manifests := secret.Data["managedResources"]
	gzipReader, err := gzip.NewReader(bytes.NewReader(manifests))
	if err != nil {
		return nil, fmt.Errorf("create gzip reader failed: %s", err.Error())
	}
	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("read gzip reader failed: %s", err.Error())
	}
	gzipReader.Close()
	managedResources := make([]string, 0)
	if err = json.Unmarshal(decompressed, &managedResources); err != nil {
		return nil, fmt.Errorf("unmarshal decompressed managed resources failed: %s", err.Error())
	}
	history := &v1alpha1.ApplicationRevisionHistory{
		ApplicationName:  string(secret.Data["applicationName"]),
		ApplicationUID:   string(secret.Data["applicationUID"]),
		Project:          string(secret.Data["project"]),
		HistoryID:        historyID,
		ManagedResources: managedResources,
	}
	return history, nil
}

// ApplicationRevisionHistoryToSecretName hashes application-name/application-uid/history-id to a secret name using
// a formula. This secret name not in stable, and has no practical significance, don't rely on it.
func ApplicationRevisionHistoryToSecretName(prefix string, appName, appUID string, historyID int64) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s/%s/%d", appName, appUID, historyID)))
	return fmt.Sprintf("%s-%v", prefix, h.Sum32())
}
