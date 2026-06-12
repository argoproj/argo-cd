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

hs = {}
if obj.status ~= nil then
  if obj.status.ansibleJobResult ~= nil then
    jobstatus = obj.status.ansibleJobResult.status
    if jobstatus == "successful" then
      hs.status = "Healthy"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
    if jobstatus == "failed" or jobstatus == "error" or jobstatus == "canceled" then
      hs.status = "Degraded"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
    if jobstatus == "new" or jobstatus == "pending" or jobstatus == "waiting" or jobstatus == "running" then
      hs.status = "Progressing"
      hs.message = jobstatus .. " job - " .. obj.status.ansibleJobResult.url
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for AnsibleJob"
return hs
