{
  global: {
    // User-defined global parameters; accessible to all component and environments, Ex:
    // replicas: 4,
  },
  components: {
    // Component-level parameters, defined initially from 'ks prototype use ...'
    // Each object below should correspond to a component in the components/ directory
    "guestbook-ui": {
      containerPort: 80,
      image: "gcr.io/heptio-images/ks-guestbook-demo:0.2",
      name: "ks-guestbook-ui",
      replicas: 0,
      servicePort: 80,
      type: "ClusterIP",
      command: null,
    },
  },
}
