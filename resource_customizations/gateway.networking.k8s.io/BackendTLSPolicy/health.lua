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

hs.status = "Progressing"
hs.message = "Waiting for BackendTLSPolicy status"

if obj.status ~= nil and obj.status.ancestors ~= nil then
  if obj.metadata.generation ~= nil then
    for i, ancestor in ipairs(obj.status.ancestors) do
      for _, condition in ipairs(ancestor.conditions) do
        if condition.observedGeneration ~= nil then
          if condition.observedGeneration ~= obj.metadata.generation then
              hs.message = "Waiting for Ancestor " .. (ancestor.ancestorRef.name or "") .. " to update BackendTLSPolicy status"
             return hs
          end
        end
      end
    end
  end

  for i, ancestor in ipairs(obj.status.ancestors) do
    for j, condition in ipairs(ancestor.conditions) do
      if condition.type == "Accepted" then
        if condition.status ~= "True" then
          hs.status = "Degraded"
          hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
          return hs
        else
          hs.status = "Healthy"
          hs.message = "BackendTLSPolicy is healthy"
        end
      end

      if condition.type == "ResolvedRefs" then
        if condition.status ~= "True" then
          hs.status = "Degraded"
          hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
          return hs
        end
      end
    end
  end
end

return hs
