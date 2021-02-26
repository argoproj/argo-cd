# Security Policy for Argo CD

Version: **v1.0 (2020-02-26)**

## Preface

As a deployment tool, Argo CD needs to have production access which makes
security a very important topic. The Argoproj team takes security very
seriously and is continuously working on improving it. 

## Supported Versions

We currently support the most recent release (`N`, e.g. `1.8`) and the release
previous to the most recent one (`N-1`, e.g. `1.7`). With the release of
`N+1`, `N-1` drops out of support and `N` becomes `N-1`.

We regularly perform patch releases (e.g. `1.8.5` and `1.7.12`) for the
supported versions, which will contain fixes for security vulnerabilities and
important bugs. Prior releases might receive critical security fixes on a best
effort basis, however, it cannot be guaranteed that security fixes get
back-ported to these unsupported versions.

In rare cases, where a security fix needs complex re-design of a feature or is
otherwise very intrusive, and there's a workaround available, we may decide to
provide a forward-fix only, e.g. to be released the next minor release, instead
of releasing it within a patch branch for the currently supported releases.

## Reporting a Vulnerability

If you find a security related bug in ArgoCD, we kindly ask you for responsible
disclosure and for giving us appropriate time to react, analyze and develop a
fix to mitigate the found security vulnerability.

We will do our best to react quickly on your inquiry, and to coordinate a fix
and disclosure with you. Sometimes, it might take a little longer for us to
react (e.g. out of office conditions), so please bear with us in these cases.

We will publish security advisiories using the Git Hub SA feature to keep our
community well informed, and will credit you for your findings (unless you
prefer to stay anonymous, of course).

Please report vulnerabilities by e-mail to all of the following people:

* jfischer@redhat.com
* Jesse_Suen@intuit.com
* Alexander_Matyushentsev@intuit.com
* Edward_Lee@intuit.com
