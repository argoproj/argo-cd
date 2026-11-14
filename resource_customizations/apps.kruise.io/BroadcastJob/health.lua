hs={ status= "Progressing", message= "BroadcastJob is still running" }

if obj.status ~= nil then 

-- BroadcastJob are healthy if desired number and succeeded number is equal
    if obj.status.desired == obj.status.succeeded and obj.status.phase == "completed" then 
        hs.status = "Healthy"
        hs.message = "BroadcastJob is completed successfully"
        return hs
    end
-- BroadcastJob are progressing if active is not equal to 0
    if obj.status.active ~= 0 and obj.status.phase == "running" then
        hs.status = "Progressing"
        hs.message = "BroadcastJob is still running"
        return hs
    end
-- BroadcastJob are progressing if failed is not equal to 0
    if obj.status.failed ~= 0  and obj.status.phase == "failed" then
        hs.status = "Degraded"
        hs.message = "BroadcastJob failed"
        return hs
    end

    if obj.status.phase == "paused" and obj.spec.paused == true then 
        hs.status = "Suspended"
        hs.message = "BroadcastJob is Paused"
        return hs
    end

end

return hs
