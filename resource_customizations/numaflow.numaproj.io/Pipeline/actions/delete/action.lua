if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["numaplane.numaproj.io/marked-for-deletion"] = "true"
return obj