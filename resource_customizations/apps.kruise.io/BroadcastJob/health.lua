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
