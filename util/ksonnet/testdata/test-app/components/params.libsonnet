{
  global: {
    // User-defined global parameters; accessible to all component and environments, Ex:
    // replicas: 4,
  },
  components: {
    // Component-level parameters, defined initially from 'ks prototype use ...'
    // Each object below should correspond to a component in the components/ directory
    demo: {
      containerPort: 80,
      image: "gcr.io/kuar-demo/kuard-amd64:1",
      name: "demo",
      replicas: 2,
      servicePort: 80,
      type: "ClusterIP",
    },
  },
}
