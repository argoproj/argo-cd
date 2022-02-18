package generator

import (
	"bytes"
	"context"
	"encoding/base64"
	"log"
	"strings"

	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/util/helm"

	"gopkg.in/yaml.v2"

	"k8s.io/client-go/kubernetes/scheme"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/util"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
)

const POD_PREFIX = "vcluster"

type Cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
}

type AuthInfo struct {
	ClientCertificateData string `yaml:"client-certificate-data,omitempty"`
	ClientKeyData         string `yaml:"client-key-data,omitempty"`
}

type NamedCluster struct {
	// Name is the nickname for this Cluster
	Name string `yaml:"name"`
	// Cluster holds the cluster information
	Cluster Cluster `yaml:"cluster"`
}

type NamedAuthInfo struct {
	// Name is the nickname for this AuthInfo
	Name string `yaml:"name"`
	// AuthInfo holds the auth information
	AuthInfo AuthInfo `yaml:"user"`
}

type Config struct {
	Clusters  []NamedCluster  `yaml:"clusters"`
	AuthInfos []NamedAuthInfo `yaml:"users"`
}

type ClusterGenerator struct {
	db        db.ArgoDB
	clientSet *kubernetes.Clientset
	config    *rest.Config
}

func NewClusterGenerator(db db.ArgoDB, clientSet *kubernetes.Clientset, config *rest.Config) Generator {
	return &ClusterGenerator{db, clientSet, config}
}

func (cg *ClusterGenerator) getClusterCredentials(namespace string, releaseSuffix string) ([]byte, []byte, []byte, error) {
	cmd := []string{
		"sh",
		"-c",
		"cat /root/.kube/config",
	}

	var stdout, stderr, stdin bytes.Buffer
	option := &v1.PodExecOptions{
		Command:   cmd,
		Container: "syncer",
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}

	req := cg.clientSet.CoreV1().RESTClient().Post().Resource("pods").Name(POD_PREFIX + "-" + releaseSuffix + "-0").
		Namespace(namespace).SubResource("exec")

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(cg.config, "POST", req.URL())
	if err != nil {
		return nil, nil, nil, err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	var config Config

	err = yaml.Unmarshal(stdout.Bytes(), &config)
	if err != nil {
		return nil, nil, nil, err
	}

	caData, err := base64.StdEncoding.DecodeString(config.Clusters[0].Cluster.CertificateAuthorityData)
	if err != nil {
		return nil, nil, nil, err
	}

	cert, err := base64.StdEncoding.DecodeString(config.AuthInfos[0].AuthInfo.ClientCertificateData)
	if err != nil {
		return nil, nil, nil, err
	}

	key, err := base64.StdEncoding.DecodeString(config.AuthInfos[0].AuthInfo.ClientKeyData)
	if err != nil {
		return nil, nil, nil, err
	}

	return caData, cert, key, nil
}

//TODO: also should provision service for vcluster pod
func (cg *ClusterGenerator) installVCluster(opts *util.GenerateOpts, namespace string, releaseName string) error {
	cmd, err := helm.NewCmd("/tmp", "v3", "")
	if err != nil {
		return err
	}
	log.Print("Execute helm install command")
	_, err = cmd.Freestyle("install", releaseName, "vcluster", "--values", opts.ClusterOpts.ValuesFilePath, "--repo", "https://charts.loft.sh", "--namespace", namespace, "--repository-config", "", "--create-namespace", "--wait")
	if err != nil {
		return err
	}
	return nil
}

func (cg *ClusterGenerator) getClusterServerUri(namespace string, releaseSuffix string) (string, error) {
	pod, err := cg.clientSet.CoreV1().Pods(namespace).Get(context.TODO(), POD_PREFIX+"-"+releaseSuffix+"-0", v12.GetOptions{})
	if err != nil {
		return "", err
	}
	// TODO: should be moved to service instead pod
	return "https://" + pod.Status.PodIP + ":8443", nil
}

func (cg *ClusterGenerator) Generate(opts *util.GenerateOpts) error {
	for i := 0; i < opts.ClusterOpts.Samples; i++ {
		log.Printf("Generate cluster #%v", i)

		namespace := opts.ClusterOpts.NamespacePrefix + "-" + util.GetRandomString()

		log.Printf("Namespace is %s", namespace)

		releaseSuffix := util.GetRandomString()

		log.Printf("Release suffix is %s", namespace)

		err := cg.installVCluster(opts, namespace, POD_PREFIX+"-"+releaseSuffix)
		if err != nil {
			log.Printf("Skip cluster installation due error %v", err.Error())
			continue
		}

		log.Print("Get cluster credentials")
		caData, cert, key, err := cg.getClusterCredentials(namespace, releaseSuffix)
		if err != nil {
			return err
		}

		log.Print("Get cluster server uri")
		uri, err := cg.getClusterServerUri(namespace, releaseSuffix)
		if err != nil {
			return err
		}

		log.Printf("Cluster server uri is %s", uri)

		log.Print("Create cluster")
		_, err = cg.db.CreateCluster(context.TODO(), &argoappv1.Cluster{
			Server: uri,
			Name:   opts.ClusterOpts.ClusterNamePrefix + "-" + util.GetRandomString(),
			Config: argoappv1.ClusterConfig{
				TLSClientConfig: argoappv1.TLSClientConfig{
					Insecure:   false,
					ServerName: "kubernetes.default.svc",
					CAData:     caData,
					CertData:   cert,
					KeyData:    key,
				},
			},
			ConnectionState: argoappv1.ConnectionState{},
			ServerVersion:   "1.18",
			Namespaces:      []string{opts.ClusterOpts.DestinationNamespace},
			Labels:          labels,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (cg *ClusterGenerator) Clean(opts *util.GenerateOpts) error {
	log.Printf("Clean clusters")
	namespaces, err := cg.clientSet.CoreV1().Namespaces().List(context.TODO(), v12.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, POD_PREFIX) {
			log.Printf("Delete namespace %s", ns.Name)
			err = cg.clientSet.CoreV1().Namespaces().Delete(context.TODO(), ns.Name, v12.DeleteOptions{})
			if err != nil {
				log.Printf("Delete namespace failed due: %s", err.Error())
			}
		}
	}

	secrets := cg.clientSet.CoreV1().Secrets(opts.Namespace)
	return secrets.DeleteCollection(context.TODO(), v12.DeleteOptions{}, v12.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-generator",
	})
}
