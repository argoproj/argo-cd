# Security Policy for Argo CD

Version: **v1.4 (2022-01-23)**

## Preface

As a deployment tool, Argo CD needs to have production access which makes
security a very important topic. The Argoproj team takes security very
seriously and is continuously working on improving it. 

## A word about security scanners

Many organisations these days employ security scanners to validate their
container images before letting them on their clusters, and that is a good
thing. However, the quality and results of these scanners vary greatly,
many of them produce false positives and require people to look at the
issues reported and validate them for correctness. A great example of that
is, that some scanners report kernel vulnerabilities for container images
just because they are derived from some distribution.

We kindly ask you to not raise issues or contact us regarding any issues
that are found by your security scanner. Many of those produce a lot of false
positives, and many of these issues don't affect Argo CD. We do have scanners
in place for our code, dependencies and container images that we publish. We
are well aware of the issues that may affect Argo CD and are constantly
working on the remediation of those that affect Argo CD and our users.

If you believe that we might have missed an issue that we should take a look
at (that can happen), then please discuss it with us. If there is a CVE
assigned to the issue, please do open an issue on our GitHub tracker instead
of writing to the security contact e-mail, since things reported by scanners
are public already and the discussion that might emerge is of benefit to the
general community. However, please validate your scanner results and its
impact on Argo CD before opening an issue at least roughly.

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

We will publish security advisiories using the
[Git Hub Security Advisories](https://github.com/argoproj/argo-cd/security/advisories)
feature to keep our community well informed, and will credit you for your
findings (unless you prefer to stay anonymous, of course).

Please report vulnerabilities by e-mail to the following address: 

* cncf-argo-security@lists.cncf.io

## Securing your Argo CD Instance

See the [operator manual security page](docs/operator-manual/security.md) for 
additional information about Argo CD's security features and how to make your 
Argo CD production ready.
