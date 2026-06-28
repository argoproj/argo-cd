local obj = obj or {}
if obj.metadata and obj.metadata.annotations then
    obj.metadata.annotations["autoscaling.keda.sh/paused"] = nil
end

return obj
