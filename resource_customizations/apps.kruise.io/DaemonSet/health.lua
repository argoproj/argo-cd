hs={ status = "Progressing", message = "Waiting for initialization" }

local function getTargetUpdated(desired, partition)
    if desired == nil then
        return nil
    end

    if partition == nil then
        return desired
    end

    if type(partition) == "string" then
        local percentage = string.match(partition, "^(%d+)%%$")
        if percentage ~= nil then
            return desired * (100 - tonumber(percentage)) / 100
        end

        partition = tonumber(partition)
    end

    if partition == nil then
        return desired
    end

    return desired - partition
end

if obj.status ~= nil then

    if obj.metadata.generation == obj.status.observedGeneration then

        local targetUpdated = getTargetUpdated(obj.status.desiredNumberScheduled, obj.spec.updateStrategy.rollingUpdate.partition)

        if obj.spec.updateStrategy.rollingUpdate.paused == true or not obj.status.updatedNumberScheduled then
            hs.status = "Suspended"
            hs.message = "Daemonset is paused"
            return hs
        elseif targetUpdated ~= nil and targetUpdated ~= obj.status.desiredNumberScheduled and obj.metadata.generation > 1 then
            if obj.status.updatedNumberScheduled > targetUpdated then
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
