# Redis Password Configuration Options

ArgoCD supports two methods for configuring Redis password authentication. Choose the option that best fits your deployment needs.

## Option 1: Environment Variable (Default)

**Location**: `manifests/base/`  
**Use case**: Standard deployments, existing installations  
**Setup effort**: None (default behavior)  

### Description
Uses the traditional `REDIS_PASSWORD` environment variable approach where the password is read from the `argocd-redis` Kubernetes secret.

### Usage
```bash
# Apply base manifests (default behavior)
kubectl apply -k manifests/base/

# Verify Redis secret exists
kubectl get secret argocd-redis -o jsonpath='{.data.auth}' | base64 -d
```

**✅ This is the recommended approach for most users**

## Option 2: File Mount (Advanced)

**Location**: `manifests/overlays/file-mount/`  
**Use case**: External secret management, advanced security requirements  
**Setup effort**: Moderate (requires credential files)  

### Description
Uses file-based credentials mounted into the Redis container. Supports external secret management systems like Vault, AWS Secrets Manager, etc.

### Usage

#### Step 1: Apply the file-mount overlay
```bash
# Apply the file-mount variant
kubectl apply -k manifests/overlays/file-mount/
```

#### Step 2: Create your credential secret
```bash
# Create secret with credential files
kubectl create secret generic my-redis-creds \
  --from-literal=auth=mypassword \
  --from-literal=auth_username=myuser
```

#### Step 3: Update Redis to use your secret
```bash
# Patch the Redis deployment to use your secret
kubectl patch deployment argocd-redis -p '{
  "spec": {
    "template": {
      "spec": {
        "volumes": [{
          "name": "redis-creds",
          "secret": {
            "secretName": "my-redis-creds",
            "optional": false
          }
        }]
      }
    }
  }
}'
```

## What's Included in File-Mount Overlay

### New Components
- **Redis ConfigMap**: Contains Redis configuration template and setup script
- **Init Container**: Processes credential files and generates Redis configuration

### Patches Applied
- **Redis Deployment**: Adds init container, volumes, and file-mount support
- **Server Deployment**: Adds `REDIS_CREDS_FILE_PATH` environment variable
- **Repo Server Deployment**: Adds `REDIS_CREDS_FILE_PATH` environment variable  
- **Application Controller**: Adds `REDIS_CREDS_FILE_PATH` environment variable
- **Application Controller StatefulSet**: Adds `REDIS_CREDS_FILE_PATH` environment variable

## Credential File Structure

When using file-mount, the following files can be provided:

| File Name | Description | Required |
|-----------|-------------|----------|
| `auth` | Redis password | Yes |
| `auth_username` | Redis username | No |
| `sentinel_username` | Sentinel username | No |
| `sentinel_auth` | Sentinel password | No |

## Configuration Priority

The file-mount overlay follows this priority order:
1. **File-based credentials** (if files exist and are readable)
2. **Environment variables** (fallback if files not found)
3. **No authentication** (if neither source is available)

## Switching Between Options

### From Environment Variable to File Mount
```bash
# Apply the file-mount overlay
kubectl apply -k manifests/overlays/file-mount/

# Create your credential secret (see Step 2 above)
kubectl create secret generic my-redis-creds --from-literal=auth=mypassword

# Update Redis deployment (see Step 3 above)
```

### From File Mount to Environment Variable  
```bash
# Apply base manifests
kubectl apply -k manifests/base/

# Ensure the argocd-redis secret exists
kubectl get secret argocd-redis
```

## Verification

### Check which method is active
```bash
# Check init container logs
kubectl logs deployment/argocd-redis -c redis-config-init

# Expected output for file-mount:
# "Using password from file: /redis-creds/auth"

# Expected output for environment variable:
# "Using password from environment variable"
```

### Verify Redis configuration
```bash
# Check generated Redis config
kubectl exec deployment/argocd-redis -- cat /data/redis.conf | grep requirepass

# Test Redis connection
kubectl exec deployment/argocd-redis -- redis-cli ping
```

## Integration with External Secret Management

### Example: Using External Secrets Operator
```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "argocd"
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: redis-credentials
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: my-redis-creds
    creationPolicy: Owner
  data:
  - secretKey: auth
    remoteRef:
      key: redis
      property: password
```

## Troubleshooting

### File mount not working
```bash
# Check if files are mounted correctly
kubectl exec deployment/argocd-redis -- ls -la /redis-creds/

# Check init container logs
kubectl logs deployment/argocd-redis -c redis-config-init

# Verify secret contents
kubectl get secret my-redis-creds -o yaml
```

### Environment variable not working
```bash
# Check if argocd-redis secret exists
kubectl get secret argocd-redis

# Check environment variables in container
kubectl exec deployment/argocd-redis -- env | grep REDIS_PASSWORD
```

### Connection issues
```bash
# Test Redis connectivity
kubectl exec deployment/argocd-redis -- redis-cli ping

# Check Redis logs
kubectl logs deployment/argocd-redis -c redis

# Verify ArgoCD components can connect
kubectl logs deployment/argocd-server | grep -i redis
```

## Best Practices

1. **Start with Environment Variable**: Use the default approach unless you have specific requirements
2. **Secure File Permissions**: Ensure credential files have appropriate permissions (600/400)
3. **Regular Rotation**: Implement credential rotation for enhanced security
4. **Monitor Access**: Log and monitor access to credential files
5. **Backup Strategies**: Include credential management in your backup/recovery procedures 