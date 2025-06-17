local os = require("os")
if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
if obj.spec.rolloutAfter == nil then
    obj.spec.rolloutAfter = {}
end
obj.metadata.annotations["cluster.x-k8s.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
obj.spec.rolloutAfter = os.date("!%Y-%m-%dT%XZ")
return obj
