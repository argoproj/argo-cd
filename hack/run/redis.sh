#!/bin/sh
if [ "$ARGOCD_REDIS_LOCAL" = 'true' ]; then 
	redis-server --save '' --appendonly no --port ${ARGOCD_E2E_REDIS_PORT:-6379}
else
	docker run --rm --name argocd-redis -i -p ${ARGOCD_E2E_REDIS_PORT:-6379}:${ARGOCD_E2E_REDIS_PORT:-6379} docker.io/library/redis:$(grep "image: redis" manifests/base/redis/argocd-redis-deployment.yaml | cut -d':' -f3) --save '' --appendonly no --port ${ARGOCD_E2E_REDIS_PORT:-6379}
fi
