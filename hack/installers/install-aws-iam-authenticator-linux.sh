#!/bin/bash
set -eux -o pipefail

AWS_IAM_AUTHENTICATOR_VERSION=0.4.0-alpha.1
[ -e $DOWNLOADS/aws-iam-authenticator ] || curl -sLf --retry 3 -o $DOWNLOADS/aws-iam-authenticator https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_amd64
cp $DOWNLOADS/aws-iam-authenticator $BIN/
chmod +x $BIN/aws-iam-authenticator
aws-iam-authenticator version