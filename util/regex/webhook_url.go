package regex

import (
	"fmt"
	"net/url"
	"regexp"
)

// https://www.rfc-editor.org/rfc/rfc3986#section-3.2.1
// https://github.com/shadow-maint/shadow/blob/master/libmisc/chkname.c#L36
const usernameRegex = `[a-zA-Z0-9_\.][a-zA-Z0-9_\.-]{0,30}[a-zA-Z0-9_\.\$-]?`

// BuildWebhookRegExp compiles a regex that will match any targetRevision referring to the same repo as the given webURL.
// webURL is expected to be a URL from an SCM webhook payload pointing to the web page for the repo.
func BuildWebhookRegExp(webURL string) (*regexp.Regexp, error) {
	urlObj, err := url.Parse(webURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repoURL '%s'", webURL)
	}

	regexEscapedHostname := regexp.QuoteMeta(urlObj.Hostname())
	regexEscapedPath := regexp.QuoteMeta(urlObj.Path[1:])
	regexpStr := fmt.Sprintf(`(?i)^(http://|https://|http://(%s@)|https://(%s@)|%s@|ssh://(%s@)?)%s(:[0-9]+|)[:/]%s(\.git)?$`,
		usernameRegex, usernameRegex, usernameRegex, usernameRegex, regexEscapedHostname, regexEscapedPath)

	repoRegexp, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regexp for repoURL '%s'", webURL)
	}

	return repoRegexp, nil
}
