### **time**
Time related functions.

<hr>
**`time.Now() Time`**

Executes function built-in Golang [time.Now](https://golang.org/pkg/time/#Now) function. Returns an instance of
Golang [Time](https://golang.org/pkg/time/#Time).

<hr>
**`time.Parse(val string) Time`**

Parses specified string using RFC3339 layout. Returns an instance of Golang [Time](https://golang.org/pkg/time/#Time).

### **strings**
String related functions.

<hr>
**`strings.ReplaceAll() string`**

Executes function built-in Golang [strings.ReplaceAll](https://pkg.go.dev/strings#ReplaceAll) function.

<hr>
**`strings.ToUpper() string`**

Executes function built-in Golang [strings.ToUpper](https://pkg.go.dev/strings#ToUpper) function.

<hr>
**`strings.ToLower() string`**

Executes function built-in Golang [strings.ToLower](https://pkg.go.dev/strings#ToLower) function.

### **sync**

<hr>
**`sync.GetInfoItem(app map, name string) string`**
Returns the `info` item value by given name stored in the Argo CD App sync operation.

### **repo**
Functions that provide additional information about Application source repository.
<hr>
**`repo.RepoURLToHTTPS(url string) string`**

Transforms given GIT URL into HTTPs format.

<hr>
**`repo.FullNameByRepoURL(url string) string`**

Returns repository URL full name `(<owner>/<repoName>)`. Currently supports only Github, GitLab and Bitbucket.

<hr>
**`repo.QueryEscape(s string) string`**

QueryEscape escapes the string, so it can be safely placed inside a URL

Example:
```
/projects/{{ call .repo.QueryEscape (call .repo.FullNameByRepoURL .app.status.RepoURL) }}/merge_requests
```

<hr>
**`repo.GetCommitMetadata(sha string) CommitMetadata`**

Returns commit metadata. The commit must belong to the application source repository. `CommitMetadata` fields:

* `Message string` commit message
* `Author string` - commit author
* `Date time.Time` - commit creation date
* `Tags []string` - Associated tags

<hr>
**`repo.GetAppDetails() AppDetail`**

Returns application details. `AppDetail` fields:

* `Type string` - AppDetail type
* `Helm HelmAppSpec` - Helm details
  * Fields :
    * `Name string`
    * `ValueFiles []string`
    * `Parameters []*v1alpha1.HelmParameter`
    * `Values string`
    * `FileParameters []*v1alpha1.HelmFileParameter`
  * Methods :
    * `GetParameterValueByName(Name string)` Retrieve value by name in Parameters field
    * `GetFileParameterPathByName(Name string)` Retrieve path by name in FileParameters field
*
* `Kustomize *apiclient.KustomizeAppSpec` - Kustomize details
* `Directory *apiclient.DirectoryAppSpec` - Directory details
