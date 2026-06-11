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

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" and condition.status == "True" then
      hs.status = "Healthy"
      hs.message = "3scale CustomPolicyDefinition is ready"
      return hs
    elseif condition.type == "Invalid" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale CustomPolicyDefinition configuration is invalid"
      return hs
    elseif condition.type == "Failed" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or "3scale CustomPolicyDefinition synchronization failed"
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale CustomPolicyDefinition status..."
return hs
