obj.spec.pipeline.spec.lifecycle.desiredPhase = "Running"
if obj.spec.pipeline.spec.metadata == nil then
    obj.spec.pipeline.spec.metadata = {}
end
if obj.spec.pipeline.spec.metadata.annotations == nil then
    obj.spec.pipeline.spec.metadata.annotations = {}
end
obj.spec.pipeline.spec.metadata.annotations["numaflow.numaproj.io/resume-strategy"] = "fast"
return obj