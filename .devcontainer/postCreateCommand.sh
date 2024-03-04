#!/usr/bin/env bash

# install ubuntu packages
sudo apt-get update && sudo apt-get install -y redis-server bash-completion

# For Kubectl AMD64 / x86_64
[ $(uname -m) = x86_64 ] && curl -sLO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
# For Kubectl ARM64
[ $(uname -m) = aarch64 ] && curl -sLO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"
chmod +x ./kubectl
sudo mv ./kubectl /usr/local/bin/kubectl

# For Kind AMD64 / x86_64
[ $(uname -m) = x86_64 ] && curl -sLo ./kind https://kind.sigs.k8s.io/dl/v0.22.0/kind-linux-amd64
# For Kind ARM64
[ $(uname -m) = aarch64 ] && curl -sLo ./kind https://kind.sigs.k8s.io/dl/v0.22.0/kind-linux-arm64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Make sure go path is owned by vscode
sudo chown -R vscode:vscode /home/vscode/go

# setup autocomplete for kubectl and alias k
mkdir $HOME/.kube
echo "source <(kubectl completion bash)" >> $HOME/.bashrc
echo "alias k=kubectl" >> $HOME/.bashrc
echo "complete -F __start_kubectl k" >> $HOME/.bashrc
