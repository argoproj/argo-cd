/*
Package fieldmanager is a special package as its main purpose
is to expose the dependencies required by structured-merge-diff
library to calculate diffs when server-side apply option is enabled.
The dependency tree necessary to have a `merge.Updater` instance
isn't trivial to implement and the strategy used is borrowing a copy
from Kubernetes apiserver codebase in order to expose the required
functionality.

Below there is a list of borrowed files and a reference to which
package/file in Kubernetes they were copied from:

- borrowed_fields.go: k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/internal/fields.go
- borrowed_managedfields.go: k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/internal/managedfields.go
- borrowed_typeconverter.go: k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/typeconverter.go
- borrowed_versionconverter.go: k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/versionconverter.go

In order to keep maintenance as minimal as possible the borrowed
files are verbatim copy from Kubernetes. The private objects that
need to be exposed are wrapped in the wrapper.go file. Updating
the borrowed files should be trivial in most cases but must be done
manually as we have no control over future refactorings Kubernetes
might do.
*/
package fieldmanager
