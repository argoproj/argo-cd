package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/argoproj/argo-cd/v2/performance-test/tools"

	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

type Repo struct {
	Id  int    `json:"id"`
	Url string `json:"html_url"`
}

type RepoGenerator struct {
	clientSet *kubernetes.Clientset
	bar       *tools.Bar
}

func NewRepoGenerator(clientSet *kubernetes.Clientset) Generator {
	return &RepoGenerator{clientSet: clientSet, bar: &tools.Bar{}}
}

func fetchRepos(token string, page int) ([]Repo, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/argoproj/argocd-example-apps/forks?per_page=100&page=%v", page), nil)
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(body))
	var repos []Repo
	err = json.Unmarshal(body, &repos)
	if err != nil {
		return nil, err
	}
	return repos, nil
}

func FetchRepos(token string) ([]Repo, error) {
	var (
		repos []Repo
		page  = 1
	)

	for {
		fetchedRepos, err := fetchRepos(token, page)
		if err != nil {
			return nil, err
		}
		if len(fetchedRepos) == 0 {
			break
		}
		repos = append(repos, fetchedRepos...)
		page++
	}
	return repos, nil
}

func (rg *RepoGenerator) Generate(opts *GenerateOpts) error {
	repos, err := FetchRepos(opts.GithubToken)
	if err != nil {
		return err
	}

	secrets := rg.clientSet.CoreV1().Secrets("argocd")
	rg.bar.NewOption(0, int64(len(repos)))
	for _, repo := range repos {
		_, err = secrets.Create(context.TODO(), &v1.Secret{
			ObjectMeta: v1meta.ObjectMeta{
				GenerateName: "repo-",
				Namespace:    "argocd",
				Labels: map[string]string{
					"app.kubernetes.io/generated-by": "argocd-generator",
					"argocd.argoproj.io/secret-type": "repository",
				},
				Annotations: map[string]string{
					"managed-by": "argocd.argoproj.io",
				},
			},
			Data: map[string][]byte{
				"type":    []byte("git"),
				"url":     []byte(repo.Url),
				"project": []byte("default"),
			},
		}, v1meta.CreateOptions{})
		rg.bar.Increment()
		rg.bar.Play()
	}
	rg.bar.Finish()
	if err != nil {
		return err
	}
	return nil
}

func (rg *RepoGenerator) Clean() error {
	secrets := rg.clientSet.CoreV1().Secrets("argocd")
	return secrets.DeleteCollection(context.TODO(), v1meta.DeleteOptions{}, v1meta.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-generator",
	})
}
