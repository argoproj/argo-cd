local os = require("os")

obj.metadata.annotations["cluster.x-k8s.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
obj.spec.rolloutAfter = os.date("!%Y-%m-%dT%XZ")
return obj
