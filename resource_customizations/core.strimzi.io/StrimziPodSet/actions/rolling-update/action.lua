if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["strimzi.io/manual-rolling-update"] = "true"
return obj
