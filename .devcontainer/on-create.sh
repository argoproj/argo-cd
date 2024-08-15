#!/bin/bash

# this runs as part of pre-build


echo "on-create start"
echo "$(date +'%Y-%m-%d %H:%M:%S')    on-create start" >> "$HOME/status"

# clone repos
git clone https://github.com/cse-labs/imdb-app /workspaces/imdb-app
git clone https://github.com/microsoft/webvalidate /workspaces/webvalidate

# restore the repos
dotnet restore /workspaces/webvalidate/src/webvalidate.sln
dotnet restore /workspaces/imdb-app/src/imdb.csproj

export REPO_BASE=$PWD
export PATH="$PATH:$REPO_BASE/cli"


mkdir -p "$HOME/.ssh"

{
    # add cli to path
    echo "export PATH=\$PATH:$REPO_BASE/cli"

    echo "export REPO_BASE=$REPO_BASE"
    echo "compinit"
} >> "$HOME/.zshrc"

# create local registry
docker network create kind
docker run -d -p 5500:5000 --name kind-registry --network kind registry:2
docker network connect kind kind-registry

# update the base docker images
docker pull mcr.microsoft.com/dotnet/aspnet:6.0-alpine
docker pull mcr.microsoft.com/dotnet/sdk:6.0
docker pull ghcr.io/cse-labs/webv-red:latest

# echo "downloading kic CLI"
# cd cli || exit
# tag="0.4.3"
# wget -O kic.tar.gz "https://github.com/retaildevcrews/akdc/releases/download/$tag/kic-$tag-linux-amd64.tar.gz"
# tar -xvzf kic.tar.gz
# rm kic.tar.gz
# cd "$OLDPWD" || exit

echo "generating completions"
kic completion zsh > "$HOME/.oh-my-zsh/completions/_kic"
kubectl completion zsh > "$HOME/.oh-my-zsh/completions/_kubectl"

echo "creating Kind cluster"
cat <<EOF | kind create cluster --name kind --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP

containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5500"]
    endpoint = ["http://kind-registry:5000"]
EOF


echo "building IMDb"
kic build imdb

echo "building WebValidate"
sed -i "s/RUN dotnet test//g" /workspaces/webvalidate/Dockerfile
kic build webv

echo "deploying to Kind cluster"
kic cluster deploy

# only run apt upgrade on pre-build
if [ "$CODESPACE_NAME" = "null" ]
then
    sudo apt-get update
    sudo apt-get upgrade -y
    sudo apt-get autoremove -y
    sudo apt-get clean -y
fi

echo "on-create complete"
echo "$(date +'%Y-%m-%d %H:%M:%S')    on-create complete" >> "$HOME/status"
