package upgrade

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Version struct {
	Major  int
	Minor  int
	Patch  int
	SemVer string
	Number float64
}

type Upgrade struct {
	clientSet      kubernetes.Interface
	namespace      string
	currentVersion Version
	upgradeVersion Version
}

func (u *Upgrade) GetCurrentVersion() (version Version) {
	return u.currentVersion
}

func (u *Upgrade) GetUpgradeVersion() (version Version) {
	return u.upgradeVersion
}

func (u *Upgrade) SetCurrentVersion(currentVersionTag string) (err error) {
	u.currentVersion, err = getVersionElements(currentVersionTag)
	if err != nil {
		return err
	}
	return nil
}

func (u *Upgrade) SetUpgradeVersion(upgradeVersionTag string) (err error) {
	u.upgradeVersion, err = getVersionElements(upgradeVersionTag)
	if err != nil {
		return err
	}
	return nil
}

func (u *Upgrade) getClientSet() kubernetes.Interface {
	return u.clientSet
}

func (u *Upgrade) setClientSet(clientSet kubernetes.Interface) {
	u.clientSet = clientSet
}

func (u *Upgrade) getNamespace() (namespace string) {
	return u.namespace
}

func (u *Upgrade) setNamespace(namespace string) {
	u.namespace = namespace
}

func (u *Upgrade) getConfigMap(configMapName string) (configMap *corev1.ConfigMap, err error) {
	configMap, err = u.clientSet.CoreV1().ConfigMaps(u.namespace).
		Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return configMap, nil
}

func (u *Upgrade) configMapExists(configMapName string) (exists bool) {
	_, err := u.getConfigMap(configMapName)
	return err == nil
}

func (u *Upgrade) configMapKeyExists(configMap *corev1.ConfigMap, key string) bool {
	if _, ok := configMap.Data[key]; ok {
		return true
	}
	return false
}

func (u *Upgrade) configMapValueContains(configMap *corev1.ConfigMap, key string, value string) bool {
	return strings.Contains(configMap.Data[key], value)
}

func (u *Upgrade) configMapValueEqual(configMap *corev1.ConfigMap, key string, value string) bool {
	return configMap.Data[key] == value
}

func (u *Upgrade) configMapValueRegex(configMap *corev1.ConfigMap, key string, value string) (matches []string) {
	re := regexp.MustCompile(value)
	matches = re.FindAllString(configMap.Data[key], -1)
	return matches
}

var NewForConfig = func(c *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(c)
}

func (u *Upgrade) getKubeConfigWithClient(clientConfig clientcmd.ClientConfig) error {
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}
	u.setNamespace(namespace)

	clientSet, err := NewForConfig(restConfig)
	if err != nil {
		return err
	}
	u.setClientSet(clientSet)
	return nil
}

func Run(u *Upgrade, clientConfig clientcmd.ClientConfig) (err error) {
	err = u.getKubeConfigWithClient(clientConfig)
	if err != nil {
		return err
	}

	check, err := getCheck(fmt.Sprintf("v%dv%d", u.GetCurrentVersion().Major, u.GetUpgradeVersion().Major))
	if err != nil {
		return err
	}

	checkList, err := check.performChecks(u)
	if err != nil {
		return err
	}

	err = printChecks(checkList)
	if err != nil {
		return err
	}

	return nil
}

func getVersionElements(versionTag string) (version Version, err error) {
	re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionTag)

	if len(matches) != 4 {
		return Version{}, fmt.Errorf("invalid version tag format: %s", versionTag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	semVer := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	number, _ := strconv.ParseFloat(fmt.Sprintf("%d.%d", major, minor), 64)

	return Version{
		Major:  major,
		Minor:  minor,
		Patch:  patch,
		SemVer: semVer,
		Number: number,
	}, nil
}
