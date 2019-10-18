local os = require("os")
if obj.spec.template.metadata == nil then
    obj.spec.template.metadata = {}
end
if obj.spec.template.metadata.annotations == nil then
    obj.spec.template.metadata.annotations = {}
end
obj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
return obj
