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
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local installed = false
    local healthy = false
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Installed" then
        installed = condition.status == "True"
        installed_message = condition.reason
      elseif condition.type == "Healthy" then
        healthy = condition.status == "True"
        healthy_message = condition.reason
      end
    end
    if installed and healthy then
      hs.status = "Healthy"
    else
      hs.status = "Degraded"
    end
    hs.message = installed_message .. " " .. healthy_message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for provider to be installed"
return hs
