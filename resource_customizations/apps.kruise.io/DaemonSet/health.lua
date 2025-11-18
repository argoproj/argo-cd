hs={ status = "Progressing", message = "Waiting for initialization" }

if obj.status ~= nil then

    if obj.metadata.generation == obj.status.observedGeneration then

        if obj.spec.updateStrategy.rollingUpdate.paused == true or not obj.status.updatedNumberScheduled then
            hs.status = "Suspended"
            hs.message = "Daemonset is paused"
            return hs
        elseif (obj.spec.updateStrategy.rollingUpdate.partition ~= nil) and (obj.spec.updateStrategy.rollingUpdate.partition ~= 0 and obj.metadata.generation > 1) then
            if obj.status.updatedNumberScheduled > (obj.status.desiredNumberScheduled - obj.spec.updateStrategy.rollingUpdate.partition) then
                hs.status = "Suspended"
                hs.message = "Daemonset needs manual intervention"
                return hs
            end

        elseif (obj.status.updatedNumberScheduled == obj.status.desiredNumberScheduled) and (obj.status.numberAvailable == obj.status.desiredNumberScheduled) then
            hs.status = "Healthy"
            hs.message = "All Daemonset workloads are ready and updated"    
            return hs
        
        else
            if (obj.status.updatedNumberScheduled == obj.status.desiredNumberScheduled) and (obj.status.numberUnavailable == obj.status.desiredNumberScheduled) then
                hs.status = "Degraded"
                hs.message = "Some pods are not ready or available"
                return hs
            end
        end

    end

end

return hs
