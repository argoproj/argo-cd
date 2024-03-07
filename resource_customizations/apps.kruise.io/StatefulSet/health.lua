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
