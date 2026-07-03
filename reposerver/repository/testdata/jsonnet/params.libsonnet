{
  containerPort: 80,
  image: "quay.io/argoprojlabs/argocd-e2e-container:0.2",
  name: "guestbook-ui",
  replicas: 1,
  servicePort: 80,
  type: "ClusterIP",
}
