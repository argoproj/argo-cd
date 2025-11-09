#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

CLUSTER_NAME="argocd-perf-test"

echo -e "${YELLOW}Deleting kind cluster: $CLUSTER_NAME${NC}"
kind delete cluster --name "$CLUSTER_NAME"

echo -e "${GREEN}✓ Cleanup complete${NC}"
