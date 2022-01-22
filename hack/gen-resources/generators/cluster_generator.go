package generator

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func (cg *ClusterGenerator) Generate(opts *util.GenerateOpts) error {
	//for i := 0; i < opts.ClusterOpts.Samples; i++ {
	//	cmd, err := helm.NewCmd("/tmp", "v3", "")
	//	if err != nil {
	//		return err
	//	}
	//	res, err := cmd.Freestyle("install", "vcluster-1", "vcluster", "--values", "/Users/pashavictorovich/.kube/util/values.yaml", "--repo", "https://charts.loft.sh", "--namespace", "host-namespace-5", "--repository-config", "", "--create-namespace")
	//	if err != nil {
	//		return err
	//	}
	//	fmt.Println(res)
	//}

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

	req := cg.clientSet.CoreV1().RESTClient().Post().Resource("pods").Name("vcluster-1-0").
		Namespace("host-namespace-5").SubResource("exec")

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(cg.config, "POST", req.URL())
	if err != nil {
		return err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return err
	}

	var config Config

	err = yaml.Unmarshal([]byte(stdout.String()), &config)
	if err != nil {
		return err
	}

	fmt.Println(stderr.String())
	fmt.Println(stdout.String())

	caData, err := base64.StdEncoding.DecodeString(config.Clusters[0].Cluster.CertificateAuthorityData)
	if err != nil {
		return err
	}

	cert, err := base64.StdEncoding.DecodeString(config.AuthInfos[0].AuthInfo.ClientCertificateData)
	if err != nil {
		return err
	}

	key, err := base64.StdEncoding.DecodeString(config.AuthInfos[0].AuthInfo.ClientKeyData)
	if err != nil {
		return err
	}

	tlsClientConfig := argoappv1.TLSClientConfig{
		Insecure:   false,
		ServerName: "kubernetes.default.svc",
		CAData:     []byte(caData),
		CertData:   []byte(cert),
		KeyData:    []byte(key),
	}

	pod, err := cg.clientSet.CoreV1().Pods("host-namespace-5").Get(context.TODO(), "vcluster-1-0", v12.GetOptions{})
	if err != nil {
		return err
	}

	_, err = cg.db.CreateCluster(context.TODO(), &argoappv1.Cluster{
		Server: "https://" + pod.Status.PodIP + ":8443",
		Name:   "pasha-test-6",
		Config: argoappv1.ClusterConfig{
			TLSClientConfig: tlsClientConfig,
		},
		ConnectionState: argoappv1.ConnectionState{},
		ServerVersion:   "1.18",
		Namespaces:      []string{"argocd"},
	})

	return err
}

func (cg *ClusterGenerator) Clean(opts *util.GenerateOpts) error {
	return nil
}
