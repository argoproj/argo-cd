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


-- if UID not yet created, we are progressing
if obj.status == nil or obj.status.uid == "" then
  return {
    status = "Progressing",
    message = "",
  }
end

-- NoMatchingInstances distinguishes if we are healthy or degraded
if obj.status.NoMatchingInstances then
  return {
    status = "Degraded",
    message = "can't find matching grafana instance",
  }
end
return {
  status = "Healthy",
  message = "",
}