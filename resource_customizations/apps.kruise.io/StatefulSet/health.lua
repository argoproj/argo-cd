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

hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then

    if obj.metadata.generation == obj.status.observedGeneration then

        if obj.spec.updateStrategy.rollingUpdate.paused == true or not obj.status.updatedAvailableReplicas then
            hs.status = "Suspended"
            hs.message = "Statefulset is paused"
            return hs
        elseif obj.spec.updateStrategy.rollingUpdate.partition ~= 0 and obj.metadata.generation > 1 then
            if obj.status.updatedReplicas > (obj.status.replicas - obj.spec.updateStrategy.rollingUpdate.partition) then
                hs.status = "Suspended"
                hs.message = "Statefulset needs manual intervention"
                return hs
            end

        elseif obj.status.updatedAvailableReplicas == obj.status.replicas then
            hs.status = "Healthy"
            hs.message = "All Statefulset workloads are ready and updated"    
            return hs
        
        else
            if obj.status.updatedAvailableReplicas ~= obj.status.replicas then
                hs.status = "Degraded"
                hs.message = "Some replicas are not ready or available"
                return hs
            end
        end

    end

end

return hs
