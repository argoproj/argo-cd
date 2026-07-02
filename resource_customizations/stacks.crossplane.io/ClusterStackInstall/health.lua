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

local hs = {
  status = "Progressing",
  message = "Waiting for stack to be installed"
}
if obj.status ~= nil then
  if obj.status.conditionedStatus ~= nil then
    if obj.status.conditionedStatus.conditions ~= nil then
      for i, condition in ipairs(obj.status.conditionedStatus.conditions) do
        if condition.type == "Ready" then
          hs.message = condition.reason
          if condition.status == "True" then
            hs.status = "Healthy"
            return hs
          end
        end
      end
    end
  end
end
return hs
