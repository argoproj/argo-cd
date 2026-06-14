# v2.10 to 2.11

## initiatedBy added in Application CRD

In order to address [argoproj/argo-cd#16612](https://github.com/argoproj/argo-cd/issues/16612), initiatedBy has been added in the Application CRD.

## Egress NetworkPolicy for `argocd-redis` and `argocd-redis-ha-haproxy`

Starting with Argo CD 2.11.2, the NetworkPolicy for the `argocd-redis` and `argocd-redis-ha-haproxy` dropped Egress restrictions. This change was made
to allow access to the Kubernetes API to create a secret to secure Redis access.

To retain similar networking restrictions as before 2.11.2, you can add an Egress rule to allow access only to the
Kubernetes API and access needed by Redis itself. The Egress rule for Kubernetes access will depend entirely on your
Kubernetes setup. The access for Redis itself can be allowed by adding the following to the
`argocd-redis-network-policy` NetworkPolicy:

```diff
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: argocd-redis-network-policy
spec:
  policyTypes:
  - Ingress
+  - Egress
+  egress:
+  - ports:
+    - port: 53
+      protocol: UDP
+    - port: 53
+      protocol: TCP
```

```diff
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: argocd-redis-ha-haproxy
spec:
  policyTypes:
  - Ingress
+  - Egress
+  egress:
+  - ports:
+    - port: 6379
+      protocol: TCP
+    - port: 26379
+      protocol: TCP
+    to:
+    - podSelector:
+        matchLabels:
+          app.kubernetes.io/name: argocd-redis-ha
+  - ports:
+    - port: 53
+      protocol: UDP
+    - port: 53
+      protocol: TCP
```

## Added Healthchecks

 * [route53.aws.crossplane.io/ResourceRecordSet](https://github.com/argoproj/argo-cd/commit/666499f6108124ef7bfa0c6cc616770c6dc4f42c)
 * [cloudfront.aws.crossplane.io/Distribution](https://github.com/argoproj/argo-cd/commit/21c384f42354ada2b94c18773104527eb27f86bc)
 * [beat.k8s.elastic.co/Beat](https://github.com/argoproj/argo-cd/commit/5100726fd61617a0001a27233cfe8ac4354bdbed)
 * [apps.kruise.io/AdvancedCronjob](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)
 * [apps.kruise.io/BroadcastJob](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)
 * [apps.kruise.io/CloneSet](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)
 * [apps.kruise.io/DaemonSet](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)
 * [apps.kruise.io/StatefulSet](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)
 * [rollouts.kruise.io/Rollout](https://github.com/argoproj/argo-cd/commit/d6da9f2a15fba708d70531c5b3f2797663fb3c08)