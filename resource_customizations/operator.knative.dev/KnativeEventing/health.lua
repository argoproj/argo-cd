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

local health_status = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local numTrue = 0
    local numFalse = 0
    local msg = ""
    for i, condition in pairs(obj.status.conditions) do
      msg = msg .. i .. ": " .. condition.type .. " | " .. condition.status .. "\n"
      if condition.type == "Ready" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "InstallSucceeded" and condition.status == "True" then
        numTrue = numTrue + 1
      elseif condition.type == "Ready" and condition.status == "False" then
        numFalse = numFalse + 1
      elseif condition.status == "Unknown" then
        numFalse = numFalse + 1
      end
    end
    if(numFalse > 0) then
      health_status.message = msg
      health_status.status = "Progressing"
      return health_status
    elseif(numTrue == 2) then
      health_status.message = "KnativeEventing is healthy."
      health_status.status = "Healthy"
      return health_status
    else
      health_status.message = msg
      health_status.status = "Degraded"
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for KnativeEventing"
return health_status