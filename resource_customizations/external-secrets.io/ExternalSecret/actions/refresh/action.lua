local os = require("os")
if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["force-sync"] = os.date("!%Y-%m-%dT%XZ")
return obj
