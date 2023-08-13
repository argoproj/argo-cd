package reposerver

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/git"

	//"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	"github.com/argoproj/argo-cd/v2/reposerver/metrics"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
)

// **************************************************************************
// Find soulution for this: cannot reference a different revision of the same repository ERROR
// This error thrown if using annotated tag with multisource applications

type InMemoryCredsStore struct {
    creds map[string]string
}

func NewInMemoryCredsStore() *InMemoryCredsStore {
    return &InMemoryCredsStore{
        creds: make(map[string]string),
    }
}

func (s *InMemoryCredsStore) Add(username string, token string) string {
    s.creds[username] = token

	return username
}

func (s *InMemoryCredsStore) Remove(username string) {
    delete(s.creds, username)
}

func Test_Start(t *testing.T) {
	fmt.Println("TestMain")
}


func readApplicationFromYamlFile(t *testing.T, path string) *argoappv1.Application {
	yamlBytes, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}

	app := argoappv1.Application{}
	yaml.Unmarshal(yamlBytes, &app)

	return &app
}


func Test_AppFromFile(t *testing.T) {
	app := readApplicationFromYamlFile(t, "/workspaces/argo-tests/test_files/Application.yaml")
	expected := "eck-orchestration-app"
	real_name := app.ObjectMeta.Name
	if real_name != expected {
		t.Errorf("Expected %s, got %s", expected, real_name)
	}

	privateKeyBytes, err := ioutil.ReadFile("/workspaces/argo-tests/test_files/github-app-private.key")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(privateKeyBytes))
	
	creds_store := NewInMemoryCredsStore()
	g_creds := git.NewGitHubAppCreds(
		278113,
		32812496,
		string(privateKeyBytes),
		"",
		app.Spec.Sources[0].RepoURL,
		"",
		"",
		true,
		"",
		creds_store,
	 )

	 clc, arr, err := g_creds.Environ()
	 if err != nil {
		 t.Fatal(err)
	 }
	 
	 t.Logf("%s %s", clc, arr)
	// svc := createService()
	// fmt.Println(svc)
}

func getInitializedCredsStore(t *testing.T) (*InMemoryCredsStore, []string, *argoappv1.Application)	 {
	app := readApplicationFromYamlFile(t, "/workspaces/argo-tests/test_files/Application.yaml")
	expected := "eck-orchestration-app"
	real_name := app.ObjectMeta.Name
	if real_name != expected {
		t.Errorf("Expected %s, got %s", expected, real_name)
	}

	privateKeyBytes, err := ioutil.ReadFile("/workspaces/argo-tests/test_files/github-app-private.key")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(privateKeyBytes))
	
	creds_store := NewInMemoryCredsStore()
	g_creds := git.NewGitHubAppCreds(
		278113,
		32812496,
		string(privateKeyBytes),
		"",
		app.Spec.Sources[0].RepoURL,
		"",
		"",
		true,
		"",
		creds_store,
	 )

	 _, arr, err := g_creds.Environ()
	 if err != nil {
		 t.Fatal(err)
	 }

	 return creds_store, arr, app
}

func getInitializedCreds(t *testing.T) (git.GenericHTTPSCreds, *argoappv1.Application) {
	app := readApplicationFromYamlFile(t, "/workspaces/argo-tests/test_files/Application.yaml")
	expected := "eck-orchestration-app"
	real_name := app.ObjectMeta.Name
	if real_name != expected {
		t.Errorf("Expected %s, got %s", expected, real_name)
	}

	privateKeyBytes, err := ioutil.ReadFile("/workspaces/argo-tests/test_files/github-app-private.key")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(privateKeyBytes))
	
	creds_store := NewInMemoryCredsStore()
	g_creds := git.NewGitHubAppCreds(
		278113,
		32812496,
		string(privateKeyBytes),
		"",
		app.Spec.Sources[0].RepoURL,
		"",
		"",
		true,
		"",
		creds_store,
	 )

	 return g_creds, app
}


func newService(t *testing.T) (*repository.Service, *argoappv1.Application) {
	
	creds_store, _, app := getInitializedCredsStore(t)

	service := repository.NewService(metrics.NewMetricsServer(), cache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
			1*time.Minute,
			1*time.Minute,), 
			repository.RepoServerInitConstants{ParallelismLimit: 1}, 
			argo.NewResourceTracking(), 
			creds_store, "/workspaces/argo-tests/test_files/test_repo")	

	return service, app
}

func TestResolveRevision(t *testing.T) {
	service, app := newService(t)

	fmt.Printf("Read application %s \n", app.ObjectMeta.Name)
	privateKeyBytes, err := ioutil.ReadFile("/workspaces/argo-tests/test_files/github-app-private.key")
	if err != nil {
		t.Fatal(err)
	}
	repo := &argoappv1.Repository{ 
		Repo: app.Spec.Sources[0].RepoURL,
		GithubAppId: 278113,
		GithubAppPrivateKey: string(privateKeyBytes),
		GithubAppInstallationId: 32812496,
	}
	
	response, err := service.ResolveRevision(
		context.Background(),
		&apiclient.ResolveRevisionRequest{
			Repo: repo,
			App: app,
			AmbiguousRevision: "v1.6.0",
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("Response: %s \n", response)
}


func Test_serviceInit(t *testing.T) {
	service, _ := newService(t)
	service.Init()
}

func Test_resolveAnnotatedTags(t *testing.T) {
	creds_store, app := getInitializedCreds(t)
	
	client, err := git.NewClientExt(app.Spec.Sources[0].RepoURL, "/workspaces/argo-tests/test_files/test_repo/", creds_store,  false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	
	client.Init()
	client.Checkout("v1.6.0",false)	
}


func createManifestRequest(t *testing.T, app *argoappv1.Application) *apiclient.ManifestRequest {
	
	repos := []*argoappv1.Repository{}
	refSources := map[string]*argoappv1.RefTarget{}

	privateKeyBytes, err := ioutil.ReadFile("/workspaces/argo-tests/test_files/github-app-private.key")
	if err != nil {
		t.Fatal(err)
	}

	for _, src := range app.Spec.Sources {
		r := &argoappv1.Repository{
			Repo: src.RepoURL,
		}

		repos = append(repos, r)

		if src.Ref != "" {
			refKey := "$" + src.Ref
			refSources[refKey] = &argoappv1.RefTarget{
				Repo: argoappv1.Repository{
					Repo: src.RepoURL,
					GithubAppId: 278113,
					GithubAppPrivateKey: string(privateKeyBytes),
					GithubAppInstallationId: 32812496,
				},
				TargetRevision: src.TargetRevision,
				Chart: src.Chart,
			}
		}
	}

	mainRepo := &argoappv1.Repository{ 
		Repo: app.Spec.Sources[0].RepoURL,
		GithubAppId: 278113,
		GithubAppPrivateKey: string(privateKeyBytes),
		GithubAppInstallationId: 32812496,
	}

	
	appSrc := app.Spec.GetSource()
	return &apiclient.ManifestRequest{
		Repos: repos,
		NoCache: true,
		ApplicationSource: &appSrc,
		Repo: mainRepo,
		HasMultipleSources: len(app.Spec.Sources) > 1,
		RefSources: refSources,
	}	
}


func Test_generateApplicationManifet(t *testing.T) {
	service, app := newService(t)

	mreq := createManifestRequest(t, app)  
	
	mres, err := service.GenerateManifest(context.Background(), mreq)

	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(mres)

	

}

