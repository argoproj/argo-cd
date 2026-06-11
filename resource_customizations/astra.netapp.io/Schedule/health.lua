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

hs = { status = "Healthy", message = "Protection policy not yet executed" }
if obj.status ~= nil then
  if obj.status.lastScheduleTime ~= nil then
    hs.message = "Protection policy lastScheduleTime: " .. obj.status.lastScheduleTime
  end
end
return hs
