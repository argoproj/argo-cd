#!/bin/bash
set -eux -o pipefail

AWS_IAM_AUTHENTICATOR_VERSION=0.4.0-alpha.1
curl -L -o $BIN/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64
chmod +x $BIN/aws-iam-authenticator