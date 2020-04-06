# Security Considerations

As a deployment tool, Argo CD needs to have production access which makes security a very important topic. 
The Argoproj team takes security very seriously and continuously working on improving it. Learn more about security
related features in [Security](./operator-manual/security.md) section.

## Known Issues And Workarounds

A recent security audit (thanks a lot to [Matt Hamilton](https://github.com/Eriner) of [https://soluble.ai](https://soluble.ai) )
has revealed several limitations in Argo CD which could compromise security.
Most of the issues are related to the built-in user management implementation.

#### Insecure default administrative password - CVE-2020-8828

Argo CD uses the `argocd-server` pod name (ex: `argocd-server-55594fbdb9-ptsf5`) as the default admin password.

Kubernetes users able to list pods in the argo namespace are able to retrieve the default password.

Additionally, In most installations, [the Pod name contains a random "trail" of characters](https://github.com/kubernetes/kubernetes/blob/dda530cfb74b157f1d17b97818aa128a9db8e711/staging/src/k8s.io/apiserver/pkg/storage/names/generate.go#L37).
These characters are generated using [a time-seeded PRNG](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L26) and not a CSPRNG.
An attacker could use this information in an attempt to deduce the state of the internal PRNG, aiding bruteforce attacks.

The recommended mitigation as described in the user documentation is to use SSO integration. The default admin password
should only be used for initial configuration and then [disabled](https://argoproj.github.io/argo-cd/operator-manual/user-management/#disable-admin-user)
or at least changed to a more secure password.

#### Insufficient anti-automation/anti-brute force - CVE-2020-8827

Argo CD does not enforce rate-limiting or other anti-automation mechanisms which would mitigate admin password brute force.

We are considering some simple options for rate-limiting.

#### Session-fixation - CVE-2020-8826

The authentication tokens generated for built-in users have no expiry.

These issues might be acceptable in the controlled isolated environment but not acceptable if Argo CD user interface is
exposed to the Internet.

The recommended mitigation is to change the password periodically to invalidate the authentication tokens.

## Reporting Vulnerabilities

Please report security vulnerabilities by e-mailing:

* [Jesse_Suen@intuit.com](mailto:Jesse_Suen@intuit.com)
* [Alexander_Matyushentsev@intuit.com](mailto:Alexander_Matyushentsev@intuit.com)
* [Edward_Lee@intuit.com](mailto:Edward_Lee@intuit.com)
