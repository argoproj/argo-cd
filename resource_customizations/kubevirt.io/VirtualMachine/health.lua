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

local hs = { status="Progressing", message="No status available"}
if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    for i, condition in ipairs(obj.status.conditions) do
      if condition.type == "Paused" and condition.status == "True" then
        hs.status = "Suspended"
        hs.message = "Paused"
        return hs
      end
      if condition.type == "Ready" then
        if condition.status == "True" then
          hs.status="Healthy"
          hs.message="Running"
        else
          if obj.status.created then
            hs.message = "Starting"
          else
            hs.status = "Suspended"
            hs.message = "Stopped"
          end
        end
      end
    end
  end
  if obj.status.printableStatus ~= nil then
    hs.message = obj.status.printableStatus
  end
end
return hs
