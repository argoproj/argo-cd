if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["numaplane.numaproj.io/allow-data-loss"] = "true"
return obj