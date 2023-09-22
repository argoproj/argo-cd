local os = require("os")
if obj.spec.template.metadata == nil then
    obj.spec.template.metadata = {}
end
if obj.spec.template.metadata.annotations == nil then
    obj.spec.template.metadata.annotations = {}
end
obj.spec.replicas = tonumber(scale)
return obj
