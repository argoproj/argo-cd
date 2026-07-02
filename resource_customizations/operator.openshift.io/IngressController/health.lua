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

-- healthcheck for IngressController resources
local hs = {}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
      -- if the status conditions are present, iterate over them and check their status
    for _, condition in pairs(obj.status.conditions) do
      if condition.type == "Degraded" and condition.status == "True" then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      elseif condition.type == "DeploymentReplicasAllAvailable" and condition.status == "False" then
        hs.status = "Progressing"
        hs.message =  condition.message
        return hs
      elseif condition.type == "Progressing" and condition.status == "True" then
        hs.status = "Progressing"
        hs.message =  condition.reason
        return hs
      elseif condition.type == "Available" and condition.status == "True" then
        hs.status = "Healthy"
        hs.message = "IngressController is available"
        return hs
      end
    end
  end
end

-- default status when none of the previous condition matches
hs.status = "Progressing"
hs.message = "Status of IngressController is not known yet"
return hs
