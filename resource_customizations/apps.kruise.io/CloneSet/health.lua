hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then
    -- these counters are omitempty, so they are missing when they are 0
    local updatedAvailableReplicas = obj.status.updatedAvailableReplicas or 0
    local replicas = obj.status.replicas or 0
        
    if obj.metadata.generation == obj.status.observedGeneration then

        if obj.spec.updateStrategy.paused == true then
            hs.status = "Suspended"
            hs.message = "Cloneset is paused"
            return hs
        elseif obj.spec.updateStrategy.partition ~= 0 and obj.metadata.generation > 1 then
            if obj.status.updatedReplicas >= obj.status.expectedUpdatedReplicas then
                hs.status = "Suspended"
                hs.message = "Cloneset needs manual intervention"
                return hs
            end

        elseif updatedAvailableReplicas == replicas then
            hs.status = "Healthy"
            hs.message = "All Cloneset workloads are ready and updated"    
            return hs
        
        else
            if updatedAvailableReplicas ~= replicas then
                hs.status = "Degraded"
                hs.message = "Some replicas are not ready or available"
                return hs
            end
        end
    end
end

return hs
