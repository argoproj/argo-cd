# ECR Docker Image Deployment Guide

This guide covers building and deploying ArgoCD Docker images with ECR support to various environments.

## 🏗️ Building ECR-Enabled Images

### Quick Build Commands

```bash
# Development image (local testing)
make image IMAGE_TAG=argocd-ecr:dev

# Production image (with version)
make image IMAGE_TAG=your-registry.com/argocd:v3.2.0-ecr.1

# Multi-platform production image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag your-registry.com/argocd:v3.2.0-ecr.1 \
  --push \
  .
```

### Build Verification

```bash
# Test basic functionality
docker run --rm argocd-ecr:dev argocd version

# Verify ECR support
docker run --rm argocd-ecr:dev argocd repo add --help | grep ecr

# Check image size
docker images argocd-ecr:dev
```

## 📦 Registry Deployment Strategies

### Strategy 1: AWS ECR Deployment

Perfect for EKS clusters - keeps images in same AWS account:

```bash
# 1. Create ECR repository
aws ecr create-repository \
  --repository-name argocd \
  --region us-west-2

# 2. Login to ECR
aws ecr get-login-password --region us-west-2 | \
  docker login --username AWS --password-stdin 123456789.dkr.ecr.us-west-2.amazonaws.com

# 3. Build and tag for ECR
make image IMAGE_TAG=123456789.dkr.ecr.us-west-2.amazonaws.com/argocd:v3.2.0-ecr.1

# 4. Push to ECR
docker push 123456789.dkr.ecr.us-west-2.amazonaws.com/argocd:v3.2.0-ecr.1

# 5. Deploy to EKS
kubectl set image deployment/argocd-repo-server \
  argocd-repo-server=123456789.dkr.ecr.us-west-2.amazonaws.com/argocd:v3.2.0-ecr.1 \
  -n argocd
```

### Strategy 2: DockerHub Deployment

For public distribution or multi-cloud deployments:

```bash
# 1. Login to DockerHub
docker login

# 2. Build and tag
make image IMAGE_TAG=your-username/argocd:v3.2.0-ecr.1

# 3. Push to DockerHub
docker push your-username/argocd:v3.2.0-ecr.1

# 4. Deploy
kubectl set image deployment/argocd-repo-server \
  argocd-repo-server=your-username/argocd:v3.2.0-ecr.1 \
  -n argocd
```

### Strategy 3: Private Registry Deployment

For enterprise environments:

```bash
# 1. Login to private registry
docker login your-registry.com

# 2. Build and tag
make image IMAGE_TAG=your-registry.com/argocd/argocd:v3.2.0-ecr.1

# 3. Push to private registry
docker push your-registry.com/argocd/argocd:v3.2.0-ecr.1

# 4. Create image pull secret (if required)
kubectl create secret docker-registry regcred \
  --docker-server=your-registry.com \
  --docker-username=your-user \
  --docker-password=your-pass \
  -n argocd

# 5. Deploy with image pull secret
kubectl patch serviceaccount argocd-repo-server \
  -p '{"imagePullSecrets": [{"name": "regcred"}]}' \
  -n argocd
```

## ☸️ Kubernetes Deployment Patterns

### Pattern 1: Single Component Update (Repo Server Only)

Best for testing ECR functionality:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-repo-server
  namespace: argocd
spec:
  template:
    spec:
      serviceAccountName: argocd-repo-server  # Must have IRSA annotation
      containers:
      - name: argocd-repo-server
        image: your-registry.com/argocd:v3.2.0-ecr.1
        command: ["argocd-repo-server"]
        env:
        - name: ARGOCD_LOG_LEVEL
          value: debug
        # AWS environment variables injected by IRSA
```

### Pattern 2: Full ArgoCD Installation Update

For production deployments:

```yaml
# Using Helm chart
helm upgrade argocd argo/argo-cd \
  --set global.image.repository=your-registry.com/argocd \
  --set global.image.tag=v3.2.0-ecr.1 \
  --set repoServer.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::123456789:role/argocd-ecr-role" \
  -n argocd

# Using Kustomize
# kustomization.yaml:
images:
- name: quay.io/argoproj/argocd
  newName: your-registry.com/argocd
  newTag: v3.2.0-ecr.1
```

### Pattern 3: Blue-Green Deployment

For zero-downtime updates:

```bash
# 1. Deploy new version alongside existing
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argocd-repo-server-ecr
  namespace: argocd
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-repo-server-ecr
  template:
    metadata:
      labels:
        app.kubernetes.io/name: argocd-repo-server-ecr
    spec:
      serviceAccountName: argocd-repo-server
      containers:
      - name: argocd-repo-server
        image: your-registry.com/argocd:v3.2.0-ecr.1
        command: ["argocd-repo-server"]
EOF

# 2. Test new deployment
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-repo-server-ecr -n argocd

# 3. Switch traffic (update service selector)
kubectl patch service argocd-repo-server \
  -p '{"spec":{"selector":{"app.kubernetes.io/name":"argocd-repo-server-ecr"}}}' \
  -n argocd

# 4. Remove old deployment
kubectl delete deployment argocd-repo-server -n argocd

# 5. Rename new deployment
kubectl patch deployment argocd-repo-server-ecr \
  --type='merge' \
  -p='{"metadata":{"name":"argocd-repo-server"}}' \
  -n argocd
```

## 🔒 Security Hardening

### Image Security

```bash
# Build with security scanning
make image IMAGE_TAG=secure-argocd:latest
docker scout cves secure-argocd:latest
trivy image secure-argocd:latest

# Use distroless base (create custom Dockerfile)
# FROM gcr.io/distroless/base-debian12
```

### Runtime Security

```yaml
# Pod Security Context
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 999
    runAsGroup: 999
    fsGroup: 999
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: argocd-repo-server
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      readOnlyRootFilesystem: true
```

### Network Security

```yaml
# Network Policies
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: argocd-repo-server-netpol
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: argocd-repo-server
  policyTypes:
  - Ingress
  - Egress
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS
    - protocol: TCP  
      port: 53   # DNS
  - to: []
    ports:
    - protocol: UDP
      port: 53   # DNS
```

## 📊 Monitoring and Observability

### Health Checks

```yaml
# Enhanced health checks for ECR
livenessProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 30
  periodSeconds: 30
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /healthz  
    port: 8081
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3

# Startup probe for slower ECR authentication
startupProbe:
  httpGet:
    path: /healthz
    port: 8081
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 6
```

### Logging Configuration

```yaml
env:
- name: ARGOCD_LOG_LEVEL
  value: info  # debug for development
- name: ARGOCD_LOG_FORMAT  
  value: json  # for structured logging
- name: ARGOCD_LOG_ECR_AUTH
  value: "true"  # Enable ECR-specific logging
```

### Metrics and Monitoring

```bash
# Check ECR authentication metrics
kubectl exec -n argocd deployment/argocd-repo-server -- \
  curl -s localhost:8081/metrics | grep -i ecr

# Monitor ECR token refresh
kubectl logs -n argocd deployment/argocd-repo-server -f | grep "ECR token"

# Check cache performance
kubectl exec -n argocd deployment/argocd-repo-server -- \
  curl -s localhost:8081/metrics | grep -i cache
```

## 🚀 Performance Optimization

### Build Performance

```bash
# Use Go build cache
export GOCACHE=$HOME/.cache/go-build

# Parallel builds
make -j$(nproc) argocd-all

# Docker build optimization
export DOCKER_BUILDKIT=1
export BUILDKIT_PROGRESS=plain
```

### Runtime Performance

```yaml
# Resource optimization
resources:
  limits:
    cpu: 2000m
    memory: 2Gi
  requests:
    cpu: 500m
    memory: 512Mi

# JVM optimization for large clusters
env:
- name: GOGC  
  value: "80"  # More frequent GC for lower memory usage
```

## 🧪 Testing and Validation

### Automated Testing Pipeline

```bash
# Create testing script
cat > test-pipeline.sh << 'EOF'
#!/bin/bash
set -e

echo "🧪 ECR Integration Testing Pipeline"

# Build
make argocd-all

# Unit tests
go test ./util/helm/ -run TestAWSECR -v

# Integration tests  
./test-ecr-complete.sh

# Docker build
make image IMAGE_TAG=test-argocd:latest

# Docker test
docker run --rm test-argocd:latest argocd repo add --help | grep -q ecr

echo "✅ All tests passed!"
EOF

chmod +x test-pipeline.sh
```

### Continuous Integration

```yaml
# .github/workflows/ecr-integration.yml
name: ECR Integration Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.25'
    
    - name: Install tools
      run: |
        go install github.com/vektra/mockery/v3@v3.5.0
        sudo apt-get install -y protobuf-compiler
    
    - name: Build and test
      run: |
        make argocd-all
        go test ./util/helm/ -run TestAWSECR -v
        ./test-ecr-complete.sh
    
    - name: Build Docker image
      run: |
        make image IMAGE_TAG=test:latest
        docker run --rm test:latest argocd version
```

This comprehensive guide provides everything needed for Ubuntu development and Docker deployment with ECR support.
