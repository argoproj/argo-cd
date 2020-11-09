#!/bin/bash

# Flags check
if [ -z $ARGOCD_SMOKE_URL ]; then
    # Check if nessasary flags exist
    echo "Please pass argocd server URL"
    exit 1
fi

if [ -z $ARGOCD_SMOKE_USERNAME ]; then
    # Check if nessasary flags exist
    echo "Please pass argocd server username"
    exit 1
fi

if [ -z $ARGOCD_SMOKE_PASSWORD ]; then
    # Check if nessasary flags exist
    echo "Please pass argocd server password"
    exit 1
fi
