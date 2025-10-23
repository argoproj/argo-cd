### **time**
Time related functions.

<hr>
**`time.Now() Time`**

Executes function built-in Golang [time.Now](https://golang.org/pkg/time/#Now) function. Returns an instance of
Golang [Time](https://golang.org/pkg/time/#Time).

<hr>
**`time.Parse(val string) Time`**

Parses specified string using RFC3339 layout. Returns an instance of Golang [Time](https://golang.org/pkg/time/#Time).

<hr>
Time related constants.

**Durations**

```
	time.Nanosecond   = 1
	time.Microsecond  = 1000 * Nanosecond
	time.Millisecond  = 1000 * Microsecond
	time.Second       = 1000 * Millisecond
	time.Minute       = 60 * Second
	time.Hour         = 60 * Minute
```

**Timestamps**

Used when formatting time instances as strings (e.g. `time.Now().Format(time.RFC3339)`).

```
	time.Layout      = "01/02 03:04:05PM '06 -0700" // The reference time, in numerical order.
	time.ANSIC       = "Mon Jan _2 15:04:05 2006"
	time.UnixDate    = "Mon Jan _2 15:04:05 MST 2006"
	time.RubyDate    = "Mon Jan 02 15:04:05 -0700 2006"
	time.RFC822      = "02 Jan 06 15:04 MST"
	time.RFC822Z     = "02 Jan 06 15:04 -0700" // RFC822 with numeric zone
	time.RFC850      = "Monday, 02-Jan-06 15:04:05 MST"
	time.RFC1123     = "Mon, 02 Jan 2006 15:04:05 MST"
	time.RFC1123Z    = "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
	time.RFC3339     = "2006-01-02T15:04:05Z07:00"
	time.RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
	time.Kitchen     = "3:04PM"
	// Handy time stamps.
	time.Stamp      = "Jan _2 15:04:05"
	time.StampMilli = "Jan _2 15:04:05.000"
	time.StampMicro = "Jan _2 15:04:05.000000"
	time.StampNano  = "Jan _2 15:04:05.000000000"
```

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
