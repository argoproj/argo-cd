# Ubuntu Development Setup for ArgoCD ECR Integration

This guide provides step-by-step instructions to set up a complete ArgoCD development environment on Ubuntu Linux.

## 🖥️ System Requirements

- **OS:** Ubuntu 20.04 LTS or newer (22.04 LTS recommended)
- **RAM:** 8GB minimum, 16GB recommended
- **Disk:** 50GB free space minimum
- **CPU:** 4 cores minimum for reasonable build times

## 📋 Prerequisites Installation

### Step 1: Update System

```bash
sudo apt update && sudo apt upgrade -y
```

### Step 2: Install Essential Development Tools

```bash
# Install basic build tools
sudo apt install -y \
    build-essential \
    curl \
    wget \
    git \
    unzip \
    software-properties-common \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release

# Install additional utilities
sudo apt install -y \
    jq \
    yamllint \
    tree \
    htop \
    vim \
    tmux
```

## 🔧 Development Environment Setup

### Step 3: Install Go 1.25

```bash
# Download Go 1.25
cd /tmp
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz

# Install Go
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
source ~/.bashrc

# Verify installation
go version  # Should show go1.25.0
```

### Step 4: Install Node.js and npm (for UI development)

```bash
# Install Node.js 20.x
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Verify installation
node --version  # Should show v20.x
npm --version   # Should show 10.x
```

### Step 5: Install Docker

```bash
# Add Docker GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Add Docker repository
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin

# Add user to docker group (logout/login required)
sudo usermod -aG docker $USER

# Enable Docker service
sudo systemctl enable docker
sudo systemctl start docker

# Verify installation (after logout/login)
docker --version
docker run hello-world
```

### Step 6: Install kubectl

```bash
# Download kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"

# Install kubectl
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Verify installation
kubectl version --client
```

### Step 7: Install AWS CLI v2

```bash
# Download AWS CLI v2
cd /tmp
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip

# Install AWS CLI
sudo ./aws/install

# Verify installation
aws --version  # Should show aws-cli/2.x
```

### Step 8: Install Helm

```bash
# Download and install Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Verify installation
helm version  # Should show v3.x
```

### Step 9: Install Protocol Buffer Compiler

```bash
# Install protobuf compiler
sudo apt install -y protobuf-compiler

# Verify installation
protoc --version  # Should show libprotoc 3.x or higher
```

## 🛠️ ArgoCD Development Setup

### Step 10: Clone and Setup ArgoCD

```bash
# Create development directory
mkdir -p $HOME/dev/argocd
cd $HOME/dev/argocd

# Clone your ECR-enabled ArgoCD fork
git clone <your-fork-with-ecr-changes> argo-cd
cd argo-cd

# Setup Go dependencies
go mod download && go mod tidy
```

### Step 11: Install ArgoCD Development Tools

```bash
# Install Go development tools
go install github.com/vektra/mockery/v3@v3.5.0
go install github.com/gogo/protobuf/protoc-gen-gogo@latest
go install github.com/gogo/protobuf/protoc-gen-gogofast@latest
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.16.0
go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.16.0
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/go-swagger/go-swagger/cmd/swagger@latest
go install k8s.io/code-generator/cmd/go-to-protobuf@latest

# Install additional development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/gotestyourself/gotestsum@latest
```

### Step 12: Build ArgoCD with ECR Support

```bash
# First build (this will install additional dependencies)
make argocd-all

# Verify build
./dist/argocd version
./dist/argocd repo add --help | grep ecr

# Run ECR tests
go test ./util/helm/ -run TestAWSECR -v
```

## 🐳 Docker Image Creation

### Method 1: Using ArgoCD Makefile (Recommended)

```bash
# Build standard image
make image

# Build with custom tag
make image IMAGE_TAG=your-registry.com/argocd:v3.2.0-ecr.1

# Build for multiple architectures
make image DOCKER_BUILDKIT=1 TARGET_ARCH=linux/amd64,linux/arm64 IMAGE_TAG=your-registry.com/argocd:v3.2.0-ecr.1
```

### Method 2: Manual Docker Build

```bash
# Basic build
docker build -t argocd-ecr:latest .

# Build with build args
docker build \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  --build-arg GIT_TAG=v3.2.0-ecr.1 \
  -t your-registry.com/argocd:v3.2.0-ecr.1 \
  .

# Multi-stage build optimization
docker build \
  --target argocd-build \
  --build-arg BUILDPLATFORM=linux/amd64 \
  -t argocd-build:latest \
  .
```

### Method 3: Multi-Platform Build with Buildx

```bash
# Setup buildx (one-time setup)
docker buildx create --name multiarch --driver docker-container --use
docker buildx inspect --bootstrap

# Multi-platform build
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag your-registry.com/argocd:v3.2.0-ecr.1 \
  --push \
  .

# Build for specific platform
docker buildx build \
  --platform linux/amd64 \
  --tag argocd-ecr:latest \
  --load \
  .
```

## 🔍 Verification Steps

### Verify Development Environment

```bash
# Check all tools
echo "=== Development Environment Check ==="
echo "Go: $(go version)"
echo "Node: $(node --version)"
echo "Docker: $(docker --version)"
echo "kubectl: $(kubectl version --client --short)"
echo "AWS CLI: $(aws --version)"
echo "Helm: $(helm version --short)"
echo "Protoc: $(protoc --version)"

# Check Go tools
echo ""
echo "=== Go Tools Check ==="
which mockery || echo "❌ mockery missing"
which protoc-gen-gogo || echo "❌ protoc-gen-gogo missing"  
which goimports || echo "❌ goimports missing"
which golangci-lint || echo "❌ golangci-lint missing"
```

### Verify Docker Image

```bash
# Test image functionality
docker run --rm your-registry.com/argocd:v3.2.0-ecr.1 argocd version

# Test ECR flags in image
docker run --rm your-registry.com/argocd:v3.2.0-ecr.1 argocd repo add --help | grep ecr

# Check image size
docker images your-registry.com/argocd:v3.2.0-ecr.1

# Inspect image layers
docker history your-registry.com/argocd:v3.2.0-ecr.1
```

## ⚡ Quick Setup Script

Save this as `ubuntu-dev-setup.sh` for automated installation:

```bash
#!/bin/bash

set -e

echo "🐧 Ubuntu ArgoCD Development Setup"
echo "=================================="

# Update system
sudo apt update && sudo apt upgrade -y

# Install development tools
sudo apt install -y build-essential curl wget git unzip software-properties-common \
    apt-transport-https ca-certificates gnupg lsb-release jq yamllint tree htop vim tmux

# Install Go 1.25
cd /tmp
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz

# Install Node.js 20
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# Install Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update && sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
sudo usermod -aG docker $USER

# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install AWS CLI v2
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip && sudo ./aws/install

# Install Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Install protobuf
sudo apt install -y protobuf-compiler

# Setup environment variables
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc  
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc

echo ""
echo "✅ Ubuntu development environment setup complete!"
echo "🔄 Please logout and login again for Docker group changes to take effect"
echo "🚀 Then run: source ~/.bashrc"
```

## 🔧 Development Workflow

### Daily Development Commands

```bash
# Build and test
make argocd-all && go test ./util/helm/ -run TestAWSECR

# Quick ECR validation
./test-ecr-complete.sh

# Code generation (after protobuf changes)
make protogen

# Lint code
golangci-lint run

# Build Docker image
make image IMAGE_TAG=dev/argocd:latest

# Run integration tests
./testing/test-ecr-basic.sh
```

### IDE Setup (VS Code)

```bash
# Install VS Code
wget -qO- https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > packages.microsoft.gpg
sudo install -o root -g root -m 644 packages.microsoft.gpg /etc/apt/trusted.gpg.d/
echo "deb [arch=amd64,arm64,armhf signed-by=/etc/apt/trusted.gpg.d/packages.microsoft.gpg] https://packages.microsoft.com/repos/code stable main" | sudo tee /etc/apt/sources.list.d/vscode.list
sudo apt update && sudo apt install -y code

# Recommended VS Code extensions
code --install-extension golang.go
code --install-extension ms-vscode.vscode-typescript-next
code --install-extension ms-kubernetes-tools.vscode-kubernetes-tools
code --install-extension ms-vscode.docker
```

### Environment Variables

Add to `~/.bashrc`:

```bash
# Go environment
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

# ArgoCD development
export ARGOCD_DEV_PATH=$HOME/dev/argocd/argo-cd
export PATH=$PATH:$ARGOCD_DEV_PATH/dist

# AWS (for ECR testing)
export AWS_DEFAULT_REGION=us-west-2
export AWS_PAGER=""

# Development aliases
alias argocd-dev='$ARGOCD_DEV_PATH/dist/argocd'
alias ecr-test='$ARGOCD_DEV_PATH/test-ecr-complete.sh'
```

## 🚀 Performance Optimization

### For Faster Builds

```bash
# Enable Go module proxy
go env -w GOPROXY=https://proxy.golang.org,direct

# Enable build cache
go env -w GOCACHE=$HOME/.cache/go-build

# Increase Go test timeout
go env -w GOTESTSUM_FORMAT=standard-verbose
```

### For Docker Builds

```bash
# Enable BuildKit
echo 'export DOCKER_BUILDKIT=1' >> ~/.bashrc

# Configure Docker daemon
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json > /dev/null <<EOF
{
  "features": {
    "buildkit": true
  },
  "storage-driver": "overlay2"
}
EOF

sudo systemctl restart docker
```

This setup provides a complete Ubuntu development environment for ArgoCD ECR integration work.
