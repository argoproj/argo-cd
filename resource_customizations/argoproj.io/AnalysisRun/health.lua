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

local hs = {}

function messageOrDefault(field, default)
    if field ~= nil then
      return field
    end
    return default
  end

if obj.status ~= nil then
    if obj.status.phase == "Pending" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.phase == "Running" then
        hs.status = "Progressing"
        hs.message = "Analysis run is running"
    end
    if obj.status.phase == "Successful" then
        hs.status = "Healthy"
        hs.message = messageOrDefault(obj.status.message, "Analysis run completed successfully")
    end
    if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = messageOrDefault(obj.status.message, "Analysis run failed")
    end
    if obj.status.phase == "Error" then
        hs.status = "Degraded"
        hs.message = messageOrDefault(obj.status.message, "Analysis run had an error")
    end
    if obj.status.phase == "Inconclusive" then
        hs.status = "Unknown"
        hs.message = messageOrDefault(obj.status.message, "Analysis run was inconclusive")
    end
    return hs
end

hs.status = "Progressing"
hs.message = "Waiting for analysis run to finish: status has not been reconciled."
return hs