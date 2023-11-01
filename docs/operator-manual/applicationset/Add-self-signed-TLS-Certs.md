# Add support for self-signed TLS / Certificates for Gitlab SCM/PR Provider

## Implementation details

### Overview

In order for a self-signed TLS certificate be used by an ApplicationSet's SCM / PR Gitlab Generator, the certificate needs to be mounted on the application-controller. The path of the mounted certificate must be explicitly set using the environment variable `ARGOCD_APPLICATIONSET_CONTROLLER_SCM_ROOT_CA_PATH` or alternatively using parameter `--scm-root-ca-path`. The applicationset controller will read the mounted certificate to create the Gitlab client for SCM/PR Providers

This can be achieved conveniently by setting `applicationsetcontroller.scm.root.ca.path` in the argocd-cmd-params-cm ConfigMap. Be sure to restart the ApplicationSet controller after setting this value.
