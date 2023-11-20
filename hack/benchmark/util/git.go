package util

import (
	"fmt"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

func ForwardGit() (*exec.Cmd, error) {
	log.Debug("Creating forward for Gitea.")
	forwardcmd := exec.Command("kubectl", "port-forward", "svc/gitea-http", "3000:3000", "-n", "gitea")
	if err := forwardcmd.Start(); err != nil {
		return forwardcmd, err
	}

	time.Sleep(5 * time.Second)

	return forwardcmd, nil
}

func PushGit() (string, error) {
	log.Print("Pushing Commit.")
	fs := memfs.New()
	emptyRepo := false
	r, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: "adminuser",
			Password: "password",
		},
		URL: "http://127.0.0.1:3000/adminuser/argobenchmark",
	})

	if err != nil {
		if err == transport.ErrEmptyRemoteRepository {
			emptyRepo = true
		} else {
			return "", err
		}
	}

	err = fs.MkdirAll("configmap2kb", 0755)
	if err != nil {
		return "", err
	}

	src, err := fs.Create("configmap2kb/configmap.yaml")
	if err != nil {
		return "", err
	}

	configMapContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: randomstring-configmap
data:
  randomstring: `
	configMapContent = configMapContent + StringWithCharset(2048)

	_, err = src.Write([]byte(configMapContent))
	if err != nil {
		return "", err
	}

	if emptyRepo {
		r, err = git.Init(memory.NewStorage(), fs)
		if err != nil {
			return "", err
		}
		_, err = r.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{"http://127.0.0.1:3000/adminuser/argobenchmark"},
		})
		if err != nil {
			return "", err
		}
	}

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}

	_, err = w.Add("configmap2kb/configmap.yaml")
	if err != nil {
		return "", err
	}

	commit, err := w.Commit("TestCommit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "John Doe",
			Email: "john@doe.org",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", err
	}

	err = r.Push(&git.PushOptions{
		Auth: &http.BasicAuth{
			Username: "adminuser",
			Password: "password",
		},
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", commit), nil
}
