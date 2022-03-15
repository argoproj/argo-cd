if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["cluster.x-k8s.io/paused"] = ""
return obj