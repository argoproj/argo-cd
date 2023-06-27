## argocd proj add-verification-rule

Adds a source verification rule for given project

### Synopsis


The add-verification-rule command adds a source verification rule.

A source verification rule defines how to verify the source repositories that
are used by the project's applications.

VERIFICATION MODES:

- head:   This mode will verify only the commit that's pointed to by the HEAD
          of the application's target revision. If given target revision is an
          annotated tag, only the signature of the tag will be verified.
- lax:    This mode will verify all commits between the revision an application
          has last synced to and the target revision. If the target revision is
		  an annotated tag, the tag's signature will be verified additionally.
- strict: This mode will verify all commits in the repository that led up to
          target revision. This mode ensures that all commits in the history
		  of the repository have been signed.

CURRENT LIMITATIONS AND CAVEATS:

Each AppProject only supports a single verification rule, so any subsequent
call to add-verification-rule will overwrite the existing verification rule
instead of adding one. Also, a source verification rule has no effect when
there is no GnuPG verification key defined.

This will change in a future release of Argo CD, as the source verification
feature is progressing.
		

```
argocd proj add-verification-rule PROJECT --repo-pattern=<pattern> --verification-type=<type> --verification-mode=<mode> [flags]
```

### Options

```
  -h, --help                       help for add-verification-rule
      --repo-pattern string        The repository pattern to match
      --signature-keys strings     List of keys allowed to make signatures
      --source-type string         The source type (one of: git) (default "git")
      --verification-mode string   The verification mode to use (one of: off, head, lax, strict)
      --verification-type string   The verification type to use (one of: gpg) (default "gpg")
```

### Options inherited from parent commands

```
      --auth-token string               Authentication token
      --client-crt string               Client certificate file
      --client-crt-key string           Client certificate key file
      --config string                   Path to Argo CD config (default "/home/user/.config/argocd/config")
      --core                            If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server
      --grpc-web                        Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.
      --grpc-web-root-path string       Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.
  -H, --header strings                  Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)
      --http-retry-max int              Maximum number of retries to establish http connection to Argo CD server
      --insecure                        Skip server certificate and domain verification
      --kube-context string             Directs the command to the given kube-context
      --logformat string                Set the logging format. One of: text|json (default "text")
      --loglevel string                 Set the logging level. One of: debug|info|warn|error (default "info")
      --plaintext                       Disable TLS
      --port-forward                    Connect to a random argocd-server port using port forwarding
      --port-forward-namespace string   Namespace name which should be used for port forwarding
      --server string                   Argo CD server address
      --server-crt string               Server certificate file
```

### SEE ALSO

* [argocd proj](argocd_proj.md)	 - Manage projects

