# Security Considerations

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
|2020-04-08|[CVE-2020-11576](https://nvd.nist.gov/vuln/detail/CVE-2020-11576)|User Enumeration|Medium|v1.5.0|v1.5.1|
|2020-04-08|[CVE-2020-8826](https://nvd.nist.gov/vuln/detail/CVE-2020-8826)|Session-fixation|High|all|n/a|
|2020-04-08|[CVE-2020-8827](https://nvd.nist.gov/vuln/detail/CVE-2020-8827)|Insufficient anti-automation/anti-brute force|High|all|n/a|
|2020-04-08|[CVE-2020-8828](https://nvd.nist.gov/vuln/detail/CVE-2020-8828)|Insecure default administrative password|High|all|n/a|
|2020-04-08|[CVE-2018-21034](https://nvd.nist.gov/vuln/detail/CVE-2018-21034)|Sensitive Information Disclosure|Medium|all <= v1.5.0|v1.5.0|

## Known Issues And Workarounds

A recent security audit (thanks a lot to [Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai) )
has revealed several limitations in Argo CD which could compromise security.
Most of the issues are related to the built-in user management implementation.

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
should only be used for initial configuration and then [disabled](https://argoproj.github.io/argo-cd/operator-manual/user-management/#disable-admin-user)
or at least changed to a more secure password.

### CVE-2020-8827 - Insufficient anti-automation/anti-brute force

**Summary:**

|Risk|Reported by|Fix version|Workaround|
|----|-----------|-----------|----------|
|High|[Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai)|n/a|Yes|

**Details:**

Argo CD does not enforce rate-limiting or other anti-automation mechanisms which would mitigate admin password brute force.

We are considering some simple options for rate-limiting.

**Mitigation and/or workaround:**

As a workaround for mitigation until a fix is available, we recommend to disable local users and use SSO instead.

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

If you find a security related bug in ArgoCD, we kindly ask you for responsible
disclosure and for giving us appropriate time to react, analyze and develop a
fix to mitigate the found security vulnerability.

Please report security vulnerabilities by e-mailing:

* [Jesse_Suen@intuit.com](mailto:Jesse_Suen@intuit.com)
* [Alexander_Matyushentsev@intuit.com](mailto:Alexander_Matyushentsev@intuit.com)
* [Edward_Lee@intuit.com](mailto:Edward_Lee@intuit.com)
