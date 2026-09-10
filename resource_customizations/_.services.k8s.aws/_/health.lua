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

-- AWS ACK: https://aws-controllers-k8s.github.io/docs/
local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for i, condition in ipairs(obj.status.conditions) do

    if condition.type == "ACK.Terminal" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
    
    if condition.type == "Ready" then
      if condition.status == "False" then
        hs.status = "Progressing"
        hs.message = condition.message
        return hs
      elseif condition.status == "True" then
        hs.status = "Healthy"
        hs.message = ""
        return hs
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for resource to be ready"
return hs