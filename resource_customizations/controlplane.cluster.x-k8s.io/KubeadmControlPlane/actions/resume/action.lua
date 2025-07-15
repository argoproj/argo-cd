if obj.metadata.annotations ~= nil then
    obj.metadata.annotations["cluster.x-k8s.io/paused"] = nil
end
return obj