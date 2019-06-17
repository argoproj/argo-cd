package testrepos

type HTTPSRepo struct {
	URL, Username, Password string
}

var HTTPSTestRepo = HTTPSRepo{
	URL:      "https://gitlab.com/argo-cd-test/test-apps.git",
	Username: "blah",
	Password: "B5sBDeoqAVUouoHkrovy",
}
