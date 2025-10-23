if obj.metadata.labels == nil then
    obj.metadata.labels = {}
end
obj.metadata.labels["numaplane.numaproj.io/force-promote"] = "true"
return obj