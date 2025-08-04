obj.spec.lifecycle.desiredPhase = "Running"
if obj.metadata.labels == nil then
    obj.metadata.labels = {}
end
obj.metadata.labels["numaflow.numaproj.io/resume-strategy"] = "fast"
return obj