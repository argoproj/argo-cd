package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	db_util "github.com/argoproj/argo-cd/v2/util/db"
	errors_util "github.com/argoproj/argo-cd/v2/util/errors"
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
)

type ArgoCDGitCredentialHelperOpts struct {
	Namespace     string
	KubeClientset kubernetes.Interface
}

type GitCredentialHelperDetails struct {
	Protocol string
	Host     string
	Path     string
	Username string
	Password string
}

type ArgoCDGitCredentialHelper struct {
	ArgoCDGitCredentialHelperOpts

	query       *GitCredentialHelperDetails
	settingsMgr *settings_util.SettingsManager
}

func NewGitCredentialHelper(ctx context.Context, opts ArgoCDGitCredentialHelperOpts) *ArgoCDGitCredentialHelper {
	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	_, err := settingsMgr.InitializeSettings(true)
	errors_util.CheckError(err)

	helper := &ArgoCDGitCredentialHelper{
		ArgoCDGitCredentialHelperOpts: opts,
		settingsMgr:                   settingsMgr,
		query:                         &GitCredentialHelperDetails{},
	}
	err = ReadGitCredentialHelperQuery(os.Stdin, helper.query)
	errors_util.CheckError(err)

	return helper
}

func (helper *ArgoCDGitCredentialHelper) Run(ctx context.Context) {
	db := db_util.NewDB(helper.Namespace, helper.settingsMgr, helper.KubeClientset)

	repositories, err := db.ListRepositories(ctx)
	if err != nil {
		return
	}
	for _, repository := range repositories {
		details, err := ExtractGitCredentialHelperRepoDetails(repository)
		errors_util.CheckError(err)

		if helper.query.Match(details) {
			fmt.Println(details.String())
			return
		}
	}
}

func ReadGitCredentialHelperQuery(source *os.File, repoQuery *GitCredentialHelperDetails) error {
	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := scanner.Text()
		switch lineParts := strings.Split(line, "="); lineParts[0] {
		case "":
			break
		case "protocol":
			if len(lineParts) == 2 {
				repoQuery.Protocol = lineParts[1]
			}
		case "host":
			if len(lineParts) == 2 {
				repoQuery.Host = lineParts[1]
			}
		case "path":
			if len(lineParts) == 2 {
				repoQuery.Path = lineParts[1]
			}
		case "username":
			if len(lineParts) == 2 {
				repoQuery.Username = lineParts[1]
			}
		case "password":
			if len(lineParts) == 2 {
				repoQuery.Password = lineParts[1]
			}
		default:
			return fmt.Errorf("Unknown query part: %s", lineParts[0])
		}
	}
	return nil
}

func ExtractGitCredentialHelperRepoDetails(repo *appsv1.Repository) (*GitCredentialHelperDetails, error) {
	repoURL, err := url.Parse(repo.Repo)
	if err != nil {
		return nil, err
	}
	username := repo.Username
	password := repo.Password
	if username == "" && repoURL.User != nil {
		username = repoURL.User.Username()
	}
	if password == "" && repoURL.User != nil {
		password, _ = repoURL.User.Password()
	}
	return &GitCredentialHelperDetails{
		Host:     repoURL.Host,
		Protocol: repoURL.Scheme,
		Path:     repoURL.Path,
		Username: username,
		Password: password,
	}, nil
}

func (query *GitCredentialHelperDetails) Match(details *GitCredentialHelperDetails) bool {
	if query.Host != "" && query.Host != details.Host {
		return false
	}
	if query.Path != "" && query.Path != details.Path {
		return false
	}
	if query.Protocol != "" && query.Protocol != details.Protocol {
		return false
	}
	if query.Username != "" && query.Username != details.Username {
		return false
	}
	if query.Password != "" && query.Password != details.Password {
		return false
	}
	return true
}

func (details *GitCredentialHelperDetails) String() string {
	var d bytes.Buffer
	if details.Protocol != "" {
		d.WriteString(fmt.Sprintf("protocol=%s\n", details.Protocol))
	}
	if details.Host != "" {
		d.WriteString(fmt.Sprintf("host=%s\n", details.Host))
	}
	if details.Path != "" {
		d.WriteString(fmt.Sprintf("path=%s\n", details.Path))
	}
	if details.Username != "" {
		d.WriteString(fmt.Sprintf("username=%s\n", details.Username))
	}
	if details.Password != "" {
		d.WriteString(fmt.Sprintf("password=%s\n", details.Password))
	}
	return d.String()
}
