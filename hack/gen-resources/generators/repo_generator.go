package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	v1 "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/util"

	"k8s.io/client-go/kubernetes"
)

type Repo struct {
	Id  int    `json:"id"`
	Url string `json:"html_url"`
}

type RepoGenerator struct {
	clientSet *kubernetes.Clientset
	bar       *util.Bar
}

func NewRepoGenerator(clientSet *kubernetes.Clientset) Generator {
	return &RepoGenerator{clientSet: clientSet, bar: &util.Bar{}}
}

func fetchRepos(token string, page int) ([]Repo, error) {
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/argoproj/argocd-example-apps/forks?per_page=100&page=%v", page), nil)
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var repos []Repo
	err = json.Unmarshal(body, &repos)
	if err != nil {
		return nil, errors.New("failed to retrieve repos, reason: " + string(body))
	}
	return repos, nil
}

func FetchRepos(token string, samples int) ([]Repo, error) {
	log.Print("Fetch repos started")
	var (
		repos []Repo
		page  = 1
	)

	for {
		if page%10 == 0 {
			log.Printf("Fetch repos, page: %v", page)
		}
		fetchedRepos, err := fetchRepos(token, page)
		if err != nil {
			return nil, err
		}
		if len(fetchedRepos) == 0 {
			break
		}
		if len(repos)+len(fetchedRepos) > samples {
			repos = append(repos, fetchedRepos[0:samples-len(repos)]...)
			break
		}
		repos = append(repos, fetchedRepos...)
		page++
	}
	return repos, nil
}

func (rg *RepoGenerator) Generate(opts *util.GenerateOpts) error {
	repos, err := FetchRepos(opts.GithubToken, opts.RepositoryOpts.Samples)
	if err != nil {
		return err
	}

	secrets := rg.clientSet.CoreV1().Secrets(opts.Namespace)
	rg.bar.NewOption(0, int64(len(repos)))
	for _, repo := range repos {
		_, err = secrets.Create(context.TODO(), &v1.Secret{
			ObjectMeta: v1meta.ObjectMeta{
				GenerateName: "repo-",
				Namespace:    opts.Namespace,
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

func (rg *RepoGenerator) Clean(opts *util.GenerateOpts) error {
	log.Printf("Clean repos")
	secrets := rg.clientSet.CoreV1().Secrets(opts.Namespace)
	return secrets.DeleteCollection(context.TODO(), v1meta.DeleteOptions{}, v1meta.ListOptions{
		LabelSelector: "app.kubernetes.io/generated-by=argocd-generator",
	})
}
