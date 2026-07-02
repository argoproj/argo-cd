-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

hs={ status = "Progressing", message = "Rollout is still progressing" }

if obj.metadata.generation == obj.status.observedGeneration then

    if obj.status.canaryStatus.currentStepState == "StepUpgrade" and obj.status.phase == "Progressing" then
        hs.status = "Progressing"
        hs.message = "Rollout is still progressing"
        return hs
    end

    if obj.status.canaryStatus.currentStepState == "StepPaused" and obj.status.phase == "Progressing" then
        hs.status = "Suspended"
        hs.message = "Rollout is Paused need manual intervention"
        return hs
    end

    if obj.status.canaryStatus.currentStepState == "Completed" and obj.status.phase == "Healthy" then
        hs.status = "Healthy"
        hs.message = "Rollout is Completed"
        return hs
    end

    if obj.status.canaryStatus.currentStepState == "StepPaused" and (obj.status.phase == "Terminating" or obj.status.phase == "Disabled") then
        hs.status = "Degraded"
        hs.message = "Rollout is Disabled or Terminating"
        return hs
    end

end

return hs
