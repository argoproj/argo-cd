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


-- if no status info available yet, assume progressing
if obj.status == nil or obj.status.stageStatus == nil then
  return {
    status = "Progressing",
    message = "Waiting for Grafana status info",
  }
end

-- if last stage failed, we are stuck here
if obj.status.stageStatus == "failed" then
  return {
    status = "Degraded",
    message = "Failed at stage " .. obj.status.stage,
  }
end

-- only if "complete" stage was successful, Grafana can be considered healthy
if obj.status.stage == "complete" and obj.status.stageStatus == "success" then
  return {
    status = "Healthy",
    message = "",
  }
end

-- no final status yet, assume progressing
return {
  status = "Progressing",
  message = obj.status.stage,
}