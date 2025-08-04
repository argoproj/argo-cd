obj.spec.lifecycle.desiredPhase = "Running"
if obj.metadata.annotations == nil then
    obj.metadata.annotations = {}
end
obj.metadata.annotations["numaflow.numaproj.io/resume-strategy"] = "fast"
return obj