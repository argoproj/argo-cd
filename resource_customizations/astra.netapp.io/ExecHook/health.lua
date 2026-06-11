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

hs = { status = "Progressing", message = "No status available" }
if obj.spec ~= nil then
  if obj.spec.enabled ~= nil then
    if obj.spec.enabled == true then
      hs.status = "Healthy"
      hs.message = obj.kind .. " enabled"
    elseif obj.spec.enabled == false then
      hs.status = "Suspended"
      hs.message = obj.kind .. " disabled"
    end
  end
end
return hs
