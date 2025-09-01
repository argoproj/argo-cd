# Docker Image Creation Guide for ArgoCD ECR Integration

This guide covers all aspects of creating, building, and deploying Docker images with ECR support.

## 🏗️ Building Docker Images

### Method 1: Using ArgoCD Makefile (Recommended)

```bash
# Basic build (uses default settings)
make image

# Build with custom tag
make image IMAGE_TAG=myregistry.com/argocd:v3.2.0-ecr.1

# Build with specific platform
make image TARGET_ARCH=linux/amd64

# Build with build date and commit info
make image GIT_TAG=v3.2.0-ecr.1 BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
```

### Method 2: Multi-Platform Build

```bash
# Setup buildx for multi-platform builds (one-time setup)
docker buildx create --name argocd-builder --driver docker-container --use
docker buildx inspect --bootstrap

# Build for multiple platforms
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag myregistry.com/argocd:v3.2.0-ecr.1 \
  --push \
  .

# Build and load locally (single platform)
docker buildx build \
  --platform linux/amd64 \
  --tag argocd-ecr:latest \
  --load \
  .
```

### Method 3: Custom Dockerfile Build

```bash
# Basic Docker build
docker build -t argocd-ecr:latest .

# Build with arguments
docker build \
  --build-arg TARGETOS=linux \
  --build-arg TARGETARCH=amd64 \
  --build-arg GIT_TAG=v3.2.0-ecr.1 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -t myregistry.com/argocd:v3.2.0-ecr.1 \
  .
```

## 📦 Image Optimization

### Dockerfile Analysis

The ArgoCD Dockerfile uses multi-stage builds for optimization:

```dockerfile
# Stage 1: Builder - Dependencies and tools
FROM golang:1.25.0 AS builder
# Installs: helm, kustomize, build tools

# Stage 2: UI Build
FROM node:23.0.0 AS argocd-ui  
# Builds: React UI components

# Stage 3: Go Build
FROM golang:1.25.0 AS argocd-build
# Builds: ArgoCD binaries with ECR support

# Stage 4: Runtime
FROM ubuntu:25.04 AS argocd-base
# Final: Minimal runtime image
```

### Build Optimization Tips

```bash
# Use BuildKit for faster builds
export DOCKER_BUILDKIT=1

# Use build cache mount
docker build --build-arg BUILDKIT_INLINE_CACHE=1 .

# Build only specific target
docker build --target argocd-build -t argocd-build:latest .

# Parallel builds (if you have multiple cores)
make -j$(nproc) image
```

### Image Size Optimization

```bash
# Check image size
docker images argocd-ecr:latest

# Analyze image layers
docker history argocd-ecr:latest

# Use dive tool for detailed analysis
docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  wagoodman/dive:latest argocd-ecr:latest
```

## 🚀 Registry Operations

### Local Registry Testing

```bash
# Run local registry
docker run -d -p 5000:5000 --name registry registry:2

# Tag for local registry
docker tag argocd-ecr:latest localhost:5000/argocd:v3.2.0-ecr.1

# Push to local registry
docker push localhost:5000/argocd:v3.2.0-ecr.1

# Test pull
docker pull localhost:5000/argocd:v3.2.0-ecr.1
```

### ECR Registry Operations

```bash
# Login to ECR
aws ecr get-login-password --region us-west-2 | \
  docker login --username AWS --password-stdin 123456789.dkr.ecr.us-west-2.amazonaws.com

# Create ECR repository (if doesn't exist)
aws ecr create-repository --repository-name argocd --region us-west-2

# Tag for ECR
docker tag argocd-ecr:latest 123456789.dkr.ecr.us-west-2.amazonaws.com/argocd:v3.2.0-ecr.1

# Push to ECR
docker push 123456789.dkr.ecr.us-west-2.amazonaws.com/argocd:v3.2.0-ecr.1

# Verify image in ECR
aws ecr list-images --repository-name argocd --region us-west-2
```

### DockerHub / Other Registries

```bash
# Login to DockerHub
docker login

# Tag and push
docker tag argocd-ecr:latest your-username/argocd:v3.2.0-ecr.1
docker push your-username/argocd:v3.2.0-ecr.1

# Login to private registry
docker login your-registry.com

# Tag and push to private registry
docker tag argocd-ecr:latest your-registry.com/argocd:v3.2.0-ecr.1
docker push your-registry.com/argocd:v3.2.0-ecr.1
```

## 🧪 Image Testing

### Basic Image Validation

```bash
# Test image runs
docker run --rm argocd-ecr:latest argocd version

# Test ECR flags available
docker run --rm argocd-ecr:latest argocd repo add --help | grep ecr

# Test all ArgoCD components
docker run --rm argocd-ecr:latest argocd-server --help | head -3
docker run --rm argocd-ecr:latest argocd-repo-server --help | head -3
docker run --rm argocd-ecr:latest argocd-application-controller --help | head -3
```

### Security Scanning

```bash
# Scan with Docker Scout (if available)
docker scout cves argocd-ecr:latest

# Scan with Trivy
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy:latest image argocd-ecr:latest

# Scan with Grype
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  anchore/grype:latest argocd-ecr:latest
```

### Performance Testing

```bash
# Container resource usage
docker stats $(docker run -d argocd-ecr:latest sleep 60)

# Image layer analysis
docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  wagoodman/dive:latest argocd-ecr:latest

# Startup time test
time docker run --rm argocd-ecr:latest argocd version
```

## 🔄 CI/CD Integration

### GitHub Actions Example

```yaml
name: Build ECR ArgoCD Image
on:
  push:
    branches: [feature/ecr-integration]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3
      
    - name: Login to ECR
      uses: aws-actions/amazon-ecr-login@v2
      
    - name: Build and push
      uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: true
        tags: ${{ env.ECR_REGISTRY }}/argocd:v3.2.0-ecr.1
        cache-from: type=gha
        cache-to: type=gha,mode=max
```

### GitLab CI Example

```yaml
build-ecr-image:
  stage: build
  image: docker:latest
  services:
    - docker:dind
  before_script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
  script:
    - make image IMAGE_TAG=$CI_REGISTRY/argocd:v3.2.0-ecr.1
    - docker push $CI_REGISTRY/argocd:v3.2.0-ecr.1
```

## 🛡️ Security Considerations

### Image Security Best Practices

```bash
# Run as non-root user (ArgoCD already does this)
docker run --rm --user 999:999 argocd-ecr:latest argocd version

# Use read-only filesystem when possible
docker run --rm --read-only argocd-ecr:latest argocd version

# Limit container capabilities
docker run --rm --cap-drop=ALL argocd-ecr:latest argocd version
```

### Registry Security

```bash
# Use image signing (cosign)
cosign sign --key cosign.key your-registry.com/argocd:v3.2.0-ecr.1

# Verify signed image
cosign verify --key cosign.pub your-registry.com/argocd:v3.2.0-ecr.1

# Use image attestations
cosign attest --predicate=attestation.json your-registry.com/argocd:v3.2.0-ecr.1
```

## 📊 Monitoring and Observability

### Container Metrics

```bash
# Resource usage monitoring
docker stats argocd-container-name

# Logs with structured output
docker logs -f argocd-container-name | jq '.'

# Health check monitoring
docker inspect argocd-container-name | jq '.[].State.Health'
```

### ECR-Specific Monitoring

```bash
# Monitor ECR authentication events
docker logs argocd-container-name 2>&1 | grep -i "ecr\|token"

# Check AWS environment in container
docker exec argocd-container-name env | grep AWS

# Monitor cache performance
docker exec argocd-container-name grep -i "cache" /proc/*/cmdline
```

## 🐛 Troubleshooting

### Build Issues

**"Permission denied":**
```bash
# Fix Docker permissions
sudo usermod -aG docker $USER
# Logout and login again
```

**"Go version not supported":**
```bash
# Install Go 1.25+
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
```

**"Protobuf compilation failed":**
```bash
# Install protobuf compiler
sudo apt install -y protobuf-compiler

# Install Go protobuf tools
go install github.com/gogo/protobuf/protoc-gen-gogo@latest
```

### Runtime Issues

**"ECR authentication failed":**
```bash
# Check AWS credentials in container
docker exec container-name aws sts get-caller-identity

# Check ECR access
docker exec container-name aws ecr get-authorization-token --region us-west-2
```

**"Image too large":**
```bash
# Use multi-stage builds (already implemented)
# Remove unnecessary files
# Use .dockerignore
echo "node_modules/" >> .dockerignore
echo "dist/" >> .dockerignore
```

This comprehensive guide provides everything needed for Ubuntu development setup and Docker image creation with ECR support.
