#!/bin/bash

# this runs at Codespace creation - not part of pre-build

echo "post-create start"
echo "$(date)    post-create start" >> "$HOME/status"

sudo apt-get install curl -y
sudo apt-get install make -y

installgo() {
  sudo rm -rf /usr/local/go
  curl -OL https://golang.org/dl/go1.22.2.linux-amd64.tar.gz
  sudo tar -C /usr/local -xvf go1.22.2.linux-amd64.tar.gz

  # Add Go to PATH for the current user
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile

  # Add Go to PATH for all users (including root)
  sudo sh -c 'echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile'

  # Source the updated profile for the current user
  source ~/.profile

  # Source the updated profile for the root user
  sudo sh -c 'source /etc/profile'

  # Delete the downloaded tar file
  rm go1.22.2.linux-amd64.tar.gz
}

# Run the installgo function
installgo

# install nginx ingress by default enabled for kind cluster
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

echo "post-create complete"
echo "$(date +'%Y-%m-%d %H:%M:%S')    post-create complete" >> "$HOME/status"
