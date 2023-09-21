# Dynamic Cluster Distribution

Before Argo CD v2.9, sharding uses StatefulSet for the application controller. Although the application controller does not have any state to preserve, StatefulSets were used to get predictable hostnames and the serial number in the hostname was used to get the shard id of a particular instance.

Using StatefulSet has the following limitations:

* Any change done to the StatefulSet would cause all the child pods to restart in a serial fashion. This makes scaling up/down of the application controller slow as even existing healthy instances need to be restarted as well. 

* Each shard replica knows about the total number of available shards by evaluating the environment variable ARGOCD_CONTROLLER_REPLICAS, which needs to be kept up-to-date with the actual number of available replicas. If the number of replicas in the StatefulSet does not equal the number set in environment variable ARGOCD_CONTROLLER_REPLICAS, sharding will not work as intended, leading to both, unused and overused replicas. As this environment variable is set on the StatefulSet and propagated to the pods, all the pods in the StatefulSet need to be restarted in order to pick up the new number of total shards.


Starting v2.9, ArgoCD supports a dynamic cluster distribution feature. In this mechanism, Argo CD sharding uses Deployments for the application controller. 


## Enabling Dynamic Distribution of Clusters

Inorder to utilize the feature, the StatefulSet of Application Controller needs to be replaced with the Deployment Configuration of Application Controller. To do so, set the number of replicas of StatefulSet and the environment variable `ARGOCD_CONTROLLER_REPLICAS` to 0 disabling all current application controllers.

Once all the current controllers have stopped working, apply the below deployment configuration of application controller with the desired number of replicas.

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: argocd-application-controller
    app.kubernetes.io/name: argocd-application-controller
    app.kubernetes.io/part-of: argocd
  name: argocd-application-controller
spec:
  ports:
  - name: application-controller
    protocol: TCP
    port: 8082
    targetPort: 8082
  - name: metrics
    protocol: TCP
    port: 8084
    targetPort: 8084
  selector:
    app.kubernetes.io/name: argocd-application-controller
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/name: argocd-application-controller
    app.kubernetes.io/part-of: argocd
    app.kubernetes.io/component: application-controller
  name: argocd-application-controller
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: argocd-application-controller
  replicas: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: argocd-application-controller
    spec:
      containers:
      - args:
        - /usr/local/bin/argocd-application-controller
        env:
        - name: ARGOCD_CONTROLLER_REPLICAS
          value: "1"
        - name: ARGOCD_RECONCILIATION_TIMEOUT
          valueFrom:
            configMapKeyRef:
              name: argocd-cm
              key: timeout.reconciliation
              optional: true
        - name: ARGOCD_HARD_RECONCILIATION_TIMEOUT
          valueFrom:
            configMapKeyRef:
              name: argocd-cm
              key: timeout.hard.reconciliation
              optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: repo.server
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_TIMEOUT_SECONDS
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.repo.server.timeout.seconds
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_STATUS_PROCESSORS
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.status.processors
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_OPERATION_PROCESSORS
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.operation.processors
              optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_LOGFORMAT
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.log.format
              optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_LOGLEVEL
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.log.level
              optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_METRICS_CACHE_EXPIRATION
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.metrics.cache.expiration
              optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.self.heal.timeout.seconds
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_PLAINTEXT
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.repo.server.plaintext
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_STRICT_TLS
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.repo.server.strict.tls
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_PERSIST_RESOURCE_HEALTH
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.resource.health.persist
              optional: true
        - name: ARGOCD_APP_STATE_CACHE_EXPIRATION
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.app.state.cache.expiration
                optional: true
        - name: REDIS_SERVER
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: redis.server
                optional: true
        - name: REDIS_COMPRESSION
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: redis.compression
              optional: true
        - name: REDISDB
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: redis.db
                optional: true
        - name: ARGOCD_DEFAULT_CACHE_EXPIRATION
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.default.cache.expiration
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_OTLP_ADDRESS
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: otlp.address
                optional: true
        - name: ARGOCD_APPLICATION_NAMESPACES
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: application.namespaces
                optional: true
        - name: ARGOCD_CONTROLLER_SHARDING_ALGORITHM
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.sharding.algorithm
                optional: true
        - name: ARGOCD_APPLICATION_CONTROLLER_KUBECTL_PARALLELISM_LIMIT
          valueFrom:
              configMapKeyRef:
                name: argocd-cmd-params-cm
                key: controller.kubectl.parallelism.limit
                optional: true
        - name: ARGOCD_CONTROLLER_HEARTBEAT_TIME
          valueFrom:
            configMapKeyRef:
              name: argocd-cmd-params-cm
              key: controller.heatbeat.time
              optional: true
        image: quay.io/argoproj/argocd:latest
        imagePullPolicy: Always
        name: argocd-application-controller
        ports:
        - containerPort: 8082
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8082
          initialDelaySeconds: 5
          periodSeconds: 10
        securityContext:
          runAsNonRoot: true
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          seccompProfile:
            type: RuntimeDefault
        workingDir: /home/argocd
        volumeMounts:
        - name: argocd-repo-server-tls
          mountPath: /app/config/controller/tls
        - name: argocd-home
          mountPath: /home/argocd
      serviceAccountName: argocd-application-controller
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app.kubernetes.io/name: argocd-application-controller
              topologyKey: kubernetes.io/hostname
          - weight: 5
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app.kubernetes.io/part-of: argocd
              topologyKey: kubernetes.io/hostname
      volumes:
      - emptyDir: {}
        name: argocd-home
      - name: argocd-repo-server-tls
        secret:
          secretName: argocd-repo-server-tls
          optional: true
          items:
          - key: tls.crt
            path: tls.crt
          - key: tls.key
            path: tls.key
          - key: ca.crt
            path: ca.crt
```

Note the introduction of new environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME`. The environment variable is explained in [working of Dynamic Distribution Heartbeat Process](#working-of-dynamic-distribution)


## Working of Dynamic Distribution

Along with the new mechanism of sharding using Deployments for Application Controller, Application Controller also came up with new format for managing Controller <-> Shard mappings. 

Application Controller will create a new ConfigMap named `argocd-app-controller-shard-cm` to store the Controller <-> Shard mapping. The mapping would look like below for each shard:

```yaml
ControllerName: "argocd-application-controller-hydrxyt"
ShardNumber: 0
HeartbeatTime: "2009-11-17 20:34:58.651387237 +0000 UTC"
```

* `ControllerName`: Stores the hostname of the Application Controller pod
* `ShardNumber` : Stores the shard number managed by the controller pod
* `HeartbeatTime`: Stores the last time this heartbeat was updated.


Controller Shard Mapping is updated in the configMap during each readiness probe check of the pod, that is every 10 seconds (otherwise as configured). The controller will acquire the pod during every iteration of readiness probe check and try to update the ConfigMap with the `HeartbeatTime`. The default `HeartbeatDuration` after which the heartbeat should be updated is `10` seconds. If the ConfigMap was not updated for any controller pod for more than `3 * HeartbeatDuration`, then the readiness probe for the application pod is marked as `Unhealthy`. To increase the default `HeartbeatDuration`, you can set the environment variable `ARGOCD_CONTROLLER_HEARTBEAT_TIME` with the desired value.
