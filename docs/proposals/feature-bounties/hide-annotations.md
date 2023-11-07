# Proposal: Allow Hiding Certain Annotations in the Argo CD Web UI

Based on this issue: https://github.com/argoproj/argo-cd/issues/15693

Award amount: $100

## Solution

!!! note
     This is the proposed solution. The accepted PR may differ from this proposal.

Add a new config item in argocd-cm:

```yaml
hide.secret.annotations: |
- openshift.io/token-secret.value
```

This will hide the `openshift.io/token-secret.value` annotation from the UI. Behind the scenes, it would likely work the
same way as the `last-applied-configuration` annotation hiding works: https://github.com/argoproj/gitops-engine/blob/b0fffe419a0f0a40f9f2c0b6346b752ed6537385/pkg/diff/diff.go#L897

I considered whether we'd want to support hiding things besides annotations and in resources besides secrets, but
having reviewed existing issues, I think this narrow feature is sufficient.
