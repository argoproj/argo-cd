if obj.metadata.labels == nil then
    obj.metadata.labels = {}
end
obj.metadata.labels["numaplane.numaproj.io/promote"] = "false"
return obj