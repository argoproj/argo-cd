#!/bin/bash

# Default values for environment variables
REDIS_PORT="${ARGOCD_E2E_REDIS_PORT:-6379}"
REDIS_IMAGE_TAG=$(grep 'image: redis' manifests/base/redis/argocd-redis-deployment.yaml | cut -d':' -f3)

# Check if ARGOCD_REDIS_LOCAL is 'true'
if [ "$ARGOCD_REDIS_LOCAL" = 'true' ]; then
    # Start local Redis server with password if defined
    if [ -z "$REDIS_PASSWORD" ]; then
        echo "Starting local Redis server without password."
        redis-server --save '' --appendonly no --port "$REDIS_PORT"
    else
        echo "Starting local Redis server with password."
        redis-server --save '' --appendonly no --port "$REDIS_PORT" --requirepass "$REDIS_PASSWORD"
    fi
else
    # Run Redis in a Docker container with password if defined
    if [ -z "$REDIS_PASSWORD" ]; then
        echo "Starting Docker container without password."
        docker run --rm --name argocd-redis -i -p "$REDIS_PORT:$REDIS_PORT" docker.io/library/redis:"$REDIS_IMAGE_TAG" --save '' --appendonly no --port "$REDIS_PORT"
    else
        echo "Starting Docker container with password."
        docker run --rm --name argocd-redis -i -p "$REDIS_PORT:$REDIS_PORT" -e REDIS_PASSWORD="$REDIS_PASSWORD" docker.io/library/redis:"$REDIS_IMAGE_TAG" redis-server --save '' --requirepass "$REDIS_PASSWORD" --appendonly no --port "$REDIS_PORT"
    fi
fi
