obj.spec.pipeline.spec.lifecycle.desiredPhase = "Running"
if obj.spec.pipeline.metadata == nil then
    obj.spec.pipeline.metadata = {}
end
if obj.spec.pipeline.metadata.annotations == nil then
    obj.spec.pipeline.metadata.annotations = {}
end
obj.spec.pipeline.metadata.annotations["numaflow.numaproj.io/resume-strategy"] = "slow"
return obj