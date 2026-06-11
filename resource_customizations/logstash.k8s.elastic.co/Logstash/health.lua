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
if obj.status ~= nil and obj.status.health ~= nil then
  if obj.status.health == "green" then
    hs.status = "Healthy"
    hs.message = "Logstash status is Green"
    return hs
  elseif obj.status.health == "yellow" then
    hs.status = "Progressing"
    hs.message = "Logstash status is Yellow"
    return hs
  elseif obj.status.health == "red" then
    hs.status = "Degraded"
    hs.message = "Logstash status is Red"
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Logstash"
return hs
