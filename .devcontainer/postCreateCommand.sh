#!/usr/bin/env bash

# install ubuntu packages
sudo apt-get update
sudo apt-get install -y redis-server gnupg2 bash-completion

# install tools
curl -sLS https://get.arkade.dev | sudo sh
arkade get kind
arkade get kubectl
echo "export PATH=\$PATH:/home/vscode/.arkade/bin" >> $HOME/.bashrc

# Make sure go path is owned by vscode
sudo chown -R vscode:vscode /home/vscode/go

# install goreman for local development
cat <<EOT >> $HOME/.bashrc
if ! command -v goreman; then
    echo "Installing goreman"
    go install github.com/mattn/goreman@latest
fi
EOT

# setup autocomplete for kubectl and alias k
echo "source <(kubectl completion bash)" >> $HOME/.bashrc
echo "alias k=kubectl" >> $HOME/.bashrc
echo "complete -F __start_kubectl k" >> $HOME/.bashrc
