---
title: image Verification Policies
authors:
  - "@anithapriyanatarajan" 
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@anandf"
approvers:
  - TBD

creation-date: 2025-02-20
last-updated: 2025-02-20
---

# Image Verification Policies

This proposal introduces a new feature to verify the integrity of container images before deployment

## Summary

As containerized applications grow in complexity, ensuring the security of deployed images becomes a critical concern. Deploying unsigned or tampered images can lead to potential security risks, such as vulnerabilities being introduced into production environments. Currently, there is no automated process within ArgoCD to verify the integrity of container images before deployment, creating a security gap in the pipeline.

This proposal introduces a new feature to validate the container image using signatures before deployment, providing an additional layer of security by ensuring that only trusted and verified images are deployed in production environments. 

## Motivation

"Shift Left does not mean abandoning the Right."

Argo CD is a widely-used deployment tool that automates the continuous delivery of applications in Kubernetes environments. Since it integrates with source control repositories and can trigger deployments, Argo CD sits at a critical point in the DevOps pipeline. However, there is currently no automated method to verify the integrity of container images before deployment, leaving room for potentially unsafe or tampered images to make their way into production.

Cryptographically signed images and secure supply chain management practices are becoming essential for organizations looking to reduce the risk of vulnerabilities and threats from malicious actors.

While there are multiple ways to produce secure, signed container images, there is no existing feature in Argo CD that automates the verification of these artifacts before deployment. This proposed feature will bridge this gap, offering image verification at the time of deployment to ensure that only trusted and signed images are deployed.

### Goals

* Increase confidence in the integrity of the images deployed to production.
* Verify that images are supplied by trusted sources, ensuring that only cryptographically signed images are deployed.
* Ensure compliance with SLSA (Supply Chain Levels for Software Artifacts) standards, allowing organizations to verify that their images meet the necessary security and compliance levels.
* Abort deployment if the image signature verification fails or if the image does not adhere to the defined SLSA levels.
* Provide flexibility for users to opt into this verification process at the application level, allowing granular control.

### Non-Goals

* To start with we will focus on artifacts signed by sigstore tool cosign only. Verification of images signed by other tools like Notary could be progressed at a later stage.
* Solution to sign images (as this is a separate process handled outside of Argo CD).
* Handle artifacts other than container images (e.g., Helm charts, Kubernetes manifests).

## Proposal


### Use cases

Add a list of detailed use cases this enhancement intends to take care of.

#### Use case 1: Image Signature Verification

As an Argo CD user, I would like to ensure that my applications are only deployed if the container images are built following SLSA standards and have been cryptographically signed by a trusted party.

#### Use case 2: Per-Application Validation

As an Argo CD user, I need the ability to apply different image signature verification policies to different applications. For example, some applications may require stricter image signature checks, while others may be more flexible.

## Implementation Details/Notes/Constraints

We enable enforcement configurations at the `AppProject` level and granular verification configs at the `Application` level.

### Enforcement Configuration in `AppProject`

- **imageVerification**: enabled/disabled - Indicates if image verification should be enabled for the given project.
- **enforcementLevel**: strict/permissive - Indicates if the deployment should be blocked or progressed with a warning.
- **allowedProviders**: e.g., `cosign`, `notary` - Can be expanded to include more options.
- **allowedSigners**: e.g., `kms`,`static`,`keyless` - Allowed signing options
- **minSlsaLevel**: 0/1/2/3/4 - Expected SLSA level of images deployed.
- **defaultSigners**: Default configurations used to verify images for all applications within the project.

```
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: my-secure-project
spec:
  imageVerification:
    enabled: true
    enforcementLevel: strict  # Enforce strict validation across applications
    allowedProviders: ["cosign", "notary"]
    allowedSigners: ["kms", "static", "keyless"]
    minSlsaLevel: 2  # Enforce a minimum SLSA level for all applications
    defaultSigners:
      - method: "kms"
        signer:
          kmsKeyID: "arn:aws:kms:region:account-id:key/key-id"
          kmsProvider: "AWS"
      - method: "static"
        signer:
          publicKey: "trusted-public-key"
```

### Granular Configuration in `Application`

Application level image verification config overrides the default signer configuration at the AppProject level

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-secure-app
spec:
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  source:
    repoURL: 'https://github.com/my-repo'
    targetRevision: HEAD
    path: charts/my-secure-app
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
  imageVerification:
    signers:
      - method: "static"
        publicKey: "/mnt/<path>" #mount path where the public key can be loaded from a configmap or from external sources
      - method: "kms"
        kmsKeyID: "arn:aws:kms:region:account-id:key/key-id"
        kmsProvider: "AWS"
      - method: "keyless"
        provider: "Fulcio"
        providerUrl: "https://fulcio.example.com/cert-ref"
    slsaLevel: 3

```
### Detailed examples

#### Deploying an Image with KMS-Based Signature Verification

Below is a sample specification using AWS KMS to verify the signature of the container image before deployment

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-secure-app
spec:
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  source:
    repoURL: 'https://github.com/my-repo'
    targetRevision: HEAD
    path: charts/my-secure-app
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    imageVerification:
      signers:
       - method: "kms"
         kmsKeyID: "arn:aws:kms:region:account-id:key/key-id"
         kmsProvider: "AWS"
      slsaLevel: 3
```

In this example, 
* The image’s signature is verified using AWS KMS.
* The KMS key ID is provided along with the KMS provider (AWS).
* SLSA Level 3 ensures the image has met specific security standards for artifact verification.

### Security Considerations

* Ensures only trusted and signed container images are deployed, reducing the risk of deploying malicious or tampered images.
* Helps meet security standards and regulations by enforcing image verification policies.

### Risks and Mitigations

#### Risk-1: Misconfiguration

Incorrectly configuring the verification method, public keys, or KMS details could prevent valid images from being deployed, leading to downtime or deployment failures.

Mitigation: Implement thorough testing before production rollouts. Use automation to validate the configuration and ensure all required keys or secrets are available.

#### Risk-2: Performance Overhead

Image signature verification could introduce some latency in the deployment process, especially when dealing with large images or complex cryptographic operations.

Mitigation:  Evaluate the performance impact during testing and ensure optimizations for large-scale deployments (e.g., caching signature verification results, asynchronous checks).

### Upgrade / Downgrade Strategy

#### Upgrade


#### Downgrade


## Drawbacks

* Setting up source verification policies and ensuring they align with the security standards (SLSA, signature keys, etc.) can be complex, especially in multi-cloud or hybrid environments.

* Integrating KMS or other third-party providers for signature verification could introduce dependencies and potential availability risks. If KMS or keyless providers are not accessible, deployments could fail.

* Misconfiguration of signing keys, KMS, or secrets could result in failed image verifications, leading to deployment issues or downtime.

## Alternatives

1. Kyverno can be used to enforce image verification policies across the Kubernetes cluster, ensuring that only trusted images are deployed. This provides centralized policy enforcement without altering Argo CD configurations. However, the advantage of the proposed approach within Argo CD is the granularity it offers at the application level, allowing each application to have its own image verification policies. This ensures that image verification is closely tied to the lifecycle and specific requirements of the application, providing more fine-grained control over deployment security compared to cluster-wide enforcement.

2. As an alternative, image verification can be implemented using Argo CD's Workflow Hook functionality, where a Pre-Sync hook runs the verification process before syncing with the Kubernetes cluster. While this approach works, the proposed solution offers greater flexibility by allowing different image verification methods (e.g., KMS, keyless, secret) to be specified directly within the sync policy. It’s simple to use and ensures a consistent, standardized way to ensure image quality across all applications.
