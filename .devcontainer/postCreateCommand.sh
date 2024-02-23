#!/usr/bin/env sh

# install ubuntu packages
sudo apt-get update
sudo apt-get install -y gnupg2 bash-completion

# install tools
curl -sLS https://get.arkade.dev | sudo sh
arkade get kind
arkade get kubectl
arkade get helm
arkade get gh
echo "export PATH=\$PATH:/home/vscode/.arkade/bin" >> $HOME/.bashrc

# install goreman for local development
go install github.com/mattn/goreman@latest

# Make sure go path is owned by vscode
sudo chown -R vscode:vscode /home/vscode/go

# setup autocomplete for kubectl and alias k
echo "source <(kubectl completion bash)" >> $HOME/.bashrc
echo "alias k=kubectl" >> $HOME/.bashrc
echo "complete -F __start_kubectl k" >> $HOME/.bashrc

