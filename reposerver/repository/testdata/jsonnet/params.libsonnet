{
  containerPort: 80,
  image: "gcr.io/heptio-images/ks-guestbook-demo:0.2",
  name: "guestbook-ui",
  replicas: 1,
  servicePort: 80,
  type: "ClusterIP",
}
