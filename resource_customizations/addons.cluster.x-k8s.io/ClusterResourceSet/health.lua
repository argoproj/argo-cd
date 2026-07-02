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

function getStatus(obj) 
  local hs = {}
  hs.status = "Progressing"
  hs.message = "Initializing cluster resource set"

  if obj.status ~= nil then
    if obj.status.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditions) do

        -- Ready
        if condition.type == "ResourcesApplied" and condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "cluster resource set is applied"
          return hs
        end

        -- Resources Applied
        if condition.type == "ResourcesApplied" and condition.status == "False" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end

      end
    end
  end
  return hs
end

local hs = getStatus(obj)
return hs