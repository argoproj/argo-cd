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
  if obj.status.developerUserState ~= nil and obj.status.developerUserState == "suspended" then
    hs.status = "Suspended"
    hs.message = "3scale Developer user is waiting for admin approval"
    return hs
  end

  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.status ~= "True" then goto continue end

      if condition.type == "Ready" then
        hs.status = "Healthy"
        hs.message = "3scale DeveloperUser is ready"
        return hs
      elseif condition.type == "Invalid" then
        hs.status = "Degraded"
        hs.message = condition.message or "3scale DeveloperUser configuration is invalid"
        return hs
      elseif condition.type == "Orphan" then
        hs.status = "Degraded"
        hs.message = condition.message or "3scale DeveloperUser references non-existing resources"
        return hs
      elseif condition.type == "Failed" then
        hs.status = "Degraded"
        hs.message = condition.message or "3scale DeveloperUser synchronization failed"
        return hs
      end

      ::continue::
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale DeveloperUser status..."
return hs
