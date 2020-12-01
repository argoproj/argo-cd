## argocd-util rbac validate

Validate RBAC policy

### Synopsis


Validates an RBAC policy for being syntactically correct. The policy must be
a local file, and in either CSV or K8s ConfigMap format.


```
argocd-util rbac validate --policy-file=POLICYFILE [flags]
```

### Options

```
  -h, --help                 help for validate
      --policy-file string   path to the policy file to use
```

### SEE ALSO

* [argocd-util rbac](argocd-util_rbac.md)	 - Validate and test RBAC configuration

