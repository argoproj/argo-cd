#!/bin/bash

# AWS ECR Workload Identity CLI Examples
# These examples show how to use the new ECR authentication features

echo "🚀 ArgoCD ECR Workload Identity CLI Examples"
echo "=============================================="

# Assuming ArgoCD server is running and accessible
# export ARGOCD_SERVER=argocd.example.com
# argocd login

echo ""
echo "📝 Example 1: Basic ECR Repository"
echo "argocd repo add oci://123456789.dkr.ecr.us-west-2.amazonaws.com \\"
echo "  --type helm \\"
echo "  --enable-oci \\"
echo "  --use-aws-ecr-workload-identity \\"
echo "  --name my-ecr-charts"

echo ""
echo "📝 Example 2: ECR with Explicit Region"
echo "argocd repo add oci://123456789.dkr.ecr.eu-central-1.amazonaws.com \\"
echo "  --type helm \\"
echo "  --enable-oci \\"
echo "  --use-aws-ecr-workload-identity \\"
echo "  --aws-ecr-region eu-central-1 \\"
echo "  --name eu-charts"

echo ""
echo "📝 Example 3: Cross-Account ECR Repository"
echo "argocd repo add oci://987654321.dkr.ecr.us-east-1.amazonaws.com \\"
echo "  --type helm \\"
echo "  --enable-oci \\"
echo "  --use-aws-ecr-workload-identity \\"
echo "  --aws-ecr-region us-east-1 \\"
echo "  --aws-ecr-registry-id 987654321 \\"
echo "  --name cross-account-charts"

echo ""
echo "📝 Example 4: ECR Repository Credentials Template"
echo "argocd repocreds add '*.dkr.ecr.*.amazonaws.com' \\"
echo "  --type helm \\"
echo "  --enable-oci \\"
echo "  --use-aws-ecr-workload-identity"

echo ""
echo "📝 Example 5: Admin Command for Repository Generation"
echo "argocd admin repo generate-spec oci://123456789.dkr.ecr.us-west-2.amazonaws.com \\"
echo "  --type helm \\"
echo "  --enable-oci \\"
echo "  --use-aws-ecr-workload-identity \\"
echo "  --name generated-ecr-repo"

echo ""
echo "🔍 Example 6: List Repositories (verify ECR repos)"
echo "argocd repo list"

echo ""
echo "🔍 Example 7: Get Repository Details" 
echo "argocd repo get oci://123456789.dkr.ecr.us-west-2.amazonaws.com"

echo ""
echo "✅ Prerequisites for these commands to work:"
echo "1. ArgoCD server running with ECR-enabled image"
echo "2. EKS cluster with IRSA configured"
echo "3. ServiceAccount with eks.amazonaws.com/role-arn annotation"
echo "4. IAM role with ECR permissions"
echo "5. ECR repository exists and is accessible"

echo ""
echo "🏗️ To build your custom ArgoCD image:"
echo "make image IMAGE_TAG=myregistry/argocd:v3.2.0-ecr.1"

echo ""
echo "🚀 To deploy with Helm:"
echo "helm upgrade argocd argo/argo-cd \\"
echo "  --set global.image.tag=v3.2.0-ecr.1 \\"
echo "  --set global.image.repository=myregistry/argocd \\"
echo "  --set repoServer.serviceAccount.annotations.'eks\\.amazonaws\\.com/role-arn'='arn:aws:iam::123456789:role/argocd-ecr-role'"
