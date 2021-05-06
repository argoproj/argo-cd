# Security Considerations

!!!warning "Deprecation notice"
    This page is now deprecated and serves as an archive only. For up-to-date
    information, please have a look at our
    [security policy](https://github.com/argoproj/argo-cd/security/policy) and
    [published security advisories](https://github.com/argoproj/argo-cd/security/advisories).

As a deployment tool, Argo CD needs to have production access which makes security a very important topic.
The Argoproj team takes security very seriously and continuously working on improving it. Learn more about security
related features in [Security](./operator-manual/security.md) section.

## Overview of past and current issues

The following table gives a general overview about past and present issues known
to the ArgoCD project. See in the [Known Issues](#known-issues-and-workarounds)
section if there is a work-around available if you cannot update or if there is
no fix yet.

|Date|CVE|Title|Risk|Affected version(s)|Fix version|
|----|---|-----|----|-------------------|-----------|
|2020-06-16|[CVE-2020-1747](https://nvd.nist.gov/vuln/detail/CVE-2020-1747)|PyYAML library susceptible to arbitrary code execution|High|all|v1.5.8|
|2020-06-16|[CVE-2020-14343](https://nvd.nist.gov/vuln/detail/CVE-2020-14343)|PyYAML library susceptible to arbitrary code execution|High|all|v1.5.8|
|2020-04-14|[CVE-2020-5260](https://nvd.nist.gov/vuln/detail/CVE-2020-5260)|Possible Git credential leak|High|all|v1.4.3,v1.5.2|
|2020-04-08|[CVE-2020-11576](https://nvd.nist.gov/vuln/detail/CVE-2020-11576)|User Enumeration|Medium|v1.5.0|v1.5.1|
|2020-04-08|[CVE-2020-8826](https://nvd.nist.gov/vuln/detail/CVE-2020-8826)|Session-fixation|High|all|n/a|
|2020-04-08|[CVE-2020-8827](https://nvd.nist.gov/vuln/detail/CVE-2020-8827)|Insufficient anti-automation/anti-brute force|High|all <= 1.5.3|v1.5.3|
|2020-04-08|[CVE-2020-8828](https://nvd.nist.gov/vuln/detail/CVE-2020-8828)|Insecure default administrative password|High|all|n/a|
|2020-04-08|[CVE-2018-21034](https://nvd.nist.gov/vuln/detail/CVE-2018-21034)|Sensitive Information Disclosure|Medium|all <= v1.5.0|v1.5.0|

## Known Issues And Workarounds

A recent security audit (thanks a lot to [Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai) )
has revealed several limitations in Argo CD which could compromise security.
Most of the issues are related to the built-in user management implementation.

### CVE-2020-1747, CVE-2020-14343 - PyYAML library susceptible to arbitrary code execution

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|High|[infa-kparida](https://github.com/infa-kparida)|v1.5.8|No|

**Details:**

PyYAML library susceptible to arbitrary code execution when it processes untrusted YAML files.
We do not believe ArgoCD is affected by this vulnerability, because the impact of CVE-2020-1747 and CVE-2020-14343 is limited to usage of awscli.
The `awscli` only used for AWS IAM authentication, and the endpoint is the AWS API.

### CVE-2020-5260 - Possible Git credential leak

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|Critical|Felix Wilhelm of Google Project Zero|v1.4.3,v1.5.2|Yes|

**Details:**

ArgoCD relies on Git for many of its operations. The Git project released a
[security advisory](https://github.com/git/git/security/advisories/GHSA-qm7j-c969-7j4q)
on 2020-04-14, describing a serious vulnerability in Git which can lead to credential
leakage through credential helpers by feeding malicious URLs to the `git clone`
operation.

We do not believe ArgoCD is affected by this vulnerability, because ArgoCD does neither
make use of Git credential helpers nor does it use `git clone` for repository operations.
However, we do not know whether our users might have configured Git credential helpers on
their own and chose to release new images which contain the bug fix for Git.

**Mitigation and/or workaround:**

We strongly recommend to upgrade your ArgoCD installation to either `v1.4.3` (if on v1.4
branch) or `v1.5.2` (if on v1.5 branch) 


When you are running `v1.4.x`, you can upgrade to `v1.4.3` by simply changing the image
tags for `argocd-server`, `argocd-repo-server` and `argocd-controller` to `v1.4.3`. 
The `v1.4.3` release does not contain additional functional bug fixes.

Likewise, hen you are running `v1.5.x`, you can upgrade to `v1.5.2` by simply changing
the image tags for `argocd-server`, `argocd-repo-server` and `argocd-controller` to `v1.5.2`.
The `v1.5.2` release does not contain additional functional bug fixes.

### CVE-2020-11576 - User Enumeration

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|Medium|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|v1.5.1|Yes|

**Details:**

Argo version v1.5.0 was vulnerable to a user-enumeration vulnerability which allowed attackers to determine the usernames of valid (non-SSO) accounts within Argo.

**Mitigation and/or workaround:**

Upgrade to ArgoCD v1.5.1 or higher. As a workaround, disable local users and use only SSO authentication.

### CVE-2020-8828 - Insecure default administrative password

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|High|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|n/a|Yes|

**Details:**

Argo CD uses the `argocd-server` pod name (ex: `argocd-server-55594fbdb9-ptsf5`) as the default admin password.

Kubernetes users able to list pods in the argo namespace are able to retrieve the default password.

Additionally, In most installations, [the Pod name contains a random "trail" of characters](https://github.com/kubernetes/kubernetes/blob/dda530cfb74b157f1d17b97818aa128a9db8e711/staging/src/k8s.io/apiserver/pkg/storage/names/generate.go#L37).
These characters are generated using [a time-seeded PRNG](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L26) and not a CSPRNG.
An attacker could use this information in an attempt to deduce the state of the internal PRNG, aiding bruteforce attacks.

**Mitigation and/or workaround:**

The recommended mitigation as described in the user documentation is to use SSO integration. The default admin password
should only be used for initial configuration and then [disabled](../operator-manual/user-management/#disable-admin-user)
or at least changed to a more secure password.

### CVE-2020-8827 - Insufficient anti-automation/anti-brute force

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|High|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|n/a|Yes|

**Details:**

ArgoCD before v1.5.3 does not enforce rate-limiting or other anti-automation mechanisms which would mitigate admin password brute force.

**Mitigation and/or workaround:**

Rate-limiting and anti-automation mechanisms for local user accounts have been introduced with ArgoCD v1.5.3.

As a workaround for mitigation if you cannot upgrade ArgoCD to v1.5.3 yet, we recommend to disable local users and use SSO instead.

### CVE-2020-8826 - Session-fixation

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|High|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|n/a|Yes|

**Details:**

The authentication tokens generated for built-in users have no expiry.

These issues might be acceptable in the controlled isolated environment but not acceptable if Argo CD user interface is
exposed to the Internet.

**Mitigation and/or workaround:**

The recommended mitigation is to change the password periodically to invalidate the authentication tokens.

### CVE-2018-21034 - Sensitive Information Disclosure

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|Medium|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|v1.5.0|No|

**Details:**

In Argo versions prior to v1.5.0-rc1, it was possible for authenticated Argo users to submit API calls to retrieve secrets and other manifests which were stored within git.

**Mitigation and/or workaround:**

Upgrade to ArgoCD v1.5.0 or higher. No workaround available

## Reporting Vulnerabilities

Please have a look at our
[security policy](https://github.com/argoproj/argo-cd/security/policy)
for more details on how to report security vulnerabilities for Argo CD.
