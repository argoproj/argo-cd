local os = require("os")
if obj.metadata == nil then
    obj.metadata = {}
end
if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end

obj.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
return obj
