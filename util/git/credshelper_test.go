package git

import (
	"io/ioutil"
	"os"
	"testing"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestExtractGitCredentialHelperRepoDetails(t *testing.T) {
	verify := func(sub *testing.T, input *appsv1.Repository, expected *GitCredentialHelperDetails) {
		details, err := ExtractGitCredentialHelperRepoDetails(input)
		if err != nil {
			sub.Fatal(err)
		}

		if *details != *expected {
			sub.Errorf("Details extracted do not match those expected:\nExpected:\n%+v\nReceived:\n%+v", *expected, *details)
		}
	}

	t.Run("input=Repo with both creds", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo:     "https://github.com/org/repo",
			Username: "Tester",
			Password: "S0S3cure",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
			Password: "S0S3cure",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with URL auth", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo: "https://Test:1ns3cure@github.com/org/repo",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Test",
			Password: "1ns3cure",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with URL username only", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo: "https://Test:@github.com/org/repo",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Test",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with URL auth and both creds override", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo:     "https://Test:1ns3cure@github.com/org/repo",
			Username: "Tester",
			Password: "S0S3cure",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
			Password: "S0S3cure",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with username only", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo:     "https://github.com/org/repo",
			Username: "Tester",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with URL auth and username override", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo:     "https://Test:1ns3cure@github.com/org/repo",
			Username: "Tester",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
			Password: "1ns3cure",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
	t.Run("input=Repo with URL auth and password override", func(sub *testing.T) {
		sub.Parallel()
		repoSpec := appsv1.Repository{
			Repo:     "https://Test:1ns3cure@github.com/org/repo",
			Password: "S0S3cure",
		}
		expectedDetails := GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Test",
			Password: "S0S3cure",
		}
		verify(sub, &repoSpec, &expectedDetails)
	})
}

func TestReadGitCredentialHelperQuery(t *testing.T) {
	runTest := func(sub *testing.T, input []byte, expected *GitCredentialHelperDetails) {
		tmpfile, err := ioutil.TempFile("", "example")
		if err != nil {
			sub.Fatal(err)
		}
		defer os.Remove(tmpfile.Name()) // clean up
		defer func() {
			if err := tmpfile.Close(); err != nil {
				sub.Fatal(err)
			}
		}()

		if _, err := tmpfile.Write(input); err != nil {
			sub.Fatal(err)
		}
		if _, err := tmpfile.Seek(0, 0); err != nil {
			sub.Fatal(err)
		}

		query := GitCredentialHelperDetails{}

		if err := ReadGitCredentialHelperQuery(tmpfile, &query); err != nil {
			sub.Errorf("userInput failed: %v", err)
		}

		if query != *expected {
			sub.Errorf("Query read in does not match that expected:\nExpected:\n%+v\nReceived:\n%+v", *expected, query)
		}
	}

	t.Run("input=Server", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
		})
	})
	t.Run("input=Repo", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\npath=/org/repo\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
		})
	})
	t.Run("input=Credentialed server", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\nusername=Tester\npassword=1ns3cure\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Username: "Tester",
			Password: "1ns3cure",
		})
	})
	t.Run("input=Credentialed Repo", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\npath=/org/repo\nusername=Tester\npassword=1ns3cure\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
			Password: "1ns3cure",
		})
	})
	t.Run("input=Server with username", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\nusername=Tester\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Username: "Tester",
		})
	})
	t.Run("input=Repo with username", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, []byte("protocol=https\nhost=github.com\npath=/org/repo\nusername=Tester\n\n"), &GitCredentialHelperDetails{
			Protocol: "https",
			Host:     "github.com",
			Path:     "/org/repo",
			Username: "Tester",
		})
	})
}

func TestGitCredentialHelperDetaileMatch(t *testing.T) {
	details := GitCredentialHelperDetails{
		Protocol: "https",
		Host:     "github.com",
		Path:     "/org/repo",
		Username: "Tester",
		Password: "S0S3cure",
	}

	runTest := func (sub *testing.T, query *GitCredentialHelperDetails, details *GitCredentialHelperDetails, expected bool) {
		if query.Match(details) != expected {
			sub.Fail()
		}
	}

	t.Run("details==query", func(sub *testing.T) {
		sub.Parallel()
		query := details
		runTest(sub, &query, &details, true)
	})
	t.Run("details>query", func(sub *testing.T) {
		sub.Parallel()
		query := details
		query.Username = ""
		query.Password = ""
		runTest(sub, &query, &details, true)
	})
	t.Run("details!=query", func(sub *testing.T) {
		sub.Parallel()
		query := details
		query.Path = "/other/repo"
		query.Username = ""
		query.Password = ""
		runTest(sub, &query, &details, false)
	})
}

func TestGitCredentialHelperDetaileString(t *testing.T) {
	baseDetails := GitCredentialHelperDetails{
		Protocol: "https",
		Host:     "github.com",
		Path:     "/org/repo",
		Username: "Tester",
		Password: "S0S3cure",
	}

	runTest := func (sub *testing.T, details *GitCredentialHelperDetails, expected string) {
		if details.String() != expected {
			sub.Fail()
		}
	}

	t.Run("allDetails", func(sub *testing.T) {
		sub.Parallel()
		runTest(sub, &baseDetails, "protocol=https\nhost=github.com\npath=/org/repo\nusername=Tester\npassword=S0S3cure\n")
	})
	t.Run("repoDetails-password", func(sub *testing.T) {
		sub.Parallel()
		details := baseDetails
		details.Password = ""
		runTest(sub, &details, "protocol=https\nhost=github.com\npath=/org/repo\nusername=Tester\n")
	})
	t.Run("repoDetails-credentials", func(sub *testing.T) {
		sub.Parallel()
		details := baseDetails
		details.Username = ""
		details.Password = ""
		runTest(sub, &details, "protocol=https\nhost=github.com\npath=/org/repo\n")
	})
	t.Run("serverDetails", func(sub *testing.T) {
		sub.Parallel()
		details := baseDetails
		details.Path = ""
		runTest(sub, &details, "protocol=https\nhost=github.com\nusername=Tester\npassword=S0S3cure\n")
	})
	t.Run("serverDetails-password", func(sub *testing.T) {
		sub.Parallel()
		details := baseDetails
		details.Path = ""
		details.Password = ""
		runTest(sub, &details, "protocol=https\nhost=github.com\nusername=Tester\n")
	})
	t.Run("serverDetails-credentials", func(sub *testing.T) {
		sub.Parallel()
		details := baseDetails
		details.Path = ""
		details.Username = ""
		details.Password = ""
		runTest(sub, &details, "protocol=https\nhost=github.com\n")
	})
}
